package dispatch

import "testing"

func TestNativeDeliveryUsesDraftAndHoldGuardsWithoutTyping(t *testing.T) {
	for _, mode := range []string{"verified", "fast"} {
		for _, blocked := range []string{"", "draft", "hold"} {
			t.Run(mode+"/"+blocked, func(t *testing.T) {
				f := &fakeIO{preDraft: ""}
				if blocked == "draft" {
					f.preDraft = "unfinished human input"
				}
				io := f.io()
				calls := 0
				io.SubmitNative = func(o Opts, text string, hold func() string) Result {
					calls++
					if o.DeliveryID != "native-id" || text != taskText {
						t.Fatal("input identity changed")
					}
					return Result{State: StateQueued, JudgedBy: JudgedByDriver}
				}
				if blocked == "hold" {
					io.Hold = func() string { return "permission menu" }
				}
				opts := Opts{Pane: "%9", HasComposer: true, DeliveryID: "native-id"}
				if mode == "verified" {
					r := Deliver(io, opts, taskText)
					if blocked == "" && (r.State != StateQueued || r.Delivered) {
						t.Fatal("queue acceptance claimed execution", r)
					}
				} else {
					ok, _ := PasteAndSubmit(io, opts, taskText)
					if ok != (blocked == "") {
						t.Fatal("fast delivery bypassed guard", ok)
					}
				}
				want := 0
				if blocked == "" {
					want = 1
				}
				if calls != want || f.pasteCalls != 0 || f.enterCalls != 0 {
					t.Fatal("native request typed or bypassed a guard", calls, f.pasteCalls, f.enterCalls)
				}
			})
		}
	}
}
