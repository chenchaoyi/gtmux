package hqnudge

import (
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"os"
	"strings"
	"testing"
)

func TestNativeWakeAcceptanceAndUncertaintyNeverPasteOrReplay(t *testing.T) {
	for _, state := range []dispatch.State{dispatch.StateQueued, dispatch.StateFailed, dispatch.StateRefusedDraft} {
		t.Run(string(state), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			f := &fake{}
			x := f.io()
			calls := 0
			x.nativeSubmit = func(pane, payload, id string) (dispatch.Result, bool) {
				calls++
				return dispatch.Result{State: state}, true
			}
			enqueue(x, "synthetic native wake")
			drainInto(x, "%hq")
			if calls != 1 || len(f.sent) != 0 {
				t.Fatal("native wake typed", calls, f.sent)
			}
			if state == dispatch.StateFailed {
				if len(readQueue()) != 1 {
					t.Fatal("uncertain batch not retained", len(readQueue()))
				}
				for _, e := range readQueue() {
					if !strings.HasSuffix(e.Name(), nativeUncertainSuffix) {
						t.Fatal("uncertain wake automatically requeued", e.Name())
					}
					if b, err := os.ReadFile(queueDir() + "/" + e.Name()); err != nil || !strings.Contains(string(b), "synthetic native wake") {
						t.Fatal("uncertain payload lost", err)
					}
				}
				drainInto(x, "%hq")
				if calls != 1 {
					t.Fatal("uncertain native wake replayed")
				}
			}
			if state == dispatch.StateQueued && len(readQueue()) != 0 {
				t.Fatal("accepted wake left queued")
			}
		})
	}
}
