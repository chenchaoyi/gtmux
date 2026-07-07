package app

import (
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

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// updateCheckJSON is the `gtmux update --check --json` payload the menu-bar app
// reads to decide whether to surface a "new version available" prompt.
type updateCheckJSON struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Update  bool   `json:"update"`
	Error   string `json:"error,omitempty"`
}

// updateCheckPayload builds the --json result from the current + latest versions.
// An empty latest means the release API was unreachable (surfaced via `error`, and
// `update` stays false so the app never prompts on a failed check).
func updateCheckPayload(cur, latest string) updateCheckJSON {
	out := updateCheckJSON{Current: cur, Latest: latest, Update: latest != "" && latest != cur}
	if latest == "" {
		out.Error = "couldn't reach the release API"
	}
	return out
}

// cmdUpdate implements `gtmux update` — self-update to the latest release by
// driving the maintained installer (which fetches + SHA-verifies the CLI tarball
// AND the menu-bar app, with the same CN mirror fallback as the curl install).
// `--check` only reports; `--cli-only` skips the app. Updating in place is safe:
// the installer atomic-swaps the binary, and a running executable keeps its inode.
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
		// Machine-readable check for the menu-bar app's "check for updates".
		b, _ := json.Marshal(updateCheckPayload(cur, latest))
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

	if latest != "" && latest == cur {
		i18n.Say("gtmux is already up to date ("+cur+").", "gtmux 已是最新（"+cur+"）。")
		return 0
	}
	if latest != "" {
		i18n.Say("Updating gtmux "+cur+" → "+latest+" …", "正在更新 gtmux "+cur+" → "+latest+" …")
	} else {
		i18n.Say("Updating gtmux to the latest release…", "正在更新 gtmux 到最新版…")
	}

	return runInstaller(cliOnly, latestTag)
}

// runInstaller fetches the official install.sh and runs it in place (over the
// running binary's dir), installing the CLI + menu-bar app unless cliOnly. When
// `version` is a resolved tag (e.g. "v0.12.40"), it's handed to install.sh as
// GTMUX_VERSION so the script skips its OWN release resolution — which otherwise
// re-does (and can re-fail) the api.github.com/mirror lookup we already did in Go.
// Returns 0 on success. Shared by `gtmux update` and `doctor --fix`'s app step.
func runInstaller(cliOnly bool, version string) int {
	script := fetchInstallScript()
	if script == nil {
		// Every mirror failed → almost always the network can't reach GitHub/the CDN
		// (VPN, corp firewall, offline). Say so, and print the exact command to run by
		// hand — including the CDN mirror that works behind most firewalls — instead of
		// pointing at the README.
		gh := installScriptMirrors[0]
		cdn := installScriptMirrors[1]
		i18n.Sae("gtmux: can't reach the release server — check your internet / VPN, then run the install command by hand:",
			"gtmux: 连不上发布服务器 —— 请检查网络 / VPN，然后手动运行安装命令：")
		fmt.Fprintf(os.Stderr, "\n  curl -fsSL %s | bash\n", gh)
		i18n.Sae("  behind a firewall? use the CDN mirror:", "  在防火墙后？用 CDN 镜像：")
		fmt.Fprintf(os.Stderr, "  curl -fsSL %s | bash\n\n", cdn)
		return 1
	}
	tmp, err := os.CreateTemp("", "gtmux-install-*.sh")
	if err != nil {
		i18n.Sae("gtmux: "+err.Error(), "gtmux: "+err.Error())
		return 1
	}
	defer os.Remove(tmp.Name())
	_, _ = tmp.Write(script)
	_ = tmp.Close()

	cmd := exec.Command("bash", tmp.Name())
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	env := os.Environ()
	// Hand install.sh the version we already resolved so it doesn't re-resolve it
	// (and re-fail if the release API blips) — but never override a version the user
	// pinned themselves via the environment.
	if version != "" {
		if _, ok := os.LookupEnv("GTMUX_VERSION"); !ok {
			env = append(env, "GTMUX_VERSION="+version)
		}
	}
	// Install IN PLACE: over the binary that's running (its dir), unless the user
	// already pinned a dir.
	if _, ok := os.LookupEnv("GTMUX_BIN_DIR"); !ok {
		if self, e := os.Executable(); e == nil {
			if real, e2 := filepath.EvalSymlinks(self); e2 == nil {
				self = real
			}
			env = append(env, "GTMUX_BIN_DIR="+filepath.Dir(self))
		}
	}
	if cliOnly {
		env = append(env, "GTMUX_NO_APP=1")
	}
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		i18n.Sae("gtmux: installer failed: "+err.Error(), "gtmux: 安装失败："+err.Error())
		return 1
	}
	// The installer swapped the binary on disk, but a long-running `gtmux serve`
	// LaunchAgent keeps executing the OLD in-memory binary — so the phone app /
	// browser mirror it backs would keep serving stale assets (e.g. "no chat mode"
	// after updating) until the next logout/reboot. Re-exec it now.
	restartServeAgents()
	return 0
}

// restartServeAgents re-execs the persistent `gtmux serve` LaunchAgent on macOS so
// a freshly-installed binary takes effect immediately. No-op when the agent isn't
// loaded or off macOS. cloudflared (the separate tunnel agent) reconnects on its
// own, so only the serve needs kicking.
func restartServeAgents() {
	if runtime.GOOS != "darwin" {
		return
	}
	target := "gui/" + strconv.Itoa(os.Getuid()) + "/com.gtmux.serve"
	// Only restart if it's actually loaded — otherwise there's nothing persistent
	// to refresh (a one-off `gtmux serve` is the user's own process to manage).
	if err := exec.Command("launchctl", "print", target).Run(); err != nil {
		return
	}
	if err := exec.Command("launchctl", "kickstart", "-k", target).Run(); err == nil {
		i18n.Say("Restarted the remote serve so the update takes effect.",
			"已重启远程 serve，更新即时生效。")
	}
}

// installScriptMirrors lists the installer URL GitHub-first, then CN proxies (the
// installer's INTERNAL downloads already fall back; this only covers fetching the
// script itself).
var installScriptMirrors = []string{
	"https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh",
	// jsdelivr's CDN serves repo files reliably from inside CN — best fallback.
	"https://cdn.jsdelivr.net/gh/chenchaoyi/gtmux@main/install.sh",
	"https://gh-proxy.com/https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh",
	"https://ghfast.top/https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh",
	"https://ghproxy.net/https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh",
}

func fetchInstallScript() []byte {
	for _, u := range installScriptMirrors {
		if b := httpGetBytes(u, 20*time.Second); b != nil && strings.Contains(string(b), "gtmux") {
			return b
		}
	}
	return nil
}

// fetchLatestTag returns the latest release tag (e.g. "v0.6.0"), or "" on failure.
// It tries the GitHub API first, then jsdelivr's data API — which stays reachable on
// corp/CN networks that block api.github.com, so a `gtmux update` there can still
// resolve the version (and hand it to install.sh) instead of dead-ending.
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
