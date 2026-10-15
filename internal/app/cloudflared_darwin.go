package app

import (
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"os"
	"os/exec"
)

func ensureCloudflaredMac(yes bool) string {
	i18n.Say("cloudflared isn't installed. It's the Cloudflare tunnel client (one binary, host-side only; the mobile app never touches it).",
		"未检测到 cloudflared，它是 Cloudflare 隧道客户端（一个二进制，只在 服务器上运行；手机 App 完全不碰它）。")
	if lookTool("brew") == "" {
		i18n.Sae("Anywhere needs cloudflared, and Homebrew isn't installed to fetch it. Install cloudflared, then retry: https://github.com/cloudflare/cloudflared/releases",
			"任意网络访问需要 cloudflared，但未安装 Homebrew 来获取它。请手动安装 cloudflared 后重试：https://github.com/cloudflare/cloudflared/releases")
		return ""
	}
	if !yes && !confirm(i18n.Tr("Install it now with `brew install cloudflared`? [Y/n] ",
		"现在用 `brew install cloudflared` 安装？[Y/n] ")) {
		i18n.Say("Skipped. Install it yourself, then re-run `gtmux tunnel`.",
			"已跳过。自行安装后重试 `gtmux tunnel`。")
		return ""
	}
	i18n.Say("Installing cloudflared (brew install cloudflared)…", "正在安装 cloudflared（brew install cloudflared）…")
	c := exec.Command(lookTool("brew"), "install", "cloudflared")
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := c.Run(); err != nil {
		i18n.Sae("gtmux tunnel: `brew install cloudflared` failed: "+err.Error(),
			"gtmux tunnel: `brew install cloudflared` 失败："+err.Error())
		return ""
	}
	bin := lookTool("cloudflared")
	if bin == "" {
		i18n.Sae("gtmux tunnel: cloudflared still not found after install.",
			"gtmux tunnel: 安装后仍未找到 cloudflared。")
		return ""
	}
	return bin
}

func (s *fixState) stepCloudflaredMac() int {
	if lookTool("cloudflared") != "" {
		return 0
	}
	fmt.Printf("\n%s%s%s\n", i18n.Bold,
		i18n.Tr("cloudflared  (optional, remote phone access)", "cloudflared（可选，手机远程访问）"), i18n.Reset)
	fmt.Printf("%s%s%s\n", i18n.Dim, i18n.Tr(
		"  Install it so `gtmux tunnel` can reach your Mac from anywhere. Only needed\n  for remote access, so skip it if you don't use the mobile app away from home.",
		"  装上它，`gtmux tunnel` 就能从任何地方连回你的 Mac。仅远程访问需要，\n  不在外面用手机 App 可跳过。"), i18n.Reset)
	if lookTool("brew") == "" {
		i18n.Say("  • brew not found. Install it from https://github.com/cloudflare/cloudflared/releases",
			"  • 未找到 brew，从 https://github.com/cloudflare/cloudflared/releases 安装")
		return 0
	}
	if s.yes {
		i18n.Say("  • skipped (optional). Run `gtmux tunnel` or `brew install cloudflared` when you want remote access.",
			"  • 已跳过（可选）。想远程时跑 `gtmux tunnel` 或 `brew install cloudflared`。")
		return 0
	}
	if !confirmRisky(i18n.Tr("  install it now? [y/N] ", "  现在安装？[y/N] ")) {
		i18n.Say("  skipped.", "  已跳过。")
		return 0
	}
	i18n.Say("  • brew install cloudflared …", "  • brew install cloudflared …")
	c := exec.Command(lookTool("brew"), "install", "cloudflared")
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		i18n.Sae("  ✗ brew install failed: "+err.Error(), "  ✗ brew 安装失败："+err.Error())
		s.rc = 1
		return 0
	}
	i18n.Say("  ✓ installed cloudflared. Run `gtmux tunnel` to go live", "  ✓ 已安装 cloudflared，跑 `gtmux tunnel` 即可上线")
	return 1
}
