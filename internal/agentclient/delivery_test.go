package agentclient

import (
	"errors"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeDeliveryNeedsAddressedAcceptanceAndPreservesDrafts(t *testing.T) {
	for _, name := range []string{"accepted", "switched", "menu", "draft", "revoked", "duplicate"} {
		t.Run(name, func(t *testing.T) {
			_, a, _ := attachmentFixture(t)
			frame := "⌸ ❯ "
			pastes, enters := 0, 0
			if name == "draft" {
				frame = "⌸ ❯ unfinished private draft"
			}
			if name == "menu" {
				frame = "Select a session\n❯ 1. synthetic"
			}
			if name == "revoked" {
				a.InputReady = false
			}
			io := dispatch.IO{Capture: func() string { return frame }, CaptureColor: func() string { return frame }, Now: func() int64 { return time.Now().Unix() }, Sleep: func() {}, Paste: func(text string) error { pastes++; frame = "⌸ ❯ " + text; return nil }, Enter: func() error {
				enters++
				matches, _ := filepath.Glob(filepath.Join(InputDir(), "*.pending.json"))
				if len(matches) != 1 {
					return errors.New("pending request missing")
				}
				var r Request
				if err := readOwnedJSON(matches[0], &r, 2<<20); err != nil {
					return err
				}
				putJSON(t, filepath.Join(InputDir(), r.ID+".receipt.json"), Receipt{Version: 1, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID, SessionID: r.SessionID, PID: r.PID, State: "accepted"})
				frame = "⌸ ❯ "
				return nil
			}, InMode: func() bool { return false }}
			lookup := func(string) (Attachment, bool) {
				if name == "switched" && pastes > 0 {
					b := a
					b.SessionID = "different"
					return b, true
				}
				return a, true
			}
			opts := dispatch.Opts{Pane: a.Pane, HasComposer: true, DeliveryID: "delivery"}
			ok, _ := dispatch.PasteAndSubmit(WrapIO(a.Pane, io, lookup), opts, "Synthetic addressed input")
			want := name == "accepted" || name == "duplicate"
			if ok != want {
				t.Fatalf("%s accepted=%v pastes=%d enters=%d", name, ok, pastes, enters)
			}
			if !want && enters != 0 {
				t.Fatal("unverified input was submitted")
			}
			if name == "draft" && pastes != 0 {
				t.Fatal("draft was modified")
			}
			if name == "duplicate" {
				again, _ := dispatch.PasteAndSubmit(WrapIO(a.Pane, io, lookup), opts, "Synthetic addressed input")
				if !again || pastes != 1 || enters != 1 {
					t.Fatal("same delivery enqueued twice")
				}
			}
		})
	}
}
func TestNativeSpoolRetentionAndUnknownClaims(t *testing.T) {
	_, a, inspect := attachmentFixture(t)
	now := inspect.Now
	r, err := Prepare(a, "synthetic", "retry-key", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(InputDir(), r.ID+".claimed.json"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(a, "synthetic", "retry-key", false, now.Add(48*time.Hour)); err == nil {
		t.Fatal("unknown claim retried after timeout")
	}
	if _, err := os.Stat(filepath.Join(InputDir(), r.ID+".claimed.json")); err != nil {
		t.Fatal("unknown claim was pruned")
	}
	putJSON(t, filepath.Join(InputDir(), r.ID+".receipt.json"), Receipt{Version: 1, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID, SessionID: r.SessionID, PID: r.PID, State: "accepted"})
	next, err := Prepare(a, "synthetic", "retry-key", false, now.Add(48*time.Hour))
	if err != nil || next.ID == r.ID {
		t.Fatal("completed expired metadata did not age out")
	}
}
func TestDSTFrameKeepsDraftAndDropsOnlyComposerChrome(t *testing.T) {
	in := "history mentions ⌸ ❯ stay\n\x1b[2m⌸ \x1b[22m❯ INSERT user draft \x1b[2m⛶\x1b[22m"
	got := dstFrame(in, true)
	if !strings.HasSuffix(got, "❯ user draft") || !strings.HasPrefix(got, "history mentions ⌸ ❯ stay") {
		t.Fatal(got)
	}
}

func TestDeliveryIDCannotRetargetAfterSwitchOrRestart(t *testing.T) {
	_, a, inspect := attachmentFixture(t)
	r, err := Prepare(a, "synthetic", "stable-http-id", false, inspect.Now)
	if err != nil {
		t.Fatal(err)
	}
	switched := a
	switched.SessionID = "another"
	switched.Instance = strings.Repeat("b", 32)
	switched.BindingEpoch++
	if _, err = Prepare(switched, "synthetic", "stable-http-id", false, inspect.Now.Add(time.Minute)); err == nil {
		t.Fatal("stable ID retargeted after timeout")
	}
	if _, err = Prepare(a, "different body", "stable-http-id", false, inspect.Now); err == nil {
		t.Fatal("stable ID reused for another payload")
	}
	putJSON(t, filepath.Join(InputDir(), r.ID+".receipt.json"), Receipt{Version: 1, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID, SessionID: r.SessionID, PID: r.PID, State: "accepted"})
	got, err := Prepare(switched, "synthetic", "stable-http-id", false, inspect.Now.Add(time.Minute))
	if err != nil || got.ID != r.ID {
		t.Fatal("accepted old target did not remain idempotent", err)
	}
}
