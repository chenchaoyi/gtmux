package app

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// The menu-bar app (Gtmux.app, bundle id com.gtmux.menubar) is the native Swift
// app in macapp/. It's distributed prebuilt — installed by the curl installer
// (install.sh) or built from source via macapp/build.sh — so there's no Go
// "install-app" anymore. uninstall-app stays for a clean removal.

const menubarBundleID = "com.gtmux.menubar"

func gtmuxAppPath() string { return filepath.Join(homeDir(), "Applications", "Gtmux.app") }

// installedAppPath returns where Gtmux.app actually is (~/Applications preferred, the
// Homebrew-cask default /Applications as a fallback), or "" if it isn't installed.
func installedAppPath() string {
	for _, p := range []string{gtmuxAppPath(), filepath.Join("/Applications", "Gtmux.app")} {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// cmdApp launches the menu-bar app (Gtmux.app). Handy after install/reboot, or when
// a teammate can't find how to start it — it's a menu-bar-only app (no dock icon),
// so `open` puts the status dot in the top-right menu bar.
func cmdApp(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			usage()
			return 0
		}
	}
	app := installedAppPath()
	if app == "" {
		i18n.Say("Gtmux.app isn't installed. Install it, then re-run `gtmux app`:",
			"未安装 Gtmux.app。装好后再跑 `gtmux app`：")
		i18n.Say("  brew install --cask chenchaoyi/tap/gtmux-app   (or `gtmux doctor --fix`)",
			"  brew install --cask chenchaoyi/tap/gtmux-app   （或 `gtmux doctor --fix`）")
		return 1
	}
	if err := exec.Command("open", app).Run(); err != nil {
		i18n.Sae("couldn't launch Gtmux.app: "+err.Error(), "启动 Gtmux.app 失败："+err.Error())
		return 1
	}
	i18n.Say("✓ launched the menu-bar app — look for the status dot in the top-right menu bar (no dock icon).",
		"✓ 已启动菜单栏 app —— 看屏幕右上角菜单栏的状态点（无 dock 图标）。")
	return 0
}

func launchAgentPath() string {
	return filepath.Join(homeDir(), "Library", "LaunchAgents", menubarBundleID+".plist")
}

// cmdUninstallApp removes ~/Applications/Gtmux.app, its login item, and stops a
// running instance.
func cmdUninstallApp(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			usage()
			return 0
		}
	}
	runQuiet("launchctl", "unload", "-w", launchAgentPath())
	_ = os.Remove(launchAgentPath())
	runQuiet("pkill", "-f", filepath.Join("Gtmux.app", "Contents", "MacOS", "GtmuxBar"))

	app := gtmuxAppPath()
	runQuiet(lsregister, "-u", app)
	if err := os.RemoveAll(app); err != nil {
		i18n.Sae("failed to remove Gtmux.app: "+err.Error(), "删除 Gtmux.app 失败："+err.Error())
		return 1
	}
	i18n.Say("✓ removed Gtmux.app and its login item", "✓ 已删除 Gtmux.app 及登录项")
	return 0
}
