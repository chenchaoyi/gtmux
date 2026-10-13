package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func TestDoctorFinalSummaryFiltersAndPrioritizesWithProvenance(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	sections := []dsection{
		{"Logs", []dcheck{{stRec, "same label", "recent errors", "gtmux logs --since 1d"}, {stOK, "healthy only", "installed", ""}, {stInfo, "optional only", "not configured", ""}}},
		{"HQ", []dcheck{{stMiss, "same label", "missing prerequisite", "keep this full guidance"}, {stRec, "second recommendation", "old context", "manual action, not necessarily fixed by doctor --fix"}}},
	}
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		var out bytes.Buffer
		renderDoctorResults(&out, sections)
		text := out.String()
		complete := i18n.Tr("Check complete", "检查已完成")
		if !strings.Contains(text, complete) || strings.Contains(text, "待检查") {
			t.Fatalf("misleading completion: %s", text)
		}
		before := strings.Index(text, "HQ › same label")
		after := strings.Index(text, "Logs › same label")
		if before < 0 || after < 0 || before >= after {
			t.Fatalf("missing provenance or incorrect severity ordering: %s", text)
		}
		for _, required := range []string{"recent errors", "missing prerequisite", "gtmux logs --since 1d", "keep this full guidance", "manual action, not necessarily fixed by doctor --fix"} {
			if !strings.Contains(text, required) {
				t.Fatalf("guidance lost: %s", required)
			}
		}
		for _, excluded := range []string{"healthy only", "optional only"} {
			if strings.Contains(text, excluded) {
				t.Fatalf("non-actionable row listed: %s", text)
			}
		}
		if len(doctorFindings(sections, stRec)) != 2 || len(doctorFindings(sections, stMiss)) != 1 {
			t.Fatal("classification changed")
		}
	}
}

func TestDoctorNoFindingsDoesNotSuggestUnfinishedWork(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	sections := []dsection{{"Tools", []dcheck{{stOK, "healthy", "installed", ""}, {stInfo, "optional", "not installed", ""}}}}
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		var out bytes.Buffer
		renderDoctorResults(&out, sections)
		text := out.String()
		if !strings.Contains(text, i18n.Tr("Check complete", "检查已完成")) || !strings.Contains(text, i18n.Tr("1 informational item", "1 信息")) {
			t.Fatalf("missing completion/info count: %s", text)
		}
		if strings.Contains(text, "Tools ›") || strings.Contains(text, "待检查") {
			t.Fatalf("invented pending work: %s", text)
		}
	}
}
