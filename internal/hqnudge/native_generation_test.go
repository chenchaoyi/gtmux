package hqnudge

import (
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHRW014GenerationChangeWithSameInodeBodyAndMtimeCannotBeDeleted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	x := f.io()
	enqueue(x, "original-A")
	claimed, _, _ := claimBatch(queuedNames(func(string) bool { return true }))
	if len(claimed) != 1 {
		t.Fatal("missing claim")
	}
	c := claimed[0]
	r, err := prepareNativeIntent("%isolated", "#123abc", "original-A · #123abc", claimed)
	if err != nil {
		t.Fatal(err)
	}
	var before, after unix.Stat_t
	if err := unix.Stat(c, &before); err != nil {
		t.Fatal(err)
	}
	// Explicit kernel metadata transitions, no sleeps or inode-allocation lottery.
	if err := os.Chmod(c, 0400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(c, 0600); err != nil {
		t.Fatal(err)
	}
	if err := unix.Stat(c, &after); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(c)
	if err != nil {
		t.Fatal(err)
	}
	if before.Dev != after.Dev || before.Ino != after.Ino || before.Mtim != after.Mtim || string(b) != "original-A" {
		t.Fatal("fixture changed inode/body/mtime")
	}
	if before.Ctim == after.Ctim {
		t.Fatal("fixture did not establish changed ctime; not a product assertion")
	}
	t.Logf("same dev/inode=%d/%d mtime=%v, ctime before=%v after=%v", before.Dev, before.Ino, before.Mtim, before.Ctim, after.Ctim)
	r.Outcome = "accepted"
	if err := writeNativeIntent(r); err != nil {
		t.Fatal(err)
	}
	if err := settleNativeIntent(r); err == nil {
		t.Error("generation change was accepted as unchanged claim identity")
	}
	if _, err := os.Lstat(c); err != nil {
		t.Fatal("recovery deleted the changed-generation claim", err)
	}
	// A held claim protects its destination from rename-overwrite, but does not
	// authorize a channel-wide outage for an independently identified new event.
	queuedReplacement := claimBase(c)
	if err := os.WriteFile(queuedReplacement, []byte("foreign-slot-B"), 0600); err != nil {
		t.Fatal(err)
	}
	enqueue(x, "independent-C")
	var submissions []string
	x.nativeSubmit = func(_ string, text, _ string) (dispatch.Result, bool) {
		submissions = append(submissions, text)
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	drainInto(x, "%isolated")
	if len(submissions) != 1 || !strings.Contains(submissions[0], "independent-C") || strings.Contains(submissions[0], "foreign-slot-B") || strings.Contains(submissions[0], "original-A") {
		t.Fatal("conflicting slot overwrote claim or blocked fresh C", submissions)
	}
	if b, err := os.ReadFile(c); err != nil || string(b) != "original-A" {
		t.Fatal("held claim was overwritten by queued replacement", err, string(b))
	}
	if b, err := os.ReadFile(queuedReplacement); err != nil || string(b) != "foreign-slot-B" {
		t.Fatal("held queued replacement was consumed", err, string(b))
	}
}

func TestHRW014UncertainPublicationOnlyRetiresItsExactOwnedLink(t *testing.T) {
	for _, mode := range []string{"exact", "changed-payload", "foreign-link"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			f := &fake{}
			x := f.io()
			enqueue(x, "original-A")
			claimed, _, _ := claimBatch(queuedNames(func(string) bool { return true }))
			if len(claimed) != 1 {
				t.Fatal("missing claim")
			}
			c := claimed[0]
			r, err := prepareNativeIntent("%isolated", "#123abc", "original-A · #123abc", claimed)
			if err != nil {
				t.Fatal(err)
			}
			parked := claimBase(c) + nativeUncertainSuffix
			if mode == "foreign-link" {
				if err := os.WriteFile(parked, []byte("original-A"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Link(c, parked); err != nil {
					t.Fatal(err)
				} // process stops before unlink
				if mode == "changed-payload" {
					if err := os.WriteFile(parked, []byte("changed"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			err = recoverNativeIntents()
			if mode == "exact" {
				if err != nil {
					t.Fatal("own interrupted park publication was blocked", err)
				}
				if _, err := os.Lstat(c); !os.IsNotExist(err) {
					t.Fatal("extra claim link not retired")
				}
				if b, err := os.ReadFile(parked); err != nil || string(b) != "original-A" {
					t.Fatal("original uncertainty lost", err)
				}
				if b, err := os.ReadFile(nativeIntentPath(r)); err != nil || len(b) == 0 {
					t.Fatal("original payload/ID evidence lost", err)
				}
			} else {
				if err == nil {
					t.Fatal("changed/foreign publication was accepted")
				}
				if _, err := os.Lstat(c); err != nil {
					t.Fatal("foreign conflict deleted original claim", err)
				}
				if _, err := os.Lstat(parked); err != nil {
					t.Fatal("foreign conflict deleted parked evidence", err)
				}
			}
			if filepath.Dir(parked) != queueDir() {
				t.Fatal("fixture escaped queue")
			}
		})
	}
}
