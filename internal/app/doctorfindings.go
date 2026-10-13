package app

import (
	"fmt"
	"io"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

type doctorFinding struct {
	section string
	row     dcheck
}

func doctorFindings(sections []dsection, status int) []doctorFinding {
	var out []doctorFinding
	for _, section := range sections {
		for _, row := range section.rows {
			if row.status == status {
				out = append(out, doctorFinding{section.title, row})
			}
		}
	}
	return out
}

func renderDoctorResults(w io.Writer, sections []dsection) {
	fmt.Fprint(w, "  ", i18n.Tr("Check complete", "检查已完成"))
	for _, group := range []struct {
		status int
		name   string
	}{
		{stOK, i18n.Tr("passed", "正常")},
		{stRec, i18n.Tr("recommendations", "建议处理")},
		{stMiss, i18n.Tr("feature issues", "影响功能")},
		{stInfo, i18n.Tr("informational items", "信息")},
	} {
		count := len(doctorFindings(sections, group.status))
		if i18n.Lang() == "en" && count == 1 {
			switch group.status {
			case stRec:
				group.name = "recommendation"
			case stMiss:
				group.name = "feature issue"
			case stInfo:
				group.name = "informational item"
			}
		}
		_, color := statusGlyph(group.status)
		if count == 0 {
			color = i18n.Dim
		}
		fmt.Fprintf(w, " · %s%d %s%s", color, count, group.name, i18n.Reset)
	}
	fmt.Fprintln(w)
	renderDoctorFindings(w, sections)
}

// Completion is a probe lifecycle state. Recommendations and blocking findings
// are results, not unfinished probes; preserve every item's original guidance.
func renderDoctorFindings(w io.Writer, sections []dsection) {
	for _, group := range []struct {
		status int
		title  string
	}{
		{stMiss, i18n.Tr("Affects features", "影响功能")},
		{stRec, i18n.Tr("Recommendations", "建议处理")},
	} {
		findings := doctorFindings(sections, group.status)
		if len(findings) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n  %s%s (%d)%s\n", i18n.Bold, group.title, len(findings), i18n.Reset)
		for _, finding := range findings {
			row := finding.row
			glyph, color := statusGlyph(row.status)
			title, _ := doctorReportCopy(row.label)
			fmt.Fprintf(w, "    %s%s%s  %s › %s", color, glyph, i18n.Reset, finding.section, title)
			if row.value != "" {
				fmt.Fprintf(w, " — %s", row.value)
			}
			fmt.Fprintln(w)
			if row.note != "" {
				fmt.Fprintf(w, "       %s\n", row.note)
			}
		}
	}
}
