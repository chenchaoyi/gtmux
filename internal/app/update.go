package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/i18n"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// updateCheckJSON is the `gtmux update --check --json` payload the menu-bar app
// reads to decide whether to surface a "new version available" prompt.
type updateCheckJSON struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Update    bool   `json:"update"`
	Error     string `json:"error,omitempty"`
	Source    string `json:"source,omitempty"`
	Warning   string `json:"warning,omitempty"`
	CLIUpdate bool   `json:"cli_update"`
	AppUpdate bool   `json:"app_update"`
}

// updateCheckPayload builds the --json result. `update` is true when `latest` is newer
// than EITHER the CLI (`cur`) OR the installed menu-bar app (`app`, "" when absent) — so a
// STALE app still surfaces the prompt even when the CLI is already current. Without this,
// a CLI-ahead-of-app machine reported "no update" and the app could never escape being
// behind (the reported version-inconsistency loop). An empty latest means the release API
// was unreachable (`error` set, `update` stays false so the app never prompts on failure).
func updateCheckPayload(cur, app, latest string) updateCheckJSON {
	cliBehind, appBehind := newerRelease(latest, cur), app != "" && newerRelease(latest, app)
	out := updateCheckJSON{Current: cur, Latest: latest, Update: cliBehind || appBehind, CLIUpdate: cliBehind, AppUpdate: appBehind}
	if latest == "" {
		out.Error = "couldn't reach the release API"
	} else if !out.Update && (!comparableRelease(cur) || (app != "" && !comparableRelease(app))) {
		out.Error = "installed component version is unknown; automatic replacement skipped"
	}
	return out
}

// installedAppVersion reads the on-disk menu-bar bundle's CFBundleShortVersionString
// (~/Applications preferred, /Applications fallback — where install.sh and the Homebrew
// cask put Gtmux.app). "" if no app / unreadable / not macOS. Read from the plist, not any
// in-memory value, so it reflects the ACTUAL installed app — the input that lets
// `gtmux update` refresh a stale app while the CLI is already current.
func installedAppVersion() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	for _, app := range installedAppCandidates(state.Home(), "/Applications/Gtmux.app") {
		plist := filepath.Join(app, "Contents", "Info.plist")
		out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleShortVersionString", plist).Output()
		if err != nil {
			continue
		}
		// Normalize a leading "v": the app bundle's CFBundleShortVersionString carries the
		// git tag verbatim ("v0.44.10") while the CLI's Version is v-stripped, so an
		// un-normalized compare would read a CURRENT app as "behind" (never matching) and
		// double-"v" it in doctor. Strip it so both sides speak the same form.
		if v := strings.TrimPrefix(strings.TrimSpace(string(out)), "v"); v != "" {
			return v
		}
	}
	return ""
}

// An existing bundle with unreadable metadata is unknown, not an absent app that
// the updater may install over. Keep doctor's historical version reader unchanged.
func installedAppUpdateVersion() string {
	v := installedAppVersion()
	if v == "" && runtime.GOOS == "darwin" && len(installedAppCandidates(state.Home(), "/Applications/Gtmux.app")) > 0 {
		return "unknown"
	}
	return v
}

// installedAppCandidates is the single install-location check shared by doctor,
// doctor --fix, and update. Homebrew uses /Applications; the standalone installer
// usually uses ~/Applications. A bundle without Info.plist is incomplete.
func installedAppCandidates(home, systemPath string) []string {
	paths := []string{filepath.Join(home, "Applications", "Gtmux.app"), systemPath}
	var found []string
	for _, path := range paths {
		if fi, err := os.Stat(filepath.Join(path, "Contents", "Info.plist")); err == nil && !fi.IsDir() {
			found = append(found, path)
		}
	}
	return found
}

// cmdUpdate implements `gtmux update` — self-update to the latest release by
// downloading release assets and verifying the CLI against GitHub checksums.
// Mirrors may supply CLI bytes, but never checksums, scripts, or the menu-bar app.
// `--check` only reports; `--cli-only` skips the app. Updating in place is safe:
// the updater atomic-swaps the binary, and a running executable keeps its inode.
func cmdUpdate(args []string) int {
	checkOnly, cliOnly, jsonOut := false, false, false
	for _, a := range args {
		switch a {
		case "-h", "--help":
			i18n.Say("usage: gtmux update [--check [--json]] [--cli-only]",
				"用法：gtmux update [--check [--json]] [--cli-only]")
			i18n.Say("  Update gtmux (CLI + menu-bar app) to the latest release.",
				"  把 gtmux（CLI + 菜单栏 app）更新到最新版。")
			i18n.Say("  --check: only report if a newer version exists. --cli-only: skip the app.",
				"  --check：只检查有无新版。--cli-only：只更新 CLI，不动 app。")
			i18n.Say("  --json: with --check, print {current,latest,update} as JSON (for the app).",
				"  --json：配合 --check，以 JSON 输出 {current,latest,update}（供 app 调用）。")
			return 0
		case "--check":
			checkOnly = true
		case "--json":
			jsonOut = true
			checkOnly = true // JSON is a machine-readable check; never installs
		case "--cli-only":
			cliOnly = true
		default:
			i18n.Sae("gtmux update: unknown option '"+a+"'", "gtmux update: 未知选项 '"+a+"'")
			return 2
		}
	}

	return performUpdate(checkOnly, cliOnly, jsonOut, updateOperations{
		current: Version, appVersion: installedAppUpdateVersion, latest: fetchLatestRelease,
		install: runReleaseInstaller, changes: printWhatChanged,
		doctor: func() { postUpdateDoctor(installedCLIPath(), 90*time.Second) },
	})
}

// Keep the command's decision/output path testable without network or installation.
type updateOperations struct {
	current    string
	appVersion func() string
	latest     func() latestRelease
	install    func(bool, string) (releaseInstallResult, int)
	changes    func(string, string)
	doctor     func()
}

func performUpdate(checkOnly, cliOnly, jsonOut bool, ops updateOperations) int {
	cur := strings.TrimPrefix(ops.current, "v")
	release := ops.latest()
	latest := strings.TrimPrefix(release.Tag, "v")
	appV := ""
	if !cliOnly && os.Getenv("GTMUX_NO_APP") == "" {
		appV = ops.appVersion()
	}
	payload := updateCheckPayload(cur, appV, latest)
	payload.Source = release.Source
	if release.Source == "jsdelivr" {
		payload.Warning = "cached release index may be behind GitHub"
		// Older consumers already treat an error without an update as an
		// inconclusive check, rather than claiming the installation is current.
		if !payload.Update {
			payload.Error = payload.Warning
		}
	}
	if jsonOut {
		b, _ := json.Marshal(payload)
		fmt.Println(string(b))
		return 0
	}
	pinned := !checkOnly && os.Getenv("GTMUX_VERSION") != ""
	if latest == "" && !pinned {
		i18n.Sae("gtmux update: couldn't resolve a stable release from the release API.",
			"gtmux update: 无法从发布接口确定稳定版本。")
		return 1
	}
	if payload.Warning != "" {
		i18n.Sae("GitHub latest is unavailable; the cached release index may be behind.",
			"无法确认 GitHub 最新版，缓存版本索引可能落后。")
	}
	// GTMUX_VERSION is the existing explicit install pin, including downgrades.
	// Checks always describe the discovered release, never the installation pin.
	if !payload.Update && !pinned {
		i18n.Say("No newer comparable release found (CLI "+cur+", release "+latest+").",
			"未发现可比较的更新版本（CLI "+cur+"，发布版本 "+latest+"）。")
		return 0
	}
	if checkOnly {
		i18n.Say("update available: "+latest+"  (run `gtmux update`)",
			"有可更新版本："+latest+"  （运行 `gtmux update`）")
		return 0
	}
	if pinned {
		latest = strings.TrimPrefix(os.Getenv("GTMUX_VERSION"), "v")
	}
	i18n.Say("Updating selected components to "+latest+" …", "正在将需要更新的组件更新到 "+latest+" …")
	result, rc := ops.install(cliOnly, release.Tag)
	diag.DidRC("act.update", "gtmux", rc, "update components",
		"from", cur, "to", latest, "cliOnly", cliOnly)
	if result.CLIInstalled || result.AppInstalled {
		from := cur
		if result.AppInstalled && (!result.CLIInstalled || newerRelease(cur, appV)) {
			from = appV
		}
		ops.changes(from, latest)
		if result.CLIInstalled && os.Getenv("GTMUX_ACTOR") != "menubar" {
			ops.doctor()
		}
	}
	return rc
}

// Automatic updates only select stable triples (optional build metadata). Unknown
// installed versions are left alone. For prerelease/dev builds, compare the numeric
// baseline conservatively: the same stable triple must not replace a newer local build.
func newerRelease(target, current string) bool {
	target = strings.TrimSpace(target)
	current = strings.TrimSpace(current)
	if !releaseTagPattern.MatchString(target) || !releaseTagPattern.MatchString(current) || strings.Contains(strings.SplitN(target, "+", 2)[0], "-") {
		return false
	}
	t, tok := parseSemver(target)
	c, cok := parseSemver(current)
	return tok && cok && compareSemver(t, c) > 0
}

func comparableRelease(version string) bool {
	_, ok := parseSemver(version)
	return ok && releaseTagPattern.MatchString(strings.TrimSpace(version))
}

// printWhatChanged summarises the versions just crossed. Best-effort and SILENT on
// failure: this runs after a SUCCESSFUL install, so an error here would be noise about a
// cosmetic step — and worse, would read as though the update itself had gone wrong.
func printWhatChanged(from, to string) {
	if to == "" {
		return
	}
	entries := changesBetween(fetchReleases(), from, to)
	lines, omitted := flatten(entries, changelogMax)
	if len(lines) == 0 {
		return
	}
	fmt.Println()
	i18n.Say(i18n.Bold+"  What changed"+i18n.Reset, i18n.Bold+"  本次更新"+i18n.Reset)
	for _, l := range lines {
		fmt.Println("    · " + l)
	}
	if omitted > 0 {
		i18n.Say(i18n.Dim+"    +"+strconv.Itoa(omitted)+" more (gtmux whatsnew)"+i18n.Reset,
			i18n.Dim+"    还有 "+strconv.Itoa(omitted)+" 条（gtmux whatsnew）"+i18n.Reset)
	}
}

// runInstaller installs release assets using local code. Remote scripts are never
// fetched or executed. Shared by update, install app, and doctor --fix.
const partialUpdateExit = 3

func runInstaller(cliOnly bool, version string) int {
	_, rc := runReleaseInstaller(cliOnly, version)
	return rc
}

func runReleaseInstaller(cliOnly bool, version string) (releaseInstallResult, int) {
	installer, err := newReleaseInstaller(cliOnly, version)
	var result releaseInstallResult
	if err == nil {
		result, err = installer.install()
	}
	if result.CLIInstalled {
		restartServeAgents()
	}
	return result, reportReleaseInstall(result, err)
}

func reportReleaseInstall(result releaseInstallResult, err error) int {
	if err != nil {
		i18n.Sae("gtmux: update failed: "+err.Error(), "gtmux: 更新失败："+err.Error())
		return 1
	}
	if result.AppError != nil {
		cli := "CLI unchanged"
		cliZH := "CLI 未变更"
		if result.CLIInstalled {
			cli = "CLI installed"
			cliZH = "CLI 已安装"
		}
		app := "menu-bar app not installed"
		appZH := "菜单栏 app 未安装"
		if result.AppInstalled {
			app = "menu-bar app installed, activation failed"
			appZH = "菜单栏 app 已安装，激活失败"
		}
		next := "retry `gtmux update` (or `gtmux install app` if absent)"
		nextZH := "重试 `gtmux update`（未装 App 时用 `gtmux install app`）"
		if result.AppInstalled {
			next = "run `gtmux app` from a desktop session; check the app's login setting"
			nextZH = "请在桌面会话运行 `gtmux app`，并检查 App 登录启动设置"
		}
		i18n.Sae("gtmux: "+cli+"; "+app+"; "+next+": "+result.AppError.Error(),
			"gtmux: "+cliZH+"；"+appZH+"；"+nextZH+"："+result.AppError.Error())
		return partialUpdateExit
	}
	return 0
}

// restartServeAgents re-execs the long-running LaunchAgents that run the gtmux binary, so
// a freshly installed one takes effect immediately: serve, and the Direct tunnel client.
// The tunnel client was left out while the only tunnel agent was cloudflared, which is not
// gtmux code; once Direct ran `gtmux tunnel-client`, every update left it on the old
// binary until the next login (v1.0.34 installed a client that reports its own state, and
// nothing reported it). The platform service adapter bounds every manager call; inactive services are not started.
func restartServeAgents() {
	for _, label := range []string{serveAgentLabel, selfTunnelAgentLabel} {
		if !serviceLoaded(label) {
			continue
		}
		if err := serviceManager().Restart(label); err != nil {
			i18n.Sae("Could not restart "+label+". Restart it manually to use the update.", "未能重启 "+label+"，请手动重启后使用新版。")
		} else {
			i18n.Say("Restarted "+label+" with the updated binary.", "已使用新版重启 "+label+"。")
		}
	}
}

// runBounded runs an external command with a hard timeout, returning a non-nil error
// if it fails OR exceeds the deadline (the process is killed). Used for best-effort
// maintenance calls that must never hang an interactive command.
func runBounded(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Run()
}

// Release lookup tries GitHub, then a potentially lagging jsdelivr index. The
// update command retains the source; legacy installer callers only need the tag.
type latestRelease struct{ Tag, Source string }

func fetchLatestTag() string { return fetchLatestRelease().Tag }

func fetchLatestRelease() latestRelease { return resolveLatestRelease(httpGetBytes) }

func resolveLatestRelease(get func(string, time.Duration) []byte) latestRelease {
	if b := get("https://api.github.com/repos/chenchaoyi/gtmux/releases/latest", 10*time.Second); b != nil {
		if tag := parseTagName(string(b)); newerRelease(tag, "0.0.0") {
			return latestRelease{tag, "github"}
		}
	}
	if b := get("https://data.jsdelivr.com/v1/packages/gh/chenchaoyi/gtmux", 10*time.Second); b != nil {
		if tag := parseJsdelivrLatest(string(b)); tag != "" {
			return latestRelease{tag, "jsdelivr"}
		}
	}
	return latestRelease{}
}

// parseTagName pulls "tag_name":"vX.Y.Z" out of the GitHub releases/latest JSON
// "" if absent, malformed, draft or prerelease.
func parseTagName(body string) string {
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if json.Unmarshal([]byte(body), &release) != nil || release.Draft || release.Prerelease {
		return ""
	}
	return release.Tag
}

// parseJsdelivrLatest selects the highest stable numeric version in the cached
// index. Returns it tag-shaped ("vX.Y.Z").
func parseJsdelivrLatest(body string) string {
	var d struct {
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	}
	if json.Unmarshal([]byte(body), &d) != nil || len(d.Versions) == 0 {
		return ""
	}
	// Do not trust list order or select prereleases as stable updates.
	v := ""
	for _, entry := range d.Versions {
		candidate := strings.TrimSpace(entry.Version)
		if newerRelease(candidate, "0.0.0") && (v == "" || newerRelease(candidate, v)) {
			v = candidate
		}
	}
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

func httpGetBytes(url string, timeout time.Duration) []byte {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	return b
}
