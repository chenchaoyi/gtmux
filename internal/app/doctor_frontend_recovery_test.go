package app

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestDPC004DoctorDoesNotPromiseAutomaticRecovery(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	for _, lang := range []string{"en", "zh"} {
		for _, mode := range []string{"confirmed-stopped", "missing-proof"} {
			t.Run(lang+"/"+mode, func(t *testing.T) {
				i18n.SetLang(lang)
				pane, index, proof := doctorAssociationFixture(t)
				if mode == "missing-proof" {
					if err := os.Remove(proof); err != nil {
						t.Fatal(err)
					}
				} else {
					data, err := os.ReadFile(proof)
					if err != nil {
						t.Fatal(err)
					}
					var f resume.CodexFrontend
					if err := json.Unmarshal(data, &f); err != nil {
						t.Fatal(err)
					}
					f.BridgePID = 1 << 30 // Beyond the supported OS PID range, not a real process.
					data, _ = json.Marshal(f)
					if err := os.WriteFile(proof, data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				beforeIndex, _ := os.ReadFile(index)
				beforeProof, _ := os.ReadFile(proof)
				row := staleBindingsFor([]radar.PaneRow{pane}, time.Now().Unix(), func(string, resume.Record) bool { return false })
				if row.status != stMiss || strings.Contains(row.note, "will retry") || strings.Contains(row.note, "客户端会自动重试") {
					t.Fatal("unavailable association promised an unproven retry", row)
				}
				want := i18n.Tr("cannot be confirmed", "无法确认")
				if mode == "confirmed-stopped" {
					want = i18n.Tr("waiting cannot restore", "等待不会恢复")
				}
				if !strings.Contains(row.note, want) {
					t.Fatal("recovery guidance lost evidence distinction", row)
				}
				afterIndex, _ := os.ReadFile(index)
				afterProof, _ := os.ReadFile(proof)
				if !bytes.Equal(beforeIndex, afterIndex) || !bytes.Equal(beforeProof, afterProof) {
					t.Fatal("doctor changed the conversation")
				}
			})
		}
	}
}
