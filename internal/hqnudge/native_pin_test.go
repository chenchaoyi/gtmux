package hqnudge

import (
	"errors"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func fixtureNativePin(t *testing.T) (nativeIntent, string, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	x := f.io()
	enqueue(x, "original-A")
	c, _, _ := claimBatch(queuedNames(func(string) bool { return true }))
	if len(c) != 1 {
		t.Fatal("missing original claim")
	}
	r, err := prepareNativeIntent("%isolated", "#123abc", "original-A · #123abc", c)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(state.Dir(), "hq-nudge-native-claims")
	pins, err := os.ReadDir(dir)
	if err != nil || len(pins) != 1 {
		t.Fatalf("pre-submission intent has no single durable identity anchor: pins=%d err=%v", len(pins), err)
	}
	return r, c[0], filepath.Join(dir, pins[0].Name())
}

func TestHRW015NativeIntentKeepsOriginalIncarnationAlive(t *testing.T) {
	r, c, pin := fixtureNativePin(t)
	original, err := os.Stat(c)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := os.Stat(pin)
	if err != nil || !os.SameFile(original, anchor) {
		t.Fatal("anchor is not the original inode", err)
	}
	if err := os.Remove(c); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c, []byte("original-A"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(c, original.ModTime(), original.ModTime()); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Stat(c)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(anchor, replacement) {
		t.Fatal("replacement reused an inode that is still held alive")
	}
	r.Outcome = "accepted"
	if err := writeNativeIntent(r); err != nil {
		t.Fatal(err)
	}
	if err := settleNativeIntent(r); err == nil {
		t.Fatal("replacement became cleanup authority")
	}
	if _, err := os.Stat(c); err != nil {
		t.Fatal("replacement deleted", err)
	}
	if b, err := os.ReadFile(pin); err != nil || string(b) != "original-A" {
		t.Fatal("original evidence lost", err)
	}
}

func TestHRW015TerminalCleanupAndUnknownAnchorsStayDistinct(t *testing.T) {
	for _, outcome := range []string{"accepted", "refused", "uncertain", "missing-pin", "foreign-pin"} {
		t.Run(outcome, func(t *testing.T) {
			r, c, pin := fixtureNativePin(t)
			r.Outcome = outcome
			if outcome == "missing-pin" || outcome == "foreign-pin" {
				r.Outcome = "accepted"
				if err := os.Remove(pin); err != nil {
					t.Fatal(err)
				}
				if outcome == "foreign-pin" {
					if err := os.WriteFile(pin, []byte("foreign"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := writeNativeIntent(r); err != nil {
				t.Fatal(err)
			}
			err := settleNativeIntent(r)
			if outcome == "missing-pin" || outcome == "foreign-pin" {
				if err == nil {
					t.Fatal("unknown anchor authorized cleanup")
				}
				if _, err := os.Stat(c); err != nil {
					t.Fatal("original claim removed", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if outcome == "uncertain" {
				if _, err := os.Stat(pin); err != nil {
					t.Fatal("uncertain evidence retired", err)
				}
				if _, err := os.Stat(nativeIntentPath(r)); err != nil {
					t.Fatal("uncertain payload/ID retired", err)
				}
			} else {
				if _, err := os.Stat(pin); !os.IsNotExist(err) {
					t.Fatal("terminal anchor not retired")
				}
				if _, err := os.Stat(nativeIntentPath(r)); !os.IsNotExist(err) {
					t.Fatal("terminal journal not retired")
				}
				if outcome == "refused" {
					if b, err := os.ReadFile(claimBase(c)); err != nil || string(b) != "original-A" {
						t.Fatal("refused work lost", err)
					}
				}
			}
		})
	}
}

func TestHRW015UnknownMetadataCannotSupplyAnIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "claim")
	if err := os.WriteFile(path, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"stat-error", "zero-time", "invalid-time", "changed-during-read"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			stat := func(fd int, s *unix.Stat_t) error {
				calls++
				if mode == "stat-error" {
					return errors.New("fixture stat unavailable")
				}
				if err := unix.Fstat(fd, s); err != nil {
					return err
				}
				if mode == "zero-time" {
					s.Ctim.Sec = 0
				}
				if mode == "invalid-time" {
					s.Ctim.Nsec = -1
				}
				if mode == "changed-during-read" && calls == 2 {
					s.Ctim.Sec++
				}
				return nil
			}
			if _, err := nativeFingerprintWithStat(path, stat); err == nil {
				t.Fatal("unknown/changing metadata granted identity")
			}
			if b, err := os.ReadFile(path); err != nil || string(b) != "private" {
				t.Fatal("read-only observation mutated claim", err)
			}
		})
	}
}
func TestHRW015AnchorBudgetHoldsBeforeJournalOrInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	x := f.io()
	enqueue(x, "unsubmitted")
	claims, _, _ := claimBatch(queuedNames(func(string) bool { return true }))
	if len(claims) != 1 {
		t.Fatal("missing claim")
	}
	dir := filepath.Join(state.Dir(), "hq-nudge-native-claims")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// Contract limit, including retained pre-intent/uncertain evidence. No cleanup
	// guesses are permitted merely to make room for another submission.
	for i := 0; i < 1600; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("retained-%04d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := prepareNativeIntent("%isolated", "#123abc", "unsubmitted · #123abc", claims); err == nil {
		t.Fatal("exhausted anchors admitted new native mutation")
	}
	if pins, err := os.ReadDir(dir); err != nil || len(pins) != 1600 {
		t.Fatal("budget operation removed evidence or created a new pin", err)
	}
	if journals, err := os.ReadDir(nativeIntentDir()); !os.IsNotExist(err) && (err != nil || len(journals) != 0) {
		t.Fatal("budget failure published intent", err)
	}
	if b, err := os.ReadFile(claims[0]); err != nil || string(b) != "unsubmitted" {
		t.Fatal("budget failure consumed claim", err)
	}
	mutations := 0
	x.nativeSubmit = func(string, string, string) (dispatch.Result, bool) {
		mutations++
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	enqueue(x, "independent-new")
	drainInto(x, "%isolated")
	if mutations != 0 {
		t.Fatal("exhausted pin budget reached native transport")
	}
	if pins, err := os.ReadDir(dir); err != nil || len(pins) != 1600 {
		t.Fatal("transport admission changed retained pins", err)
	}
}

func TestHRW015UncertainRestartRequiresItsRetainedAnchor(t *testing.T) {
	for _, mode := range []string{"missing", "foreign"} {
		t.Run(mode, func(t *testing.T) {
			r, c, pin := fixtureNativePin(t)
			if err := settleNativeIntent(r); err != nil {
				t.Fatal(err)
			} // prepared -> parked uncertain
			if err := os.Remove(pin); err != nil {
				t.Fatal(err)
			}
			if mode == "foreign" {
				if err := os.WriteFile(pin, []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			protected, err := recoverNativeIntentsWithProtection()
			if err == nil || !protected[filepath.Base(c)] {
				t.Fatal("uncertain restart with unknown anchor lost exact claim protection", protected, err)
			}
			if b, err := os.ReadFile(claimBase(c) + nativeUncertainSuffix); err != nil || string(b) != "original-A" {
				t.Fatal("uncertain payload lost", err)
			}
			if _, err := os.Stat(nativeIntentPath(r)); err != nil {
				t.Fatal("uncertain journal lost", err)
			}
		})
	}
}
