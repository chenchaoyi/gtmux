package app

import (
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
	for _, part := range i18n.WrapDisp(i18n.Tr("Pane IDs: backup → current (same layout position)", "窗格 ID：备份 → 现在（对应同一布局位置）"), width) {
		b.WriteString(part + "\n")
	}
	byKey := map[string]windowShape{}
	for _, w := range live {
		byKey[w.key()] = w
	}
	write := func(text string) {
		for _, part := range i18n.WrapDisp(text, width-4) {
			b.WriteString("    " + part + "\n")
		}
	}
	for _, s := range saved {
		b.WriteString("  " + i18n.TruncDisp(restoreInline(s.key()), width-2) + "\n")
		l, exists := byKey[s.key()]
		if exists && correspondingPaneIDs(s, l) {
			var pairs []string
			for i, id := range s.PaneIDs {
				pairs = append(pairs, id+" → "+l.PaneIDs[i])
			}
			write(strings.Join(pairs, " · "))
			continue
		}
		oldIDs := strings.Join(s.PaneIDs, " ")
		if oldIDs == "" {
			oldIDs = i18n.Tr("not recorded or unreadable", "未记录或无法读取")
		}
		write(i18n.Tr("Backup: ", "备份：") + oldIDs)
		if !exists {
			write(i18n.Tr("Current: window missing", "现在：窗口未恢复"))
			continue
		}
		newIDs := strings.Join(l.PaneIDs, " ")
		if newIDs == "" {
			newIDs = i18n.Tr("unavailable", "无法读取")
		}
		write(i18n.Tr("Current: ", "现在：") + newIDs)
		write(i18n.Tr("Individual matches could not be confirmed.", "无法确定逐一对应关系。"))
	}
	b.WriteByte('\n')
	return b.String()
}
