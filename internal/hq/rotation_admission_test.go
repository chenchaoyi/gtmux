package hq

import (
	"os"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqinput"
)

func TestHRW007RegistrationSharesTheInputClaimLease(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	release, acquired, err := hqinput.Acquire()
	if err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	defer func() { release() }()
	r := rotationRequest{Pane: "%6", Agent: "codex", Retiring: "old-session", Input: "/new", CreatedAt: time.Now().Unix()}
	if _, ok, _ := queueHQRotation(r); ok {
		t.Fatal("registration crossed an already held input claim lease")
	}
	if _, err := os.Stat(rotationPendingPath()); !os.IsNotExist(err) {
		t.Fatal("contended registration wrote pending state", err)
	}
	release()
	release = func() {}
	first, ok, why := queueHQRotation(r)
	if !ok {
		t.Fatal(why)
	}
	before, err := os.ReadFile(rotationPendingPath())
	if err != nil {
		t.Fatal(err)
	}
	r.CreatedAt += 60
	next, ok, why := queueHQRotation(r)
	if !ok || next.CreatedAt != first.CreatedAt {
		t.Fatal("retry changed request deadline", next, ok, why)
	}
	r.Retiring = "foreign-session"
	if _, ok, _ := queueHQRotation(r); ok {
		t.Fatal("foreign request replaced reservation")
	}
	after, err := os.ReadFile(rotationPendingPath())
	if err != nil || string(before) != string(after) {
		t.Fatal("original reservation changed", err)
	}
}
