package app

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// cmdUpdate implements `gtmux update` — self-update to the latest release by
// driving the maintained installer (which fetches + SHA-verifies the CLI tarball
// AND the menu-bar app, with the same CN mirror fallback as the curl install).
// `--check` only reports; `--cli-only` skips the app. Updating in place is safe:
// the installer atomic-swaps the binary, and a running executable keeps its inode.
func cmdUpdate(args []string) int {
	checkOnly, cliOnly := false, false
	for _, a := range args {
		switch a {
		case "-h", "--help":
			i18n.Say("usage: gtmux update [--check] [--cli-only]",
				"用法: gtmux update [--check] [--cli-only]")
			i18n.Say("  Update gtmux (CLI + menu-bar app) to the latest release.",
				"  把 gtmux(CLI + 菜单栏 app)更新到最新版。")
			i18n.Say("  --check: only report if a newer version exists. --cli-only: skip the app.",
				"  --check:只检查有无新版。--cli-only:只更新 CLI,不动 app。")
			return 0
		case "--check":
			checkOnly = true
		case "--cli-only":
			cliOnly = true
		default:
			i18n.Sae("gtmux update: unknown option '"+a+"'", "gtmux update: 未知选项 '"+a+"'")
			return 2
		}
	}

	cur := strings.TrimPrefix(Version, "v")
	latest := strings.TrimPrefix(fetchLatestTag(), "v")

	if checkOnly {
		switch {
		case latest == "":
			i18n.Sae("gtmux update: couldn't reach the release API (network?).",
				"gtmux update: 连不上发布接口(网络?)。")
			return 1
		case latest == cur:
			i18n.Say("gtmux is up to date ("+cur+").", "gtmux 已是最新("+cur+")。")
		default:
			i18n.Say("update available: "+cur+" → "+latest+"  (run `gtmux update`)",
				"有新版本:"+cur+" → "+latest+"  (运行 `gtmux update`)")
		}
		return 0
	}

	if latest != "" && latest == cur {
		i18n.Say("gtmux is already up to date ("+cur+").", "gtmux 已是最新("+cur+")。")
		return 0
	}
	if latest != "" {
		i18n.Say("Updating gtmux "+cur+" → "+latest+" …", "正在更新 gtmux "+cur+" → "+latest+" …")
	} else {
		i18n.Say("Updating gtmux to the latest release…", "正在更新 gtmux 到最新版…")
	}

	script := fetchInstallScript()
	if script == nil {
		i18n.Sae("gtmux update: couldn't download the installer. Use the manual curl command from the README.",
			"gtmux update: 下载安装脚本失败。可用 README 里的手动 curl 命令。")
		return 1
	}
	tmp, err := os.CreateTemp("", "gtmux-install-*.sh")
	if err != nil {
		i18n.Sae("gtmux update: "+err.Error(), "gtmux update: "+err.Error())
		return 1
	}
	defer os.Remove(tmp.Name())
	_, _ = tmp.Write(script)
	_ = tmp.Close()

	cmd := exec.Command("bash", tmp.Name())
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	env := os.Environ()
	// Update IN PLACE: install over the binary that's running (its dir), unless
	// the user already pinned a dir.
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
		i18n.Sae("gtmux update: installer failed: "+err.Error(), "gtmux update: 安装失败: "+err.Error())
		return 1
	}
	return 0
}

// installScriptMirrors lists the installer URL GitHub-first, then CN proxies (the
// installer's INTERNAL downloads already fall back; this only covers fetching the
// script itself).
var installScriptMirrors = []string{
	"https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh",
	"https://ghfast.top/https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh",
	"https://gh-proxy.com/https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh",
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
func fetchLatestTag() string {
	b := httpGetBytes("https://api.github.com/repos/chenchaoyi/gtmux/releases/latest", 10*time.Second)
	if b == nil {
		return ""
	}
	// Minimal parse: find "tag_name":"vX.Y.Z" without pulling in a JSON struct.
	const key = `"tag_name":`
	i := strings.Index(string(b), key)
	if i < 0 {
		return ""
	}
	rest := string(b)[i+len(key):]
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
