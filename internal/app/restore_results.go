package app

import (
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

type resumeAttempt struct {
	Pane, Loc, State string
	Record, Before   resume.Record
	IssuedAt         int64
}

func resumeAttemptConfirmed(attempt resumeAttempt, current resume.Record, live bool) bool {
	return live && current.Agent == attempt.Record.Agent && current.SessionID == attempt.Record.SessionID &&
		current.HomeID == attempt.Record.HomeID && current.UpdatedAt >= attempt.IssuedAt &&
		(current.Frontend != "" && current.Frontend != attempt.Before.Frontend || current.UpdatedAt > attempt.Before.UpdatedAt)
}

func resumeStateLabel(state string) string {
	switch state {
	case "confirmed":
		return i18n.Tr("Confirmed", "已确认")
	case "pending":
		return i18n.Tr("Command sent · awaiting startup", "命令已发送 · 待启动确认")
	case "prefilled":
		return i18n.Tr("Prefilled · press Enter in pane", "已预填 · 到窗格按回车")
	case "missing":
		return i18n.Tr("Skipped · no saved conversation", "已跳过 · 对话尚未保存或已不存在")
	case "unreadable":
		return i18n.Tr("Skipped · history unreadable", "已跳过 · 无法读取历史")
	case "invalid":
		return i18n.Tr("Skipped · invalid history", "已跳过 · 历史文件无效")
	case "foreign-home":
		return i18n.Tr("Skipped · different Codex home", "已跳过 · Codex 存储目录不同")
	}
	return i18n.Tr("Command could not be sent", "恢复命令未能发送")
}

func renderResumeAttempts(attempts []resumeAttempt, width int) string {
	if len(attempts) == 0 {
		return ""
	}
	width = maxInt(20, min(width, 100))
	confirmed, pending, skipped := 0, 0, 0
	for _, a := range attempts {
		switch a.State {
		case "confirmed":
			confirmed++
		case "pending", "prefilled":
			pending++
		default:
			skipped++
		}
	}
	var b strings.Builder
	line := func(s string) {
		for _, part := range wrapRestoreCell(s, width) {
			b.WriteString(part + "\n")
		}
	}
	line(i18n.Tr("Conversation startup", "对话启动结果"))
	line(fmt.Sprintf(i18n.Tr("%d confirmed · %d pending · %d skipped/failed", "%d 个已确认 · %d 个待确认 · %d 个跳过或失败"), confirmed, pending, skipped))
	nameWidth := min(32, width/3)
	for _, a := range attempts {
		name := a.Loc
		if name == "" {
			name = a.Pane
		}
		prefix := "  " + i18n.PadRight(i18n.TruncDisp(restoreInline(name), nameWidth), nameWidth) + "  "
		for i, part := range wrapRestoreCell(a.Pane+" · "+resumeStateLabel(a.State), width-nameWidth-4) {
			if i == 0 {
				b.WriteString(prefix + part + "\n")
			} else {
				b.WriteString(strings.Repeat(" ", nameWidth+4) + part + "\n")
			}
		}
	}
	line(i18n.Tr("Details: ", "详细记录：") + restoreTraceHint)
	return b.String()
}

// Preserve complete IDs and commands even when an unbroken token is wider than
// the column. WrapDisp deliberately leaves such words intact for normal prose.
func wrapRestoreCell(text string, width int) []string {
	width = maxInt(1, width)
	var out []string
	for _, part := range i18n.WrapDisp(text, width) {
		line := ""
		for _, r := range part {
			if line != "" && i18n.DispWidth(line+string(r)) > width {
				out = append(out, line)
				line = ""
			}
			line += string(r)
		}
		out = append(out, line)
	}
	return out
}
