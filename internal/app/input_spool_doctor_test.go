package app

import (
	"os"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func TestInputSpoolDoctorShowsWeightedSaturation(t *testing.T) {
	original := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(original) })
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			i18n.SetLang(lang)
			for _, tc := range []struct {
				slots, names          int
				err                   error
				status                int
				value, noteEN, noteZH string
			}{
				{4, 2, nil, stOK, "4/100", "claims and receipts", "领取和回执"},
				{80, 42, nil, stRec, "80/100", "unresolved requests", "未决请求"},
				{97, 51, nil, stMiss, "97/100", "settlement reservations", "结算预留"},
				{0, 101, agentclient.ErrInputFull, stMiss, "101", "replay records", "防重记录"},
				{0, 0, os.ErrPermission, stMiss, "", "permissions", "读取权限"},
			} {
				row := inputSpoolDoctorRow(agentclient.InputSpoolUsage{
					UsedSlots: tc.slots, SlotLimit: 100, Entries: tc.names, ScanLimit: 100, Reservation: 4,
				}, tc.err)
				note := tc.noteEN
				if lang == "zh" {
					note = tc.noteZH
				}
				if row.status != tc.status || !strings.Contains(row.value, tc.value) ||
					!strings.Contains(row.note, note) || tc.err != nil && strings.Contains(row.value, "0/100") {
					t.Fatalf("unexpected doctor row: %+v", row)
				}
			}
		})
	}
}
