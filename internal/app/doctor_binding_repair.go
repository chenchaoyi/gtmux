package app

import (
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func (s *fixState) stepConversationBindings() int {
	return s.repairConversationBindings(radar.GatherPanes())
}

func (s *fixState) repairConversationBindings(panes []radar.PaneRow) int {
	applied := 0
	for _, pane := range panes {
		if pane.Tier != "agent" || pane.Loc == "" {
			continue
		}
		repair, ok := resume.PlanConversationBindingRepair(pane.Loc, pane.PaneID)
		if !ok || !s.ask(i18n.Tr("Restore the verified conversation link for ", "恢复已核实的对话关联 ")+pane.PaneID,
			i18n.Tr("Use the current Codex connection for the same conversation. This only changes its saved reference.",
				"已核实当前 Codex 连接属于同一段对话。只修正保存的关联。")) {
			continue
		}
		if err := repair.Apply(); err != nil {
			i18n.Sae("  Could not verify the repair for "+pane.PaneID+". Check the remaining conversation findings.",
				"  未能验证 "+pane.PaneID+" 的修复，请查看剩余的会话绑定提示。")
			s.rc = 1
			continue
		}
		i18n.Say("  Restored the conversation link for "+pane.PaneID+".", "  已恢复 "+pane.PaneID+" 的对话关联。")
		applied++
	}
	return applied
}
