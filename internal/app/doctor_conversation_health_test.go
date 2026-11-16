package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func doctorAssociationFixture(t *testing.T) (radar.PaneRow, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	now := time.Now()
	writeCodexSession(t, transcript.CodexHome(), "2026/01/01", "doctor-root", "/synthetic", now.Add(-time.Minute))
	pane := radar.PaneRow{Tier: "agent", Agent: "Codex", PaneID: "%9", Loc: "synthetic:0.0", Title: "synthetic doctor"}
	record := resume.Record{Agent: "codex", SessionID: "doctor-root", HomeID: transcript.CodexHomeID(), Cwd: "/synthetic", Frontend: "0123456789abcdef0123456789abcdef"}
	index := filepath.Join(resume.Dir(), base64.RawURLEncoding.EncodeToString([]byte(pane.Loc))+".json")
	proof := filepath.Join(resume.Dir(), "codex-frontends", record.Frontend+".json")
	if err := os.MkdirAll(filepath.Dir(proof), 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(record)
	if err := os.WriteFile(index, data, 0600); err != nil {
		t.Fatal(err)
	}
	f := resume.CodexFrontend{Instance: record.Frontend, HomeID: record.HomeID, SessionID: record.SessionID,
		BridgePID: 101, BridgeStart: "bridge", BackendPID: 200, BackendStart: "backend", Sequence: 1,
		StartedAt: now.Add(-time.Minute).UnixMilli(), UpdatedAt: now.Add(-10 * time.Second).UnixMilli(),
		CodexBindingTarget: resume.CodexBindingTarget{Pane: pane.PaneID, Loc: pane.Loc, PID: 300, PanePID: 100,
			ClientStart: "client", RootStart: "root", TTY: "ttys999", Cwd: "/synthetic"}}
	data, _ = json.Marshal(f)
	if err := os.WriteFile(proof, data, 0600); err != nil {
		t.Fatal(err)
	}
	return pane, index, proof
}

func TestDCA001DoctorReportsFailedManagedAssociation(t *testing.T) {
	for _, mode := range []string{"expired", "missing-proof"} {
		t.Run(mode, func(t *testing.T) {
			pane, index, proof := doctorAssociationFixture(t)
			if mode == "missing-proof" {
				if err := os.Remove(proof); err != nil {
					t.Fatal(err)
				}
			}
			beforeIndex, _ := os.ReadFile(index)
			beforeProof, _ := os.ReadFile(proof)
			row := staleBindingsFor([]radar.PaneRow{pane}, time.Now().Unix(), func(string, resume.Record) bool { return false })
			if row.status != stMiss || !strings.Contains(row.note, pane.PaneID) {
				t.Fatalf("managed association failure hidden: %+v", row)
			}
			want := "proof-expired"
			if mode == "missing-proof" {
				want = "frontend-record-unavailable"
			}
			if !strings.Contains(row.note, want) {
				t.Fatalf("missing concrete reason %s: %+v", want, row)
			}
			afterIndex, _ := os.ReadFile(index)
			afterProof, _ := os.ReadFile(proof)
			if !bytes.Equal(beforeIndex, afterIndex) || !bytes.Equal(beforeProof, afterProof) {
				t.Fatal("doctor changed association evidence")
			}
		})
	}
}

func TestDCA002DoctorDistinguishesMissingUnsupportedAndVerified(t *testing.T) {
	pane, index, _ := doctorAssociationFixture(t)
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	row := staleBindingsFor([]radar.PaneRow{pane}, time.Now().Unix(), func(string, resume.Record) bool { return false })
	if row.status != stRec || !strings.Contains(row.note, pane.PaneID) {
		t.Fatalf("unbound supported pane disappeared: %+v", row)
	}
	for _, control := range []radar.PaneRow{
		{Tier: "plain", PaneID: "%1", Loc: "plain:0.0", Command: "bash"},
		{Tier: "agent", Agent: "Gemini", PaneID: "%2", Loc: "unsupported:0.0"},
	} {
		if got := staleBindingsFor([]radar.PaneRow{control}, time.Now().Unix(), func(string, resume.Record) bool { return false }); got.status != stInfo {
			t.Fatalf("unsupported/plain treated as failed association: %+v", got)
		}
	}
	data, _ := json.Marshal(resume.Record{Agent: "codex", SessionID: "doctor-root", HomeID: transcript.CodexHomeID()})
	if err := os.WriteFile(index, data, 0600); err != nil {
		t.Fatal(err)
	}
	if got := staleBindingsFor([]radar.PaneRow{pane}, time.Now().Unix(), func(string, resume.Record) bool { return true }); got.status != stOK {
		t.Fatalf("verified control lost: %+v", got)
	}
}

func TestDCA003DoctorJSONKeepsFailureAndRecoveryBoundary(t *testing.T) {
	pane, _, _ := doctorAssociationFixture(t)
	row := staleBindingsFor([]radar.PaneRow{pane}, time.Now().Unix(), func(string, resume.Record) bool { return false })
	var output bytes.Buffer
	if code := writeDoctorJSON(&output, []dsection{{title: "Agents", rows: []dcheck{row}}}, time.Now()); code != 1 {
		t.Fatalf("failed association looked successful: exit=%d %s", code, output.String())
	}
	var report doctorReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	got := report.Sections[0].Rows[0]
	if got.Status != "blocking" || !strings.Contains(got.Note, "proof-expired") || got.Action != "" || got.Command != "" {
		t.Fatalf("doctor invented repair or dropped cause: %+v", got)
	}
}

func TestDPC003DoctorDoesNotPromiseAutomaticRecovery(t *testing.T) {
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
