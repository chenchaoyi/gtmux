package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestCHH003_WorkspaceObservationIngestShortCircuitsCaughtUpCursor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	events.Append(events.Record{Event: "Stop", Pane: "%1", Summary: "done"})
	seq := events.CurrentSeq()
	if seq <= 0 {
		t.Fatalf("CurrentSeq() = %d, want > 0", seq)
	}

	s := hqworkspace.New(state.HQHome())
	now := time.Now().Unix()
	if err := s.ActivateObservations(seq, now); err != nil {
		t.Fatalf("ActivateObservations: %v", err)
	}

	// Make events.jsonl unreadable: caught-up cursor == CurrentSeq() must short-circuit
	// without calling events.ReadSince.
	if err := os.Chmod(events.Path(), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(events.Path(), 0o600) })

	if err := ingestWorkspaceObservations(s, now+1); err != nil {
		t.Fatalf("ingestWorkspaceObservations failed on caught-up cursor: %v", err)
	}
}

func TestCHH005_TranscriptConditionalETagShortCircuitAndSinglePaneGeometryQuery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// 1) transcriptRecordReadFull populates ETag metadata, then short-circuits on matching If-None-Match.
	projDir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(projDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sid := "sess-etag-1"
	logBody := strings.Join([]string{
		`{"type":"user","timestamp":"2026-11-28T08:00:00Z","message":{"role":"user","content":"Hello"}}`,
		`{"type":"user","isCompactSummary":true,"timestamp":"2026-11-28T08:00:02Z","message":{"role":"user","content":"This session is being continued from a previous conversation..."}}`,
		`{"type":"assistant","timestamp":"2026-11-28T08:00:05Z","message":{"role":"assistant","content":"World"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(projDir, sid+".jsonl"), []byte(logBody), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := resume.Record{Agent: "claude", SessionID: sid, Cwd: home}

	publicBody, pubMeta, err := transcriptRecordReadFull("%1", rec, false, 0, 20, "", false, false)
	if err != nil || pubMeta.Etag == "" || !pubMeta.ContextPrepared {
		t.Fatalf("initial public read err=%v meta=%+v", err, pubMeta)
	}
	if strings.Contains(string(publicBody), `"context"`) {
		t.Fatalf("public transcript leaked context segment: %s", string(publicBody))
	}

	// Conditional read with matching If-None-Match returns nil body with identical ETag.
	condBody, condMeta, err := transcriptRecordReadFull("%1", rec, false, 0, 20, pubMeta.Etag, false, false)
	if err != nil || condBody != nil || condMeta.Etag != pubMeta.Etag {
		t.Fatalf("conditional short-circuit = (%q, %+v, %v), want nil body + matching ETag", string(condBody), condMeta, err)
	}

	// Reader view (includeContext=true) gets a distinct ETag and preserves context segments.
	readerBody, readerMeta, err := transcriptRecordReadFull("%1", rec, false, 0, 20, pubMeta.Etag, true, false)
	if err != nil || readerMeta.Etag == pubMeta.Etag || !strings.Contains(string(readerBody), `"context"`) {
		t.Fatalf("includeContext=true read = (%s, %+v, %v)", string(readerBody), readerMeta, err)
	}

	// 2) paneGeometry fetches width + cursor in ONE tmux invocation.
	countFile := filepath.Join(t.TempDir(), "tmux-calls")
	fakeTmux := filepath.Join(t.TempDir(), "fake-tmux")
	script := "#!/bin/sh\necho x >> " + countFile + "\necho '132 7 22 24 1'\n"
	if err := os.WriteFile(fakeTmux, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	origBin := tmux.Bin
	tmux.Bin = fakeTmux
	t.Cleanup(func() { tmux.Bin = origBin })

	cols, cx, up, vis, ok := paneGeometry("%1")
	if !ok || cols != 132 || cx != 7 || up != 1 || !vis {
		t.Fatalf("paneGeometry = (%d, %d, %d, %v, %v), want (132, 7, 1, true, true)", cols, cx, up, vis, ok)
	}
	callsRaw, _ := os.ReadFile(countFile)
	if lines := strings.Count(string(callsRaw), "\n"); lines != 1 {
		t.Fatalf("paneGeometry invoked tmux %d times, want 1", lines)
	}
}

func TestCHH008_DoctorOversizeBoardAndWorkspaceErrorLogging(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	if err := state.MkdirAll(filepath.Dir(hq.BoardPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	oversize := strings.Repeat("- handoff entry\n", hq.BoardLineBudget+10)
	if err := state.WritePrivateFile(hq.BoardPath(), []byte(oversize)); err != nil {
		t.Fatal(err)
	}

	row := rowHQBoard(now.Unix())
	if row.status != stRec || !strings.Contains(row.note, "10") {
		t.Fatalf("rowHQBoard did not warn on >250-line board: %+v", row)
	}

	// Verify failed workspace action records "error" attribute in diagnostic log.
	diag.SetProcess("cli", "user")
	t.Cleanup(func() { diag.SetProcess("", "") })
	s := hqworkspace.New(state.HQHome())
	_, err := applyWorkspaceAction(s, workspaceAction{Op: "cancel", ID: "missing-msg"}, now.Unix())
	if err == nil {
		t.Fatal("expected error cancelling non-existent workspace message")
	}
	entries := storeEntries(now.Add(time.Minute), time.Hour, func(e diag.Entry) bool {
		return e.Event == "act.hq-workspace"
	})
	if len(entries) == 0 {
		t.Fatal("expected act.hq-workspace log entry")
	}
	last := entries[len(entries)-1]
	if fmtErr, _ := last.Attrs["error"].(string); fmtErr == "" {
		t.Fatalf("act.hq-workspace log entry missing error attr: %+v", last)
	}
}
