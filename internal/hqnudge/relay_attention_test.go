package hqnudge

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
)

func TestHRA001ReservedRelaySharesFullDecisionBatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{base: time.Now().Add(time.Minute).UnixNano()}
	x := f.io()
	p0 := func(i int) string { return hqwake.Line(hqwake.ClassAgentRelay, fmt.Sprintf("decision-%03d", i)) }
	for i := range maxQueueEntries {
		enqueue(x, p0(i))
		f.nano++
	}
	ordinary := queuedNames(func(n string) bool { return !isRelayAttention(n) })
	if len(ordinary) != maxQueueEntries || prioOf(ordinary[len(ordinary)-1]) != 0 {
		t.Fatalf("fixture did not fill decision queue: %d", len(ordinary))
	}
	recorded := 0
	admit := func() (bool, error) { recorded++; return true, nil }
	line := "relay:protected-formal-review"
	if ok, err := ReserveRelayAttention(line, admit); !ok || err != nil {
		t.Fatal("decision pressure denied the protected opportunity", err)
	}
	if ok, err := ReserveRelayAttention("relay:another", admit); ok || err != nil || recorded != 1 {
		t.Fatalf("reserved capacity or source reservation was exceeded: %t %v %d", ok, err, recorded)
	}
	before := queuedNames(func(n string) bool { return !isRelayAttention(n) })
	if strings.Join(before, ",") != strings.Join(ordinary, ",") {
		t.Fatal("reserve changed a decision file")
	}
	reserved := queuedNames(isRelayAttention)
	if len(reserved) != 1 || strings.HasSuffix(reserved[0], ".txt") {
		t.Fatalf("old queue reader could treat protected work as ordinary: %v", reserved)
	}
	x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
		if strings.Count(payload, line) != 1 || strings.Count(payload, "decision-") != maxBatchLines {
			t.Fatalf("unexpected native batch %q", payload)
		}
		return dispatch.Result{State: dispatch.StateLanded}, true
	}
	drain(x, "%16")
	after := queuedNames(func(n string) bool { return !isRelayAttention(n) })
	if len(after) != maxQueueEntries-maxBatchLines || strings.Join(after, ",") != strings.Join(ordinary[maxBatchLines:], ",") ||
		len(queuedNames(isRelayAttention)) != 0 || f.pastes != 0 || f.enters != 0 {
		t.Fatal("drain trimmed or evicted existing decisions or used legacy input")
	}
}

func TestHRA002ProtectedRelayRequiresGuardedNativeWindow(t *testing.T) {
	for _, mode := range []string{"draft", "busy", "copy", "unreadable", "identity", "unknown", "safe refusal"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			f := &fake{base: time.Now().Add(time.Minute).UnixNano()}
			x := f.io()
			calls := 0
			x.nativeSubmit = func(string, string, string) (dispatch.Result, bool) {
				calls++
				if mode == "safe refusal" && calls == 1 {
					return dispatch.Result{State: dispatch.StateRefusedDraft, Submission: dispatch.SubmissionNotSubmitted, FailureCode: dispatch.InputDraftPresent}, true
				}
				return dispatch.Result{State: dispatch.StateFailed, Submission: dispatch.SubmissionUnknown}, true
			}
			switch mode {
			case "draft":
				f.draft = "keep my real draft"
			case "busy":
				x.busy = func(string) bool { return true }
			case "copy":
				f.inMode = true
			case "unreadable":
				f.unstructured = true
			case "identity":
				x.identity = func(string) string { return "" }
			}
			if ok, err := ReserveRelayAttention("relay:original", func() (bool, error) { return true, nil }); !ok || err != nil {
				t.Fatal("fixture could not reserve a report", err)
			}
			drain(x, "%16")
			if mode == "safe refusal" {
				due := queuedNames(isRelayAttention)
				if len(due) != 1 || strings.HasSuffix(due[0], ".txt") {
					t.Fatal("positive refusal was not held as an old-reader-invisible retry", due)
				}
				attempt, good := parseRetry(due[0])
				if !good || attempt.Number != 1 || attempt.Target != wakeIdentity(x, "%16") || attempt.At <= x.nowNano() {
					t.Fatalf("retry lost its backoff and target: %+v", attempt)
				}
				drain(x, "%16")
				if calls != 1 {
					t.Fatal("retry ran before backoff")
				}
				f.nano += int64(6 * time.Second)
				drain(x, "%16")
			}
			want := 0
			switch mode {
			case "unknown":
				want = 1
			case "safe refusal":
				want = 2
			}
			drain(x, "%16")
			if calls != want || f.pastes != 0 || f.enters != 0 {
				t.Fatalf("unsafe or repeated input, native=%d paste=%d enter=%d", calls, f.pastes, f.enters)
			}
			items := readQueue()
			if len(items) != 1 || (want > 0 && !strings.HasSuffix(items[0].Name(), relayAttentionSuffix+nativeUncertainSuffix)) ||
				(want == 0 && !isRelayAttention(items[0].Name())) {
				t.Fatal("original protected queue or unknown evidence was not preserved", items)
			}
		})
	}
}
