package hqnudge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
)

// The transport callback uses the real persistent native claim/receipt protocol.
// A panic models process loss after the outcome is durable, before hqnudge cleanup.
func TestHRW009NativeOutcomeCrashCannotRebatchOldInput(t *testing.T) {
	for _, outcome := range []string{"accepted", "uncertain"} {
		for _, extra := range []bool{false, true} {
			t.Run(outcome+"/new-entry-"+map[bool]string{false: "no", true: "yes"}[extra], func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				f := &fake{}
				x := f.io()
				a := agentclient.Attachment{Version: 1, Pane: "%isolated", PID: 101, Instance: strings.Repeat("a", 32), HomeID: "synthetic-home", SessionID: "root-a", InputReady: true}
				var mutations []string
				crash := true
				x.nativeSubmit = func(pane, payload, id string) (dispatch.Result, bool) {
					r, err := agentclient.Prepare(a, payload, "hq-wake-"+id, false, time.Now())
					if err != nil {
						return dispatch.Result{State: dispatch.StateFailed}, true
					}
					if receipt, ok := agentclient.ReadReceipt(r); ok {
						if receipt.State == "accepted" {
							return dispatch.Result{State: dispatch.StateQueued}, true
						}
						return dispatch.Result{State: dispatch.StateFailed}, true
					}
					if err := agentclient.ClaimNative(r); err != nil {
						t.Fatal(err)
					}
					mutations = append(mutations, payload)
					result := outcome
					if !crash {
						result = "accepted"
					}
					if err := agentclient.FinishNative(r, result, ""); err != nil {
						t.Fatal(err)
					}
					if crash {
						panic("stop after durable native outcome")
					}
					return dispatch.Result{State: dispatch.StateQueued}, true
				}
				enqueue(x, "original-A")
				func() {
					defer func() {
						if recover() == nil {
							t.Fatal("interruption not reached")
						}
					}()
					drainInto(x, "%isolated")
				}()
				if len(mutations) != 1 {
					t.Fatal("first real native mutation missing", mutations)
				}
				for _, e := range readQueue() {
					if strings.HasSuffix(e.Name(), sendingSuffix) {
						p := filepath.Join(queueDir(), e.Name())
						old := time.Now().Add(-2 * time.Minute)
						if err := os.Chtimes(p, old, old); err != nil {
							t.Fatal(err)
						}
					}
				}
				crash = false
				if extra {
					enqueue(x, "newcomer-B")
				}
				drainInto(x, "%isolated")
				drainInto(x, "%isolated")
				oldCount, newCount := 0, 0
				for _, s := range mutations {
					if strings.Contains(s, "original-A") {
						oldCount++
					}
					if strings.Contains(s, "newcomer-B") {
						newCount++
					}
				}
				if oldCount != 1 || extra && newCount != 1 || !extra && newCount != 0 {
					t.Fatalf("old input rebatched or fresh input blocked: A=%d B=%d mutations=%q", oldCount, newCount, mutations)
				}
				// Same text from a genuinely new event is not body-hash deduplicated.
				enqueue(x, "original-A")
				drainInto(x, "%isolated")
				count := 0
				for _, s := range mutations {
					if strings.Contains(s, "original-A") {
						count++
					}
				}
				if count != 2 {
					t.Fatal("legitimate new identical text suppressed", mutations)
				}
				if len(f.sent) != 0 {
					t.Fatal("native recovery fell back to paste", f.sent)
				}
			})
		}
	}
}

func TestHRW010BusyReturnPreservesDelayedNewestKeyedProducer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const base = int64(1700000000)
	hqwake.StampDone("%worker", base)
	immediate, due := hqwake.DoneDue("%worker", base+29, 30)
	if immediate {
		t.Fatal("fixture did not use merge deadline")
	}
	f := &fake{}
	old := prod
	prod = f.io()
	defer func() { prod = old }()
	clock := (base + 29) * int64(time.Second)
	prod.nowNano = func() int64 { return clock }
	dueNano := due * int64(time.Second)
	updates := 0
	prod.nativeSubmit = func(string, string, string) (dispatch.Result, bool) {
		if updates == 0 {
			updates++
			DeliverKeyedAt("%isolated", "done-%worker", "newest-B", dueNano)
		}
		return dispatch.Result{State: dispatch.StateRefusedWaiting}, true
	}
	DeliverKeyedAt("%isolated", "done-%worker", "original-A", dueNano)
	if updates != 0 {
		t.Fatal("future entry claimed early")
	}
	immediate2, due2 := hqwake.DoneDue("%worker", base+29, 30)
	if immediate2 || due2 != due {
		t.Fatal("second real producer changed due")
	}
	clock = dueNano + 1
	Drain("%isolated")
	var payloads []string
	for _, e := range readQueue() {
		if strings.HasSuffix(e.Name(), ".txt") {
			b, err := os.ReadFile(filepath.Join(queueDir(), e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			payloads = append(payloads, string(b))
			if dueOf(e.Name()) != dueNano {
				t.Fatal("original deadline lost")
			}
		}
	}
	if updates != 1 || len(payloads) != 1 || payloads[0] != "newest-B" {
		t.Fatalf("busy return replaced newer producer: updates=%d payloads=%q", updates, payloads)
	}
	if len(f.sent) != 0 {
		t.Fatal("busy native path typed input")
	}
}

func TestHRW009JournalFailuresAndForeignReplacementHoldWithoutMutation(t *testing.T) {
	for _, mode := range []string{"write-failure", "malformed", "replaced-claim", "replaced-identical", "prepared-before-call", "accepted-before-cleanup", "untracked-old-native"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			f := &fake{}
			x := f.io()
			calls := 0
			x.nativeSubmit = func(string, string, string) (dispatch.Result, bool) {
				calls++
				return dispatch.Result{State: dispatch.StateQueued}, true
			}
			enqueue(x, "retained-A")
			if mode == "write-failure" {
				if err := os.WriteFile(nativeIntentDir(), []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if mode == "malformed" {
				if err := os.MkdirAll(nativeIntentDir(), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(nativeIntentDir(), "invalid.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				claimed, msgs, _ := claimBatch(queuedNames(func(string) bool { return true }))
				if len(claimed) != 1 || len(msgs) != 1 {
					t.Fatal("missing original claim")
				}
				if mode == "untracked-old-native" {
					old := time.Now().Add(-2 * time.Minute)
					if err := os.Chtimes(claimed[0], old, old); err != nil {
						t.Fatal(err)
					}
				} else {
					r, err := prepareNativeIntent("%isolated", "#123abc", "exact-original-payload · #123abc", claimed)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "replaced-claim" {
						if err := os.WriteFile(claimed[0], []byte("foreign replacement"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					if mode == "replaced-identical" {
						if err := os.Remove(claimed[0]); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(claimed[0], []byte("retained-A"), 0600); err != nil {
							t.Fatal(err)
						}
						r.Outcome = "accepted"
						if err := writeNativeIntent(r); err != nil {
							t.Fatal(err)
						}
					}
					if mode == "accepted-before-cleanup" {
						r.Outcome = "accepted"
						if err := writeNativeIntent(r); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			drainInto(x, "%isolated")
			drainInto(x, "%isolated")
			if calls != 0 || len(f.sent) != 0 {
				t.Fatal("unsafe recovery authorized mutation", calls, f.sent)
			}
			if mode == "replaced-identical" {
				count := 0
				for _, e := range readQueue() {
					if strings.HasSuffix(e.Name(), sendingSuffix) {
						count++
					}
				}
				if count != 1 {
					t.Fatal("accepted recovery deleted replacement inode with identical bytes")
				}
			}
			if mode == "prepared-before-call" {
				files, err := os.ReadDir(nativeIntentDir())
				if err != nil || len(files) != 1 {
					t.Fatal("uncertain intent lost", err)
				}
				b, err := os.ReadFile(filepath.Join(nativeIntentDir(), files[0].Name()))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(b), "exact-original-payload") || !strings.Contains(string(b), "#123abc") || !strings.Contains(string(b), "uncertain") {
					t.Fatal("original recovery identity lost", string(b))
				}
			}
			if mode == "replaced-claim" {
				for _, e := range readQueue() {
					b, _ := os.ReadFile(filepath.Join(queueDir(), e.Name()))
					if strings.HasSuffix(e.Name(), sendingSuffix) && string(b) != "foreign replacement" {
						t.Fatal("replacement changed")
					}
				}
			}
			if mode == "accepted-before-cleanup" {
				files, _ := os.ReadDir(nativeIntentDir())
				if len(files) != 0 || len(readQueue()) != 0 {
					t.Fatal("accepted recovery did not finish idempotently")
				}
			}
		})
	}
}

func TestHRW010ClaimReturnPreservesForeignConflictAndInterruptedPublication(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	x := f.io()
	enqueue(x, "old-unkeyed")
	claimed, _, _ := claimBatch(queuedNames(func(string) bool { return true }))
	if len(claimed) != 1 {
		t.Fatal("fixture claim missing")
	}
	c := claimed[0]
	dst := claimBase(c)
	if err := os.WriteFile(dst, []byte("foreign-new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := returnClaim(c); err == nil {
		t.Fatal("foreign non-key conflict was ignored")
	}
	a, _ := os.ReadFile(c)
	b, _ := os.ReadFile(dst)
	if string(a) != "old-unkeyed" || string(b) != "foreign-new" {
		t.Fatal("conflict overwritten", string(a), string(b))
	}
	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(c, dst); err != nil {
		t.Fatal(err)
	} // stop after no-clobber publication
	if err := returnClaim(c); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(dst)
	if string(b) != "old-unkeyed" {
		t.Fatal("original entry lost")
	}
	if _, err := os.Stat(c); !os.IsNotExist(err) {
		t.Fatal("duplicate claim survived")
	}
}

func TestHRW010RecoveryAfterReturnPublicationKeepsConcurrentKeyUpdate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	x := f.io()
	enqueueKeyed(x, "key", "old-A", 1)
	claimed, _, _ := claimBatch(queuedNames(func(string) bool { return true }))
	if len(claimed) != 1 {
		t.Fatal("missing claim")
	}
	r, err := prepareNativeIntent("%isolated", "#123abc", "old-A · #123abc", claimed)
	if err != nil {
		t.Fatal(err)
	}
	r.Outcome = "refused"
	if err := writeNativeIntent(r); err != nil {
		t.Fatal(err)
	}
	dst := claimBase(claimed[0])
	if err := os.Link(claimed[0], dst); err != nil {
		t.Fatal(err)
	}
	// Stop after publication; a producer may legitimately update the queue inode.
	enqueueKeyed(x, "key", "newest-B", 1)
	if err := recoverNativeIntents(); err != nil {
		t.Fatal("published queue ownership was mistaken for a replaced claim", err)
	}
	b, err := os.ReadFile(dst)
	if err != nil || string(b) != "newest-B" {
		t.Fatal("newest update lost", err, string(b))
	}
	if _, err := os.Stat(claimed[0]); !os.IsNotExist(err) {
		t.Fatal("old claim survived settled refused return")
	}
	files, _ := os.ReadDir(nativeIntentDir())
	if len(files) != 0 {
		t.Fatal("refused recovery not closed")
	}
}
