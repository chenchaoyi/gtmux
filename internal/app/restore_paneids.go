package app

import (
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Layouts record pane IDs at their spatial positions, not pane-index order.
// This receipt never supplies evidence to resume, input or pane cleanup.
func layoutPaneIDs(raw string) []string {
	if !layoutChecksumRe.MatchString(raw) {
		return nil
	}
	var ids []string
	seen := map[string]bool{}
	for _, match := range layoutLeafRe.FindAllStringSubmatch(raw, -1) {
		id := "%" + match[2]
		if seen[id] {
			return nil
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func correspondingPaneIDs(saved, live windowShape) bool {
	return saved.Panes > 0 && saved.Panes == live.Panes && len(saved.PaneIDs) == saved.Panes &&
		len(live.PaneIDs) == live.Panes && (saved.Panes == 1 || saved.Layout != "" && saved.Layout == live.Layout)
}

func renderRestorePaneIDs(saved, live []windowShape, width int) string {
	if len(saved) == 0 || len(live) == 0 {
		return ""
	}
	width = maxInt(20, min(width, 100))
	var b strings.Builder
	b.WriteByte('\n')
	sessions := map[string]bool{}
	panes := 0
	for _, w := range saved {
		sessions[w.Session] = true
		panes += w.Panes
	}
	for _, part := range i18n.WrapDisp(fmt.Sprintf(i18n.Tr("Pane IDs · %d sessions · %d panes", "窗格对应 · %d 个会话 · %d 个窗格"), len(sessions), panes), width) {
		b.WriteString(part + "\n")
	}
	nameWidth := min(32, width/2-2)
	contentWidth := width - nameWidth - 4
	b.WriteString("  " + i18n.PadRight(i18n.TruncDisp(i18n.Tr("Session:window", "会话:窗口"), nameWidth), nameWidth) + "  " + i18n.TruncDisp(i18n.Tr("Backup → current", "备份 → 现在"), contentWidth) + "\n")
	b.WriteString("  " + strings.Repeat("─", width-2) + "\n")
	byKey := map[string]windowShape{}
	for _, w := range live {
		byKey[w.key()] = w
	}
	write := func(name, text string) {
		for i, part := range wrapRestoreCell(text, contentWidth) {
			label := ""
			if i == 0 {
				label = i18n.TruncDisp(restoreInline(name), nameWidth)
			}
			b.WriteString("  " + i18n.PadRight(label, nameWidth) + "  " + part + "\n")
		}
	}
	for _, s := range saved {
		l, exists := byKey[s.key()]
		if exists && correspondingPaneIDs(s, l) {
			var pairs []string
			for i, id := range s.PaneIDs {
				pairs = append(pairs, id+" → "+l.PaneIDs[i])
			}
			write(s.key(), strings.Join(pairs, " · "))
			continue
		}
		oldIDs := strings.Join(s.PaneIDs, " ")
		if oldIDs == "" {
			oldIDs = i18n.Tr("not recorded or unreadable", "未记录或无法读取")
		}
		text := i18n.Tr("Backup: ", "备份：") + oldIDs
		if !exists {
			write(s.key(), text+" / "+i18n.Tr("Current: window missing", "现在：窗口未恢复"))
			continue
		}
		newIDs := strings.Join(l.PaneIDs, " ")
		if newIDs == "" {
			newIDs = i18n.Tr("unavailable", "无法读取")
		}
		write(s.key(), text+" / "+i18n.Tr("Current: ", "现在：")+newIDs+" · "+i18n.Tr("Individual matches unconfirmed", "逐一对应未确认"))
	}
	b.WriteByte('\n')
	return b.String()
}
