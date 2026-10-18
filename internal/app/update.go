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
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Update  bool   `json:"update"`
	Error   string `json:"error,omitempty"`
}

// updateCheckPayload builds the --json result. `update` is true when `latest` is newer
// than EITHER the CLI (`cur`) OR the installed menu-bar app (`app`, "" when absent) — so a
// STALE app still surfaces the prompt even when the CLI is already current. Without this,
// a CLI-ahead-of-app machine reported "no update" and the app could never escape being
// behind (the reported version-inconsistency loop). An empty latest means the release API
// was unreachable (`error` set, `update` stays false so the app never prompts on failure).
func updateCheckPayload(cur, app, latest string) updateCheckJSON {
	behind := latest != "" && (latest != cur || (app != "" && latest != app))
	out := updateCheckJSON{Current: cur, Latest: latest, Update: behind}
	if latest == "" {
		out.Error = "couldn't reach the release API"
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

	cur := strings.TrimPrefix(Version, "v")
	latestTag := fetchLatestTag() // "vX.Y.Z" (with the v), "" if unreachable
	latest := strings.TrimPrefix(latestTag, "v")

	if jsonOut {
		// Machine-readable check for the menu-bar app's "check for updates" — factors in
		// the app's OWN version so a stale app (CLI already current) still prompts.
		b, _ := json.Marshal(updateCheckPayload(cur, installedAppVersion(), latest))
		fmt.Println(string(b))
		return 0
	}

	if checkOnly {
		switch {
		case latest == "":
			i18n.Sae("gtmux update: couldn't reach the release API (network?).",
				"gtmux update: 连不上发布接口（网络问题？）。")
			return 1
		case latest == cur:
			i18n.Say("gtmux is up to date ("+cur+").", "gtmux 已是最新（"+cur+"）。")
		default:
			i18n.Say("update available: "+cur+" → "+latest+"  (run `gtmux update`)",
				"有新版本："+cur+" → "+latest+"  （运行 `gtmux update`）")
		}
		return 0
	}

	// The menu-bar app updates via THIS command, so "already up to date" must consider
	// the app too — else a stale app (CLI already current) short-circuits here and the app
	// can never refresh itself (the reported loop: click update → "already up to date" →
	// app never swapped → "Update failed"). When the app is behind, fall through and run
	// the installer's app step. (--cli-only ignores the app by design.)
	appV := ""
	if !cliOnly {
		appV = installedAppVersion()
	}
	appBehind := appV != "" && latest != "" && appV != latest

	if latest != "" && latest == cur && !appBehind {
		i18n.Say("gtmux is already up to date ("+cur+").", "gtmux 已是最新（"+cur+"）。")
		return 0
	}
	switch {
	case latest != "" && latest == cur && appBehind:
		i18n.Say("Updating the menu-bar app "+appV+" → "+latest+" (CLI already "+cur+") …",
			"正在更新菜单栏 app "+appV+" → "+latest+"（CLI 已是 "+cur+"）…")
	case latest != "":
		i18n.Say("Updating gtmux "+cur+" → "+latest+" …", "正在更新 gtmux "+cur+" → "+latest+" …")
	default:
		i18n.Say("Updating gtmux to the latest release…", "正在更新 gtmux 到最新版…")
	}

	rc := diag.DidRC("act.update", "gtmux", runInstaller(cliOnly, latestTag), "updated gtmux",
		"from", cur, "to", latest, "cliOnly", cliOnly)
	if rc == 0 {
		printWhatChanged(cur, latest)
		if os.Getenv("GTMUX_ACTOR") != "menubar" {
			postUpdateDoctor(installedCLIPath(), 90*time.Second)
		}
	}
	return rc
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
func runInstaller(cliOnly bool, version string) int {
	installer, err := newReleaseInstaller(cliOnly, version)
	if err == nil {
		err = installer.install()
	}
	if err != nil {
		i18n.Sae("gtmux: update failed: "+err.Error(), "gtmux: 更新失败："+err.Error())
		return 1
	}
	restartServeAgents()
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

// fetchLatestTag returns the latest release tag (e.g. "v0.6.0"), or "" on failure.
// It tries the GitHub API first, then jsdelivr's data API — which stays reachable on
// corp/CN networks that block api.github.com, so a `gtmux update` there can still
// resolve the version; installation still requires authoritative GitHub assets.
func fetchLatestTag() string {
	if b := httpGetBytes("https://api.github.com/repos/chenchaoyi/gtmux/releases/latest", 10*time.Second); b != nil {
		if tag := parseTagName(string(b)); tag != "" {
			return tag
		}
	}
	if b := httpGetBytes("https://data.jsdelivr.com/v1/packages/gh/chenchaoyi/gtmux", 10*time.Second); b != nil {
		if tag := parseJsdelivrLatest(string(b)); tag != "" {
			return tag
		}
	}
	return ""
}

// parseTagName pulls "tag_name":"vX.Y.Z" out of the GitHub releases/latest JSON
// (a minimal scan — no struct). "" if absent.
func parseTagName(body string) string {
	const key = `"tag_name":`
	i := strings.Index(body, key)
	if i < 0 {
		return ""
	}
	rest := body[i+len(key):]
	q1 := strings.Index(rest, `"`)
	if q1 < 0 {
		return ""
	}
	rest = rest[q1+1:]
	q2 := strings.Index(rest, `"`)
	if q2 < 0 {
		return ""
	}
	return rest[:q2]
}

// parseJsdelivrLatest reads the newest version from jsdelivr's package data API,
// whose `versions` list is sorted newest-first. Returns it tag-shaped ("vX.Y.Z").
func parseJsdelivrLatest(body string) string {
	var d struct {
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	}
	if json.Unmarshal([]byte(body), &d) != nil || len(d.Versions) == 0 {
		return ""
	}
	v := strings.TrimSpace(d.Versions[0].Version)
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
