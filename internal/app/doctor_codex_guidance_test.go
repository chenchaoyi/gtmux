package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestDBR003DoctorCodexGuidanceCoversUnavailableStates(t *testing.T) {
	for _, state := range []string{"expired", "missing", "unverified", "mixed", "other-agent"} {
		t.Run(state, func(t *testing.T) {
			pane, index, proof := doctorAssociationFixture(t)
			missing := radar.PaneRow{Tier: "agent", Agent: "Codex", PaneID: "%10", Loc: "synthetic:0.1"}
			other := radar.PaneRow{Tier: "agent", Agent: "Claude Code", PaneID: "%11", Loc: "synthetic:0.2"}
			panes, wantStatus := []radar.PaneRow{pane}, stMiss
			wantFocus, wantReason := []string{"%9"}, "proof-expired"
			switch state {
			case "missing":
				panes, wantStatus = []radar.PaneRow{missing}, stRec
				wantFocus, wantReason = []string{"%10"}, "resume-record-unavailable"
			case "unverified":
				writeDoctorCurrentProof(t, proof)
				wantStatus, wantReason = stRec, "current-client-unverified"
			case "mixed":
				panes = append(panes, missing, other)
				wantFocus = append(wantFocus, "%10")
			case "other-agent":
				panes, wantStatus = []radar.PaneRow{other}, stRec
				wantFocus, wantReason = nil, "resume-record-unavailable"
			}
			beforeIndex, beforeProof := doctorReadAssociationFixture(t, index), doctorReadAssociationFixture(t, proof)
			row := staleBindingsFor(panes, time.Now().Unix(), func(string, resume.Record) bool { return false })
			if row.status != wantStatus || !strings.Contains(row.note, wantReason) {
				t.Fatalf("diagnosis changed: %+v", row)
			}
			for _, id := range wantFocus {
				if !strings.Contains(row.note, "gtmux focus "+id) {
					t.Fatalf("missing terminal guidance for %s: %s", id, row.note)
				}
			}
			if strings.Contains(row.note, "gtmux focus %11") ||
				strings.Contains(row.note, "gtmux codex -- resume") != (len(wantFocus) > 0) {
				t.Fatalf("wrong recovery guidance: %s", row.note)
			}
			if len(wantFocus) > 0 && !strings.Contains(row.note, "CODEX_HOME") {
				t.Fatalf("the original launch context was omitted: %s", row.note)
			}
			if !bytes.Equal(beforeIndex, doctorReadAssociationFixture(t, index)) ||
				!bytes.Equal(beforeProof, doctorReadAssociationFixture(t, proof)) {
				t.Fatal("the diagnostic wrote association evidence")
			}
		})
	}
}

func doctorReadAssociationFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writeDoctorCurrentProof(t *testing.T, path string) {
	t.Helper()
	bridge, ok := launchhost.ReadAgent(os.Getpid())
	if !ok {
		t.Fatal("could not read the test process")
	}
	var receipt resume.CodexFrontend
	if err := json.Unmarshal(doctorReadAssociationFixture(t, path), &receipt); err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%%9\\t100\\tsynthetic:0.0\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	t.Cleanup(func() { tmux.Bin = old })
	tmux.Bin = stub
	receipt.BridgePID, receipt.BridgeStart, receipt.UpdatedAt = bridge.PID, bridge.Start, time.Now().UnixMilli()
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
}
