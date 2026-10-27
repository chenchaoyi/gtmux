package app

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func ensureCloudflaredMac(yes bool) string {
	bin := lookTool("cloudflared")
	action := "install"
	brew := lookTool("brew")
	if bin != "" {
		if _, err := checkCloudflared(bin); err == nil {
			return bin
		}
		action = "upgrade"
		brew = cloudflaredBrew(bin)
		if brew == "" {
			cloudflaredSecurityError(fmt.Errorf("update this non-Homebrew installation to 2026.10.3 or later"))
			return ""
		}
	}
	if brew == "" {
		cloudflaredSecurityError(fmt.Errorf("cloudflared is unavailable and Homebrew was not found"))
		return ""
	}
	if !yes && !confirm(i18n.Tr("Install/update cloudflared with its Homebrew? Existing long-lived connections may need to reconnect. [Y/n] ", "通过原 Homebrew 安装或更新 cloudflared？已有长连接重启后需要重新连接。[Y/n] ")) {
		return ""
	}
	i18n.Say("Updating the Standard tunnel client with its original Homebrew…", "正在通过原 Homebrew 安装或更新 Standard 隧道客户端…")
	command, args := brew, []string{action, "cloudflared"}
	// An existing Intel prefix on Apple silicon must run its own Homebrew under Rosetta.
	if runtime.GOARCH == "arm64" && strings.HasPrefix(brew, "/usr/local/") {
		command, args = "/usr/bin/arch", append([]string{"-x86_64", brew}, args...)
	}
	c := exec.Command(command, args...)
	c.Env = append(os.Environ(), "HOMEBREW_NO_INSTALL_CLEANUP=1")
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := c.Run(); err != nil {
		i18n.Sae("Homebrew did not complete the cloudflared update. Review its error; do not disable trust or TLS checks.", "Homebrew 未完成 cloudflared 更新。请按上方具体错误处理，不要关闭信任或 TLS 校验。")
		return ""
	}
	bin = lookTool("cloudflared")
	if bin == "" {
		return ""
	}
	if _, err := checkCloudflared(bin); err != nil {
		cloudflaredSecurityError(err)
		return ""
	}
	if serviceLoaded(tunnelAgentLabel) {
		i18n.Say("The installed client is updated. The existing Standard connector must be restarted to use it; long-lived connections will reconnect. Run gtmux tunnel --service --recover.", "已更新安装文件。已有 Standard 连接进程仍需重启，长连接会重新连接；请运行 gtmux tunnel --service --recover。")
	}
	return bin
}

func (s *fixState) stepCloudflaredMac() int {
	bin := lookTool("cloudflared")
	if bin != "" {
		if _, err := checkCloudflared(bin); err == nil {
			return 0
		}
		fmt.Println(i18n.Tr("cloudflared needs a security update.", "cloudflared 需要安全更新。"))
		if requireCloudflared(s.yes) == "" {
			s.rc = 1
			return 0
		}
		return 1
	}
	// Do not install an absent optional dependency in unattended hook setup.
	if s.yes || tunnelBackend() == "direct" {
		return 0
	}
	if !confirmRisky(i18n.Tr("Install the optional Standard tunnel client? [y/N] ", "安装可选的 Standard 隧道客户端？[y/N] ")) {
		return 0
	}
	if requireCloudflared(true) == "" {
		s.rc = 1
		return 0
	}
	return 1
}
