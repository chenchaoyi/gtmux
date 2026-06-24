package app

import (
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// The menu-bar app (Gtmux.app, bundle id com.gtmux.menubar) is the native Swift
// app in macapp/. It's distributed prebuilt — installed by the curl installer
// (install.sh) or built from source via macapp/build.sh — so there's no Go
// "install-app" anymore. uninstall-app stays for a clean removal.

const menubarBundleID = "com.gtmux.menubar"

func gtmuxAppPath() string { return filepath.Join(homeDir(), "Applications", "Gtmux.app") }

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
