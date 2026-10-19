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
		return dcheck{stRec, label, i18n.Tr("dst is not installed", "未安装 dst"), i18n.Tr("Web histories remain available; install a supported terminal client for pane control", "Web 对话仍可读取；控制终端会话需安装支持的终端客户端")}
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
		return dcheck{status, label, fmt.Sprintf("dst %s · %d verified · %d with input grant", pkg.Version, attached, ready), i18n.Tr("Fresh native attachments are verified; message consumption does not declare HQ review", "已核验实时原生绑定；读取消息不会自动标记 HQ 已审阅")}
	}
	return dcheck{stRec, label, i18n.Tr("no verified native terminal attachment", "尚无已核验的原生终端绑定"), i18n.Tr("Initialize the dst profile once, install hooks, then launch dst --backend dsh. Existing processes need a deliberate restart; doctor does not restart them. Inspect dst startup diagnostics if the bridge stays absent.", "先完成 dst 首次 profile 初始化，再安装 hooks 并启动 dst --backend dsh。已有进程需择机重开，doctor 不会替你重启。若仍无绑定，请检查 dst 启动诊断。")}
}
