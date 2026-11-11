package app

import (
	"bytes"
	"context"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/screenshot"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestASC004And005ScreenshotDeliveryUsesExactTargetAndDurableAcceptance(t *testing.T) {
	for _, state := range []string{"queued", "active", "landed", "unknown", "changed", "waiting", "fallback"} {
		t.Run(state, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			target := screenshot.Target{Version: 1, Pane: "%9", Loc: "synthetic:0.0", Agent: "codex", SessionID: "s", HomeID: "h", Instance: "i", ClientPID: 100, PanePID: 99, ServerPID: 90, ServerStart: "birth"}
			token, e := screenshot.MintTarget(target, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			oldT, oldP, oldD, oldS, oldH, oldIO := screenshotCurrentTarget, screenshotPhysical, screenshotDispatch, screenshotSaveAttachment, screenshotHold, screenshotIO
			t.Cleanup(func() {
				screenshotCurrentTarget, screenshotPhysical, screenshotDispatch, screenshotSaveAttachment, screenshotHold, screenshotIO = oldT, oldP, oldD, oldS, oldH, oldIO
			})
			screenshotCurrentTarget = func(string) (screenshot.Target, error) {
				v := target
				if state == "changed" {
					v.SessionID = "other"
				}
				return v, nil
			}
			screenshotPhysical = func(string) (int, string, int, string, error) { return 99, "synthetic:0.0", 90, "birth", nil }
			screenshotHold = func(string) string {
				if state == "waiting" {
					return "approval"
				}
				return ""
			}
			screenshotIO = func(string, string) dispatch.IO {
				if state == "fallback" {
					return dispatch.IO{}
				}
				return dispatch.IO{SubmitNative: func(dispatch.Opts, string, func() string) dispatch.Result { return dispatch.Result{} }}
			}
			saved, calls := 0, 0
			screenshotSaveAttachment = func(name string, data []byte) (string, error) { saved++; return filepath.Join(t.TempDir(), name), nil }
			screenshotDispatch = func(d dispatch.IO, o dispatch.Opts, text string) dispatch.Result {
				calls++
				if o.ExpectedSessionID != "s" || o.ExpectedHomeID != "h" || o.ExpectedInstance != "i" || o.ExpectedClientPID != 100 || o.Context == nil || !o.ActiveImageInput {
					t.Fatal("exact binding missing", o)
				}
				if d.Hold() == "" && state == "waiting" {
					t.Fatal("approval bypassed")
				}
				switch state {
				case "queued":
					return dispatch.Result{State: dispatch.StateQueued}
				case "active":
					return dispatch.Result{State: dispatch.StateQueued, Evidence: "codex-active-accepted:turn"}
				case "landed":
					return dispatch.Result{State: dispatch.StateLanded, Delivered: true}
				}
				return dispatch.Result{State: dispatch.StateFailed}
			}
			var b bytes.Buffer
			_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
			path := filepath.Join(t.TempDir(), "in.png")
			_ = os.WriteFile(path, b.Bytes(), 0600)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			id := "synthetic-send-0001"
			r, e := screenshotSend(ctx, path, token, id, "")
			if state == "changed" {
				if e == nil || saved != 0 || calls != 0 {
					t.Fatal("retargeted", r, e)
				}
				return
			}
			if state == "waiting" || state == "fallback" {
				if e == nil || r.State != "blocked" || calls != 0 {
					t.Fatal(r, e, calls)
				}
				return
			}
			want := state
			if state == "active" {
				want = "active_turn_accepted"
			}
			if r.State != want {
				t.Fatal(r, e)
			}
			if (e != nil) != (state == "unknown") {
				t.Fatal(r, e)
			}
			if r.Confirmed != (state == "active" || state == "landed") {
				t.Fatal("acceptance misrepresented", r)
			}
			_, _ = screenshotSend(ctx, path, token, id, "")
			if calls != 1 || saved != 1 {
				t.Fatal("duplicate replay", calls, saved)
			}
			got, e := screenshot.ReadReceipt(id)
			if e != nil || got.State != want {
				t.Fatal(got, e)
			}
		})
	}
}
