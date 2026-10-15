//go:build !darwin

package app

import "github.com/chenchaoyi/gtmux/internal/i18n"

func platformTerminalChecks() []dcheck {
	return []dcheck{{stInfo, i18n.Tr("desktop integration", "桌面集成"), i18n.Tr("headless host", "无桌面服务器"), i18n.Tr("Manage tmux here, or connect from the phone/browser. Desktop focus is unavailable.", "可在本机或手机、浏览器管理 tmux，不提供桌面窗口跳转。")}}
}
func platformSleepChecks() []dcheck { return nil }

func ensureCloudflared(bool) string {
	i18n.Sae("Install cloudflared from https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/ then retry. Direct does not require cloudflared.", "请先按 https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/ 安装 cloudflared 后重试。直连不需要此依赖。")
	return ""
}
func (s *fixState) stepCloudflared() int { return 0 }
func tmuxInstallHint() string {
	return i18n.Tr("install tmux using your system package manager (Debian/Ubuntu: apt install tmux)", "通过系统包管理器安装 tmux（Debian/Ubuntu：apt install tmux）")
}

func prepareHookDesktop() {}

func platformServiceChecks() []dcheck {
	var rows []dcheck
	for _, label := range []string{serveAgentLabel, tunnelAgentLabel, selfTunnelAgentLabel} {
		if !fileExists(serviceManager().Path(label)) {
			continue
		}
		status, value, note := stOK, i18n.Tr("running", "运行中"), i18n.Tr("systemd user service", "systemd 用户服务")
		if !serviceLoaded(label) {
			status = stRec
			value = i18n.Tr("not running", "未运行")
			note = i18n.Tr("Check systemctl --user status "+label+".service. Use foreground mode if no user manager is available.", "运行 systemctl --user status "+label+".service 检查；没有用户管理器时可改用前台模式。")
		}
		rows = append(rows, dcheck{status, label, value, note})
	}
	return rows
}
