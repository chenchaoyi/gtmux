package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func TestSPL025ArchiveDoctorWarnsAboutCapacityAndRepair(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			original := i18n.Lang()
			i18n.SetLang(lang)
			t.Cleanup(func() { i18n.SetLang(original) })
			for _, tc := range []struct {
				health agentclient.InputArchiveHealth
				status int
				en, zh string
			}{
				{agentclient.InputArchiveHealth{LimitBytes: 100}, stOK, "readable", "可读取"},
				{agentclient.InputArchiveHealth{UsedBytes: 80, LimitBytes: 100}, stRec, "Back up", "备份"},
				{agentclient.InputArchiveHealth{AdmissionReserved: true}, stMiss, "paused", "暂停"},
				{agentclient.InputArchiveHealth{NeedsRepair: true}, stMiss, "verified backup", "已核验的备份"},
				{agentclient.InputArchiveHealth{RecoveryPending: true}, stRec, "checkpoint", "检查点"},
			} {
				row := inputArchiveDoctorRow(tc.health)
				word := tc.en
				if lang == "zh" {
					word = tc.zh
				}
				if row.status != tc.status || !strings.Contains(row.note, word) {
					t.Fatalf("unexpected doctor result: %+v", row)
				}
			}
		})
	}
}
