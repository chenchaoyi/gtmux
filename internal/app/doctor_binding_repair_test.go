package app

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestDBR002DoctorIdentifiesTheTerminalWithoutTouchingIt(t *testing.T) {
	pane, index, proof := doctorAssociationFixture(t)
	beforeIndex, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	beforeProof, err := os.ReadFile(proof)
	if err != nil {
		t.Fatal(err)
	}
	panes := []radar.PaneRow{pane}
	row := staleBindingsFor(panes, time.Now().Unix(), func(string, resume.Record) bool { return false })
	if row.status != stMiss || !strings.Contains(row.note, "gtmux focus "+pane.PaneID) ||
		!strings.Contains(row.note, "gtmux codex -- resume") {
		t.Fatalf("missing actionable recovery steps: %+v", row)
	}
	s := &fixState{yes: true}
	if repaired := s.repairConversationBindings(panes); repaired != 0 || s.rc != 0 {
		t.Fatalf("the fix reported an unverified recovery: %d, %d", repaired, s.rc)
	}
	afterIndex, _ := os.ReadFile(index)
	afterProof, _ := os.ReadFile(proof)
	if !bytes.Equal(beforeIndex, afterIndex) || !bytes.Equal(beforeProof, afterProof) {
		t.Fatal("unattended fix changed the expired evidence")
	}
}
