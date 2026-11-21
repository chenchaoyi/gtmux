package hqnudge

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
)

func TestHWW004ProtectedRelayUsesFirstWindowWithDueWaitingRetry(t *testing.T) {
	for _, name := range []string{"same target", "different target"} {
		t.Run(name, func(t *testing.T) {
			worker := newWaitingFixture(t)
			wait := queueDraftWait(t, worker, "%61")
			hq := &fake{base: time.Now().Add(time.Minute).UnixNano()}
			x := hq.io()
			const originalTarget = "0123456789abcdef0123456789abcdef"
			target := originalTarget
			x.identity = func(string) string { return target }
			type attempt struct{ payload, id, target string }
			var attempts []attempt
			x.nativeSubmit = func(_, payload, id string) (dispatch.Result, bool) {
				attempts = append(attempts, attempt{payload, id, target})
				if len(attempts) == 1 {
					return dispatch.Result{State: dispatch.StateRefusedDraft, Submission: dispatch.SubmissionNotSubmitted,
						FailureCode: dispatch.InputDraftPresent}, true
				}
				return dispatch.Result{State: dispatch.StateQueued}, true
			}
			drain(x, "%16")
			items := queuedNames(func(n string) bool { return strings.HasSuffix(n, waitingWakeSuffix) })
			if len(attempts) != 1 || len(items) != 1 {
				t.Fatalf("fixture did not schedule one waiting retry: attempts=%v items=%v", attempts, items)
			}
			retry, valid := parseRetry(items[0])
			original, err := readNativeBytes(filepath.Join(queueDir(), items[0]))
			if err != nil || !valid || retry.Number != 1 || retry.Target != originalTarget || retry.At <= x.nowNano() {
				t.Fatalf("fixture lost the original retry fence: %+v %v", retry, err)
			}
			hq.nano += int64(6 * time.Second)
			if dueForAttempt(items[0]) > x.nowNano() {
				t.Fatal("fixture did not advance past the retry backoff")
			}
			relay := hqwake.Line(hqwake.ClassTick, "workspace relay follow-up", "relay:synthetic-competing-retry", "pull: gtmux relay show synthetic-competing-retry")
			if ok, err := ReserveRelayAttention(relay, func() (bool, error) { return true, nil }); !ok || err != nil {
				t.Fatal("could not reserve the new relay", err)
			}
			if name == "different target" {
				target = "123456789abcdef0123456789abcdef0"
			}
			hq.draft = "preserve this draft"
			drain(x, "%16")
			if len(attempts) != 1 || hq.draft != "preserve this draft" {
				t.Fatal("a pending retry or report bypassed the draft guard")
			}
			hq.draft = ""
			drain(x, "%16")
			if len(attempts) != 2 || strings.Count(attempts[1].payload, relay) != 1 ||
				strings.Contains(attempts[1].payload, wait) || attempts[1].target != target {
				t.Fatalf("the first eligible window did not contain only the new relay: %+v", attempts)
			}
			left := queuedNames(func(string) bool { return true })
			if len(left) != 1 || left[0] != items[0] {
				t.Fatalf("the first eligible window changed the original retry generation: %v", left)
			}
			remaining, err := readNativeBytes(filepath.Join(queueDir(), left[0]))
			if err != nil || !bytes.Equal(original, remaining) {
				t.Fatal("the first eligible window changed the original retry content", err)
			}

			target = originalTarget
			drain(x, "%16")
			drain(x, "%16")
			if len(attempts) != 3 || !strings.Contains(attempts[2].payload, wait) ||
				strings.Contains(attempts[2].payload, relay) || attempts[2].target != originalTarget ||
				attempts[2].id != nativeAttemptID(attempts[0].id, retry) || hq.pastes != 0 || hq.enters != 0 {
				t.Fatalf("the ordinary retry must retain its target and next attempt ID and neither line can replay: %+v", attempts)
			}
		})
	}
}
