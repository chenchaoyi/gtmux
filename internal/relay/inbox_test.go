package relay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func inboxFixture(t *testing.T, id, home, thread string) Request {
	t.Helper()
	_, _, e := Put(Request{ID: id, Kind: "ask", Agent: "codex", HomeID: home, ThreadID: thread, Session: "synthetic", Body: "question", For: "hq"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Claim(id); e != nil {
		t.Fatal(e)
	}
	r, e := Resolve(id, "Synthetic HQ reply")
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestRBL002InboxCoalescesConcurrentNoticesAndPreservesReceipts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := Source{Agent: "codex", HomeID: "home-a", ThreadID: "thread-a"}
	a := inboxFixture(t, "req-aaa", "home-a", "thread-a")
	b := inboxFixture(t, "req-bbb", "home-a", "thread-a")
	other := inboxFixture(t, "req-other", "home-a", "other-thread")
	if a.ReplyAt == 0 || a.ReplyChannel != "inbox" || a.Delivered {
		t.Fatal("reply receipt", a)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for _, r := range []Request{a, b} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if BeginNotice(id, "turn-a") == nil {
				wins.Add(1)
			}
		}(r.ID)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal("burst notice count", wins.Load())
	}
	for _, r := range []Request{a, b} {
		r, _ = Get(r.ID)
		if r.Notice != nil {
			if e := FinishNotice(r.ID, false, true); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := BeginNotice(other.ID, "turn-a"); e != nil {
		t.Fatal("other thread was coalesced", e)
	}
	rows := Inbox(source, false, 20)
	if len(rows) != 2 {
		t.Fatal("wrong inbox scope", len(rows))
	}
	if _, e := Fetch(a.ID, Source{Agent: "codex", HomeID: "wrong", ThreadID: "thread-a"}); !errors.Is(e, ErrState) {
		t.Fatal("cross-home fetch", e)
	}
	fetched, e := Fetch(a.ID, source)
	if e != nil || fetched.FetchedAt == 0 || fetched.Delivered {
		t.Fatal("fetch is not delivery", fetched, e)
	}
	second, e := Fetch(a.ID, source)
	if e != nil || second.FetchedAt != fetched.FetchedAt {
		t.Fatal("fetch changed receipt", e)
	}
	if len(Inbox(source, false, 20)) != 1 || len(Inbox(source, true, 20)) != 2 {
		t.Fatal("unread/all filter")
	}
	old, _ := Get(a.ID)
	again, e := Resolve(a.ID, old.Reply)
	if e != nil || again.ReplyAt != a.ReplyAt {
		t.Fatal("identical resolve renewed age", e)
	}
	// Source pulls/notification receipts do not remove or alter the original body.
	for _, r := range List() {
		if r.Body != "question" || r.Reply != "Synthetic HQ reply" {
			t.Fatal("content rewritten")
		}
	}
}
func TestRBL002FreshnessLegacyAndBoundedSourceWait(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := inboxFixture(t, "req-fresh", "home-a", "thread-a")
	now := time.Now().Unix()
	for _, at := range []int64{0, now - 901, now + 1} {
		copy := r
		copy.ReplyAt = at
		if NoticeEligible(copy, now) {
			t.Fatal("stale/future reply not pull-only", at)
		}
	}
	if !NoticeEligible(r, now) {
		t.Fatal("fresh reply denied")
	}
	if e := transaction(func() error {
		v, _ := read(r.ID)
		v.ReplyAt = 0
		v.ReplyChannel = ""
		v.Delivered = true
		return write(v)
	}); e != nil {
		t.Fatal(e)
	}
	legacy, _ := Get(r.ID)
	if NoticeEligible(legacy, now) {
		t.Fatal("legacy replay")
	}
	s := Source{Agent: "codex", HomeID: "home-a", ThreadID: "thread-a"}
	if len(Inbox(s, false, 20)) != 0 || len(Inbox(s, true, 20)) != 1 {
		t.Fatal("legacy visibility changed")
	}
	p, _, e := Put(Request{ID: "req-pending", Kind: "ask", Agent: "codex", HomeID: "home-a", ThreadID: "thread-a", Session: "synthetic", Body: "pending", For: "hq"})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, e := Wait(ctx, p.ID, s)
	if !errors.Is(e, context.Canceled) || out.State != "pending" {
		t.Fatal("wait cancellation", out, e)
	}
	if _, e = Wait(ctx, p.ID, Source{Agent: "codex", HomeID: "other", ThreadID: "thread-a"}); !errors.Is(e, ErrState) {
		t.Fatal("foreign wait", e)
	}
	current, _ := Get(p.ID)
	if current.State != "pending" || current.FetchedAt != 0 {
		t.Fatal("timeout changed request")
	}
}

func TestRBL002InboxSortsReplyTimeAndRechecksStalenessAtClaim(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := inboxFixture(t, "req-older-request", "home-a", "thread-a")
	b := inboxFixture(t, "req-newer-request", "home-a", "thread-a")
	now := time.Now().Unix()
	if e := transaction(func() error {
		a.CreatedAt = now - 500
		a.ReplyAt = now
		b.CreatedAt = now - 1
		b.ReplyAt = now - 10
		if e := write(a); e != nil {
			return e
		}
		return write(b)
	}); e != nil {
		t.Fatal(e)
	}
	source := Source{Agent: "codex", HomeID: "home-a", ThreadID: "thread-a"}
	if rows := Inbox(source, false, 1); len(rows) != 1 || rows[0].ID != a.ID {
		t.Fatal("inbox sorted request age rather than reply age", rows)
	}
	if e := transaction(func() error { r, _ := read(a.ID); r.ReplyAt = now - 901; return write(r) }); e != nil {
		t.Fatal(e)
	}
	if e := BeginNotice(a.ID, "turn-a"); !errors.Is(e, ErrState) {
		t.Fatal("expired reply crossed final mutation guard", e)
	}
	if r, _ := Get(a.ID); r.Notice != nil || r.Reply != a.Reply {
		t.Fatal("expiry changed source data", r)
	}
	if _, e := Fetch(b.ID, source); e != nil {
		t.Fatal(e)
	}
	if e := BeginNotice(b.ID, "turn-a"); !errors.Is(e, ErrState) {
		t.Fatal("already fetched reply notified", e)
	}
}

func TestRBL002SourceWaitCancelsWhileLedgerIsLocked(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := inboxFixture(t, "req-contended", "home-a", "thread-a")
	lock, err := os.OpenFile(filepath.Join(dir(), ".lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	entered := make(chan struct{})
	var once sync.Once
	old := tryFlock
	tryFlock = func(fd, flags int) error { once.Do(func() { close(entered) }); return syscall.Flock(fd, flags) }
	defer func() { tryFlock = old }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, e := Wait(ctx, r.ID, Source{Agent: "codex", HomeID: "home-a", ThreadID: "thread-a"})
		done <- e
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("wait never reached the real lock")
	}
	cancel()
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatal("lost wait cancellation", e)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("source wait blocked on the ledger lock after cancellation")
	}
	saved, e := Get(r.ID)
	if e != nil || saved.FetchedAt != 0 || saved.Reply != r.Reply {
		t.Fatal("cancelled wait consumed reply", saved, e)
	}
}
