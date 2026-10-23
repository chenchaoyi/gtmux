package app

import (
	"fmt"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func rowDSHTerminal() dcheck {
	label := "DeepSeek Harness terminal"
	if lookTool("dst") == "" {
		return dcheck{stInfo, label, i18n.Tr("optional terminal client not found", "未找到可选终端客户端"), i18n.Tr("Web conversations remain readable. Install dst only if you want to manage DeepSeek terminal sessions; no installation is needed for Web history.", "Web 对话可正常读取。只有需要在终端管理 DeepSeek 会话时才需安装 dst；读取 Web 历史不需要它。")}
	}
	var pkg struct{ Version string }
	_ = agentclient.ReadOwnedJSON(filepath.Join(dsh.Home(), "profiles", "dsh-tui", "node_modules", "@deepseek-harness-tui", "dsh-tui", "package.json"), &pkg, 128<<10)
	attached, ready := 0, 0
	for _, a := range agentclient.All() {
		if a.Agent == "dsh" && a.Client == "dst" {
			attached++
			if a.InputReady {
				ready++
			}
		}
	}
	if attached > 0 {
		status := stOK
		if ready < attached {
			status = stRec
		}
		note := i18n.Tr("Live DeepSeek terminal sessions are verified. Reading messages does not mark HQ review.", "已核验正在运行的 DeepSeek 终端会话；读取消息不会自动标记为 HQ 已审阅。")
		if ready < attached {
			note = i18n.Tr("Some sessions have no verified input capability yet. Their status is readable, but sending is unavailable; inspect the dst startup diagnostics.", "部分会话的输入能力尚未通过验证。目前可查看状态，但不能发送消息；请查看 dst 的启动诊断。")
		}
		return dcheck{status, label, fmt.Sprintf(i18n.Tr("%d connected · %d accept messages", "%d 条已接入 · %d 条可发送消息"), attached, ready), note + " dst " + pkg.Version}
	}
	return dcheck{stInfo, label, i18n.Tr("no terminal session is connected to gtmux", "当前没有接入 gtmux 的终端会话"), i18n.Tr("Initialize the dst profile once, install hooks, then launch dst --backend dsh. Existing processes need a deliberate restart; doctor does not restart them. Inspect dst startup diagnostics if the bridge stays absent.", "先完成 dst 首次 profile 初始化，再安装 hooks 并启动 dst --backend dsh。已有进程需择机重开，doctor 不会替你重启。若仍无绑定，请检查 dst 启动诊断。")}
}
