package hqnudge

import (
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
)

// A native uncertain batch is retained, never reclaimed/coalesced into a new
// delivery identity. The input spool carries the exact acceptance uncertainty.
const nativeUncertainSuffix = ".native-uncertain"

var nativeWakeIO = dispatchbridge.DispatchIO
var nativeWakeTarget = hqpane.Find

// Preparation performs no input mutation; journal admission happens before submit.
func prepareNativeWake(pane string) func(payload, id string) (dispatch.Result, bool) {
	io := nativeWakeIO(pane, "codex")
	if io.SubmitNative == nil {
		return nil
	}
	io.Hold = func() string {
		if nativeWakeTarget() != pane {
			return "hq-target-changed"
		}
		return ""
	}
	return func(payload, id string) (dispatch.Result, bool) {
		return dispatch.Deliver(io, dispatch.Opts{Pane: pane, HasComposer: true, DeliveryID: "hq-wake-" + id, IdleWakeInput: true}, payload), true
	}
}

func nativeWakeSubmit(pane, payload, id string) (dispatch.Result, bool) {
	submit := prepareNativeWake(pane)
	if submit == nil {
		return dispatch.Result{}, false
	}
	return submit(payload, id)
}
