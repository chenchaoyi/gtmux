package hqworkspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func fixture(t *testing.T) *Store {
	t.Helper()
	s := New(t.TempDir())
	if _, err := s.Create("matter-a", "Investigate storage", 100); err != nil {
		t.Fatal(err)
	}
	return s
}

func participant(id, session string, role string, from, until int64) Association {
	return Association{ID: id, Conversation: Conversation{Agent: "codex", HomeID: "home", SessionID: session, Pane: "%14", Name: "worker"}, Role: role, From: from, Until: until}
}

func TestReadDoesNotCreateOrHideCorruption(t *testing.T) {
	home := t.TempDir()
	s := New(home)
	d, err := s.Read()
	if err != nil || len(d.Matters) != 0 {
		t.Fatalf("empty read: %v %v", d, err)
	}
	if _, err := os.Stat(filepath.Join(home, "workspace")); !os.IsNotExist(err) {
		t.Fatal("a read created workspace data")
	}
	if _, err := s.Create("matter-a", "Storage", 100); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "workspace", "records.json"), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(); err == nil {
		t.Fatal("corruption became an empty success")
	}
	if _, err := s.Create("matter-b", "Other", 101); err == nil {
		t.Fatal("mutation overwrote corruption")
	}
}

func TestExplicitMembershipMoveAndHistoricalInterval(t *testing.T) {
	s := fixture(t)
	if _, err := s.Create("matter-b", "Other goal", 100); err != nil {
		t.Fatal(err)
	}
	a := participant("assoc-a", "session-a", "execution", 100, 0)
	if err := s.Associate("matter-a", a, 0, 100); err != nil {
		t.Fatal(err)
	}
	b := participant("assoc-b", "session-b", "reference", 50, 99) // same name/pane, different conversation
	if err := s.Associate("matter-a", b, 0, 100); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Read()
	if len(d.Matters["matter-a"].Participants) != 2 {
		t.Fatal("conversations were deduplicated by name or pane")
	}
	if a.Conversation.Same(b.Conversation) {
		t.Fatal("locator became identity")
	}
	if err := s.Interval("matter-a", "assoc-b", 50, 0, 0, 110); err == nil {
		t.Fatal("unbounded historical reference accepted")
	}
	if err := s.Interval("matter-a", "assoc-b", 50, 98, 0, 110); err != nil {
		t.Fatal(err)
	}
	if err := s.Move("matter-a", "matter-b", "assoc-a", 0, 120); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	if d.Matters["matter-a"].Participants[0].RemovedAt != 120 || len(d.Matters["matter-b"].Participants) != 1 {
		t.Fatal("move lost provenance")
	}
	if err := s.Move("matter-b", "", "assoc-a", 0, 130); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	if d.Matters["matter-b"].Participants[0].RemovedAt != 130 {
		t.Fatal("unlink deleted record")
	}
}

func TestStaleChangesAndMergeSplitKeepMessages(t *testing.T) {
	s := fixture(t)
	d, _ := s.Read()
	old := d.Matters["matter-a"].Revision
	if err := s.Update("matter-a", "", "needs-input", "Need decision", old, 101); err != nil {
		t.Fatal(err)
	}
	if err := s.Update("matter-a", "Wrong", "", "", old, 102); err != ErrConflict {
		t.Fatalf("stale update: %v", err)
	}
	if _, err := s.Submit("msg-old", "matter-a", "Original requirements", 103); err != nil {
		t.Fatal(err)
	}
	if err := s.Associate("matter-a", participant("assoc-a", "session-a", "execution", 100, 0), 0, 104); err != nil {
		t.Fatal(err)
	}
	if err := s.Split("matter-a", "matter-child", "Narrow goal", []string{"assoc-a"}, 0, 105); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	if d.Matters["matter-child"].Parent != "matter-a" || d.Matters["matter-a"].Participants[0].RemovedAt != 105 {
		t.Fatal("split lost source")
	}
	if err := s.Merge("matter-child", "matter-a", 0, 106); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	rows, _ := MessagePage(d, "matter-a", 0, 100)
	if len(rows) != 1 || rows[0].Body != "Original requirements" || d.Matters["matter-child"].MergedInto != "matter-a" {
		t.Fatal("merge lost records")
	}
	if err := s.Merge("matter-a", "matter-child", 0, 107); err == nil {
		t.Fatal("merge cycle permitted")
	}
}

func TestSubmissionIdempotencyAndScope(t *testing.T) {
	s := fixture(t)
	if _, err := s.Submit("msg-one", "matter-a", "Please investigate", 101); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Read()
	revision := d.Revision
	if _, err := s.Submit("msg-one", "matter-a", "Please investigate", 102); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	if d.Revision != revision {
		t.Fatal("retry rewrote message")
	}
	for _, input := range []struct{ scope, body string }{{"", "Please investigate"}, {"matter-a", "Different instruction"}} {
		if _, err := s.Submit("msg-one", input.scope, input.body, 103); err != ErrConflict {
			t.Fatalf("ID collision: %v", err)
		}
	}
	if _, err := s.Publish("report-one", "matter-a", "msg-one", "result", "Report available", "", 104); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	if d.Matters["matter-a"].State != "open" {
		t.Fatal("result incorrectly completed matter")
	}
	if _, err := s.Publish("report-bad", "", "msg-one", "report", "Wrong scope", "", 105); err == nil {
		t.Fatal("cross-scope reply accepted")
	}
}

func TestConcurrentDuplicateMessagesAndPrivateFiles(t *testing.T) {
	s := fixture(t)
	var wg sync.WaitGroup
	var failures atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Submit("same-message", "matter-a", "Only once", 101); err != nil {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()
	d, err := s.Read()
	if err != nil || failures.Load() != 0 || len(d.Messages) != 1 {
		t.Fatalf("convergence: %v %d %d", err, failures.Load(), len(d.Messages))
	}
	st, err := os.Stat(filepath.Join(s.dir, "records.json"))
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("private records permissions")
	}
}

func TestDeliveryHoldSuccessAndNoExecutionInference(t *testing.T) {
	s := fixture(t)
	_, _ = s.Submit("msg-one", "matter-a", "Do not delete anything", 101)
	sent := 0
	target := Conversation{Agent: "codex", HomeID: "hq", SessionID: "sid-hq", Pane: "%22"}
	io := DeliveryIO{Resolve: func() (Conversation, string) { return target, "hq-busy" }, Send: func(_ Conversation, payload string) Delivery {
		sent++
		if !strings.Contains(payload, "msg-one") || !strings.Contains(payload, "matter-a") {
			t.Error("explicit context absent")
		}
		return Delivery{State: "landed", Evidence: "receipt"}
	}}
	if done, err := s.Pump(io, 102); err != nil || done || sent != 0 {
		t.Fatalf("busy typed: %v %v", done, err)
	}
	d, _ := s.Read()
	if d.Messages["msg-one"].Delivery != "pending" || d.Messages["msg-one"].Reason != "hq-busy" {
		t.Fatal("held receipt")
	}
	io.Resolve = func() (Conversation, string) { return target, "" }
	if done, err := s.Pump(io, 103); err != nil || !done {
		t.Fatalf("delivery: %v %v", done, err)
	}
	_, _ = s.Pump(io, 104)
	d, _ = s.Read()
	if sent != 1 || d.Messages["msg-one"].Delivery != "delivered" || d.Matters["matter-a"].State != "open" {
		t.Fatal("delivery duplicated or closed matter")
	}
	if err := s.Cancel("msg-one", 105); err == nil {
		t.Fatal("cancel pretended to stop submitted work")
	}
}

func TestUncertainDeliveryNeverAutomaticallyReplays(t *testing.T) {
	s := fixture(t)
	_, _ = s.Submit("msg-one", "matter-a", "Investigate", 101)
	sent := 0
	target := Conversation{Agent: "codex", SessionID: "old"}
	io := DeliveryIO{Resolve: func() (Conversation, string) { return target, "" }, Send: func(Conversation, string) Delivery {
		sent++
		return Delivery{State: "failed", Evidence: "input may be submitted"}
	}}
	_, err := s.Pump(io, 102)
	if err != nil {
		t.Fatal(err)
	}
	target.SessionID = "new"
	_, _ = s.Pump(io, 500)
	d, _ := s.Read()
	if sent != 1 || d.Messages["msg-one"].Delivery != "uncertain" {
		t.Fatal("uncertain instruction replayed across rotation")
	}
	if err := s.Retry("msg-one", false, 501); err == nil {
		t.Fatal("ordinary retry replayed uncertain instruction")
	}
	if err := s.Retry("msg-one", true, 502); err != nil {
		t.Fatal(err)
	}
	io.Send = func(Conversation, string) Delivery { sent++; return Delivery{State: "queued"} }
	_, _ = s.Pump(io, 503)
	d, _ = s.Read()
	if sent != 2 || d.Messages["msg-one"].Delivery != "accepted-queue" {
		t.Fatal("accepted queue misreported")
	}
}

func TestPostPasteRefusalAndVanishedSenderAreUncertain(t *testing.T) {
	s := fixture(t)
	_, _ = s.Submit("msg-one", "matter-a", "Instruction", 101)
	target := Conversation{Agent: "codex", SessionID: "hq"}
	io := DeliveryIO{Resolve: func() (Conversation, string) { return target, "" }, Send: func(Conversation, string) Delivery {
		return Delivery{State: "refused-waiting", SafeHold: false, Evidence: "stopped before Enter"}
	}}
	_, _ = s.Pump(io, 102)
	d, _ := s.Read()
	if d.Messages["msg-one"].Delivery != "uncertain" {
		t.Fatal("pasted draft automatically retried")
	}
	if err := s.transaction(func(d *Database) error {
		m := d.Messages["msg-one"]
		m.Delivery = "submitting"
		m.AttemptAt = 103
		d.Messages[m.ID] = m
		changed(d, 103, "test-attempt", m.MatterID, m.ID, "")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sent := 0
	io.Send = func(Conversation, string) Delivery { sent++; return Delivery{State: "landed"} }
	_, _ = s.Pump(io, 250)
	d, _ = s.Read()
	if sent != 0 || d.Messages["msg-one"].Reason != "sender-disappeared" {
		t.Fatal("vanished sender replayed")
	}
}

func TestFailedPersistencePreventsTypingAndPaginationIsLossless(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "workspace"), []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	s := New(home)
	if _, err := s.Submit("msg-one", "", "Text", 100); err == nil {
		t.Fatal("write failure hidden")
	}
	typed := false
	_, err := s.Pump(DeliveryIO{Resolve: func() (Conversation, string) { return Conversation{SessionID: "hq"}, "" }, Send: func(Conversation, string) Delivery { typed = true; return Delivery{State: "landed"} }}, 101)
	if err == nil || typed {
		t.Fatal("typing despite persistence failure")
	}
	s = fixture(t)
	for _, id := range []string{"msg-aaa", "msg-bbb", "msg-ccc"} {
		if _, err := s.Submit(id, "matter-a", id, 100); err != nil {
			t.Fatal(err)
		}
	}
	d, _ := s.Read()
	recent, more := MessagePage(d, "matter-a", 0, 2)
	if !more || len(recent) != 2 {
		t.Fatal("recent page")
	}
	older, more := MessagePage(d, "matter-a", recent[0].Seq, 2)
	if more || len(older) != 1 || older[0].ID != "msg-aaa" {
		t.Fatal("same-second messages skipped")
	}
}

func TestStartCommitsInstructionAtomicallyAndConverges(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.Start("matter-new", "Storage", "request-new", "", 100); err == nil {
		t.Fatal("empty instruction accepted")
	}
	d, _ := s.Read()
	if len(d.Matters) != 0 {
		t.Fatal("invalid instruction left orphan goal")
	}
	first, err := s.Start("matter-new", "Storage", "request-new", "check storage", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start("matter-new", "Storage", "request-new", "check storage", 101); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	if len(d.Messages) != 1 || d.Matters[first.ID].CreatedAt != 100 {
		t.Fatal("retry duplicated goal/instruction")
	}
	if _, err := s.Start("matter-new", "Storage", "request-new", "different", 102); err != ErrConflict {
		t.Fatal("same ID changed intent", err)
	}
}

func TestConcurrentPumpsKeepOneSenderEvenAfterTimeout(t *testing.T) {
	s := fixture(t)
	_, _ = s.Submit("request-one", "matter-a", "one", 100)
	_, _ = s.Submit("request-two", "matter-a", "two", 101)
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	io := DeliveryIO{Resolve: func() (Conversation, string) { return Conversation{Agent: "codex", SessionID: "hq"}, "" }, Send: func(Conversation, string) Delivery { close(started); <-release; return Delivery{State: "landed"} }}
	go func() { _, err := s.Pump(io, 102); done <- err }()
	<-started
	second := io
	second.Send = func(Conversation, string) Delivery {
		t.Error("concurrent sender typed")
		return Delivery{State: "landed"}
	}
	if attempted, err := s.Pump(second, 500); err != nil || attempted {
		t.Fatal("live sender displaced", attempted, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	d, _ := s.Read()
	if d.Messages["request-one"].Delivery != "delivered" || d.Messages["request-two"].Delivery != "pending" {
		t.Fatal("bad receipts", d.Messages)
	}
}

func TestMergedPendingDeliveryAndReportsUseCanonicalMatter(t *testing.T) {
	s := fixture(t)
	_, _ = s.Create("matter-b", "Canonical", 100)
	_, _ = s.Submit("request-old", "matter-a", "check storage", 101)
	if err := s.Merge("matter-a", "matter-b", 0, 102); err != nil {
		t.Fatal(err)
	}
	io := DeliveryIO{Resolve: func() (Conversation, string) { return Conversation{Agent: "codex", SessionID: "hq"}, "" }, Send: func(_ Conversation, wire string) Delivery {
		if !strings.Contains(wire, "matter matter-b (Canonical)") {
			t.Error("used merged scope", wire)
		}
		return Delivery{State: "landed"}
	}}
	if _, err := s.Pump(io, 103); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish("report-new", "matter-a", "request-old", "result", "found it", "", 104); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Read()
	if d.Matters["matter-b"].State != "open" || d.Messages["report-new"].MatterID != "matter-b" {
		t.Fatal("report lost canonical scope")
	}
}

func TestClosedMatterHoldsInstructionButDoesNotBlockOtherGoals(t *testing.T) {
	s := fixture(t)
	_, _ = s.Create("matter-b", "Other", 100)
	_, _ = s.Submit("request-one", "matter-a", "one", 101)
	_, _ = s.Submit("request-two", "matter-b", "two", 102)
	_ = s.Update("matter-a", "", "closed", "", 0, 103)
	io := DeliveryIO{Resolve: func() (Conversation, string) { return Conversation{Agent: "codex", SessionID: "hq"}, "" }, Send: func(_ Conversation, wire string) Delivery {
		if strings.Contains(wire, "request-one") {
			t.Error("closed goal executed")
		}
		return Delivery{State: "landed"}
	}}
	_, err := s.Pump(io, 104)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := s.Read()
	if d.Messages["request-one"].Delivery != "pending" || d.Messages["request-one"].Reason != "matter-closed" || d.Messages["request-two"].Delivery != "delivered" {
		t.Fatal("bad close/other goal handling", d.Messages)
	}
}

func TestRestoredSnapshotNeverReplaysPreviousPendingAuthorization(t *testing.T) {
	s := fixture(t)
	_, _ = s.Submit("pending-id", "matter-a", "one", 101)
	if err := s.MarkRestored(200); err != nil {
		t.Fatal(err)
	}
	io := DeliveryIO{Resolve: func() (Conversation, string) { return Conversation{Agent: "codex", SessionID: "new-hq"}, "" }, Send: func(Conversation, string) Delivery {
		t.Error("restored command replayed")
		return Delivery{State: "landed"}
	}}
	if tried, err := s.Pump(io, 201); err != nil || tried {
		t.Fatal("restored attempt", tried, err)
	}
	d, _ := s.Read()
	if d.Messages["pending-id"].Delivery != "uncertain" || d.Messages["pending-id"].Reason != "restored-backup" {
		t.Fatal(d.Messages)
	}
}

func TestMalformedAndOversizedSnapshotCannotBecomeEmptySuccess(t *testing.T) {
	s := New(t.TempDir())
	_, _ = s.Create("matter-a", "Goal", 100)
	path := filepath.Join(s.dir, "records.json")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(32<<20 + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err := s.Read(); err == nil {
		t.Fatal("oversized snapshot accepted")
	}
	if _, err := s.Create("matter-b", "New", 101); err == nil {
		t.Fatal("oversized record overwritten")
	}
}

func TestStableInstructionReceiptSurvivesRenameAndChangedReportIntentIsRejected(t *testing.T) {
	s := New(t.TempDir())
	_, _ = s.Start("matter-new", "Original", "request-new", "check", 100)
	_ = s.Update("matter-new", "Renamed", "", "", 0, 101)
	if _, err := s.Start("matter-new", "Original", "request-new", "check", 102); err != nil {
		t.Fatal("lost receipt cannot converge after rename", err)
	}
	if _, err := s.Start("matter-new", "Changed intent", "request-new", "check", 102); err != ErrConflict {
		t.Fatal("title intent changed", err)
	}
	_, err := s.Publish("report-one", "matter-new", "request-new", "result", "verified", "", 103)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish("report-one", "matter-new", "request-new", "result", "verified", "completed", 104); err != ErrConflict {
		t.Fatal("report reused ID with different state intent", err)
	}
}
