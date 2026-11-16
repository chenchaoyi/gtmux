package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func TestDPC002DoctorStorageExplainsScopeWithoutRemoval(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		for _, state := range []hq.ProcessMaterialState{
			{}, {UnclassifiedArchiveBytes: 218658}, {Files: 3, Bytes: 600, RelayBytes: 100, AttachmentBytes: 200, SourceBytes: 300},
			{Incomplete: true},
		} {
			row := hqProcessMaterialsRow(state)
			if state.UnclassifiedArchiveBytes > 0 && row.status != stInfo {
				t.Fatal("unknown archive was declared disposable", row)
			}
			var output bytes.Buffer
			writeDoctorJSON(&output, []dsection{{title: "Storage", rows: []dcheck{row}}}, time.Now())
			var report doctorReport
			if err := json.Unmarshal(output.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			got := report.Sections[0].Rows[0]
			if got.Action != "" || got.Command != "" {
				t.Fatal("read-only archive advice invented a fix", got)
			}
		}
		reason := doctorSnapshotFailure("HQ records exceed restore limits (128 MiB, 10000 entries); destination unchanged")
		if !strings.Contains(reason, "128 MiB") || !strings.Contains(reason, "10,000") || !strings.Contains(reason, i18n.Tr("safety limit", "安全上限")) || strings.Contains(reason, "10000 entries") {
			t.Fatal("recovery-package limit not explained", reason)
		}
		if doctorSnapshotFailure("synthetic disk read failed") != "synthetic disk read failed" {
			t.Fatal("unrelated backup error was hidden")
		}
	}
}
