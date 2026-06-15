package app

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/menubar"
)

//go:embed templates/Gtmux.Info.plist
var gtmuxInfoPlist string

const menubarBundleID = "com.gtmux.menubar" // separate from com.gtmux.focus (the notify click target)

func gtmuxAppPath() string { return filepath.Join(homeDir(), "Applications", "Gtmux.app") }

func launchAgentPath() string {
	return filepath.Join(homeDir(), "Library", "LaunchAgents", menubarBundleID+".plist")
}

// cmdInstallApp implements `gtmux install-app [--login] [--no-launch]`: wrap the
// gtmux-menubar binary in ~/Applications/Gtmux.app (bundle id com.gtmux.menubar,
// LSUIElement), ad-hoc sign it, register it, optionally add a login item, and
// launch it. The notification click target (GtmuxFocus.app / com.gtmux.focus)
// is untouched — this is a separate, persistent status-bar app.
func cmdInstallApp(args []string) int {
	login, launch := false, true
	for _, a := range args {
		switch a {
		case "--login":
			login = true
		case "--no-launch":
			launch = false
		case "-h", "--help":
			usage()
			return 0
		}
	}
	if runtime.GOOS != "darwin" {
		i18n.Sae("install-app is macOS-only", "install-app 仅支持 macOS")
		return 1
	}

	menubarBin, err := locateMenubarBin()
	if err != nil {
		i18n.Sae("gtmux-menubar binary not found — build it (make app) or set GTMUX_MENUBAR_BIN",
			"未找到 gtmux-menubar 可执行文件 —— 先构建(make app)或设置 GTMUX_MENUBAR_BIN")
		return 1
	}

	if err := writeGtmuxApp(menubarBin, selfPath()); err != nil {
		i18n.Sae("failed to build Gtmux.app: "+err.Error(), "构建 Gtmux.app 失败: "+err.Error())
		return 1
	}
	app := gtmuxAppPath()
	runQuiet("xattr", "-dr", "com.apple.quarantine", app)         // best-effort: clear quarantine
	runQuiet("codesign", "--force", "--deep", "--sign", "-", app) // best-effort: ad-hoc sign
	runQuiet(lsregister, "-f", app)
	i18n.Say("✓ Gtmux.app installed ("+menubarBundleID+")", "✓ 已安装 Gtmux.app ("+menubarBundleID+")")

	if login {
		if err := writeLoginAgent(app); err != nil {
			i18n.Sae("failed to register login item: "+err.Error(), "注册登录项失败: "+err.Error())
		} else {
			i18n.Say("✓ will start at login", "✓ 已设置开机自启")
		}
	}

	if launch {
		runQuiet("open", app)
		i18n.Say("✓ launched — look for the gtmux item in your menu bar", "✓ 已启动 —— 看菜单栏的 gtmux 图标")
	}
	i18n.Say("Tip: 'gtmux uninstall-app' removes it.", "提示:'gtmux uninstall-app' 可卸载。")
	return 0
}

// cmdUninstallApp reverses install-app.
func cmdUninstallApp(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			usage()
			return 0
		}
	}
	// Stop a running instance and drop the login item.
	runQuiet("launchctl", "unload", "-w", launchAgentPath())
	_ = os.Remove(launchAgentPath())
	runQuiet("pkill", "-f", filepath.Join("Gtmux.app", "Contents", "MacOS", "gtmux-menubar"))

	app := gtmuxAppPath()
	runQuiet(lsregister, "-u", app)
	if err := os.RemoveAll(app); err != nil {
		i18n.Sae("failed to remove Gtmux.app: "+err.Error(), "删除 Gtmux.app 失败: "+err.Error())
		return 1
	}
	i18n.Say("✓ removed Gtmux.app and its login item", "✓ 已删除 Gtmux.app 及登录项")
	return 0
}

// writeGtmuxApp materializes ~/Applications/Gtmux.app: the Info.plist, the
// menu-bar executable, and a copy of the CLI beside it (so the app's
// ResolveGtmux finds a version-matched gtmux without depending on $PATH).
func writeGtmuxApp(menubarBin, cliBin string) error {
	macOS := filepath.Join(gtmuxAppPath(), "Contents", "MacOS")
	if err := os.MkdirAll(macOS, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(gtmuxAppPath(), "Contents", "Info.plist"), []byte(gtmuxInfoPlist), 0o644); err != nil {
		return err
	}
	if err := copyExecutable(menubarBin, filepath.Join(macOS, "gtmux-menubar")); err != nil {
		return err
	}
	return copyExecutable(cliBin, filepath.Join(macOS, "gtmux"))
}

// writeLoginAgent installs a LaunchAgent that starts the app at login.
func writeLoginAgent(app string) error {
	exe := filepath.Join(app, "Contents", "MacOS", "gtmux-menubar")
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + menubarBundleID + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + exe + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
</dict>
</plist>
`
	if err := os.MkdirAll(filepath.Dir(launchAgentPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(launchAgentPath(), []byte(plist), 0o644); err != nil {
		return err
	}
	runQuiet("launchctl", "unload", launchAgentPath()) // ignore "not loaded"
	return exec.Command("launchctl", "load", "-w", launchAgentPath()).Run()
}

// locateMenubarBin resolves a real gtmux-menubar binary, erroring if none exists.
func locateMenubarBin() (string, error) {
	p := menubar.ResolveMenubar()
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return p, nil
	}
	if lp, err := exec.LookPath(p); err == nil {
		return lp, nil
	}
	return "", fmt.Errorf("gtmux-menubar not found")
}

// copyExecutable copies src to dst with 0755 perms.
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
