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
var nativeWakeIdentity = liveWakeIdentity

// Preparation performs no input mutation; journal admission happens before submit.
func prepareNativeWake(pane string) func(payload, id string) (dispatch.Result, bool) {
	return prepareNativeWakeFor(pane, dispatch.Opts{}, nil)
}

func prepareNativeWakeFor(pane string, opts dispatch.Opts, hold func() string) func(payload, id string) (dispatch.Result, bool) {
	io := nativeWakeIO(pane, "codex")
	if io.SubmitNative == nil {
		return nil
	}
	expected := nativeWakeIdentity(pane)
	io.Hold = func() string {
		if nativeWakeTarget() != pane || (expected != "" && nativeWakeIdentity(pane) != expected) {
			return "hq-target-changed"
		}
		if hold != nil {
			return hold()
		}
		return ""
	}
	opts.Pane, opts.HasComposer, opts.IdleWakeInput = pane, true, true
	return func(payload, id string) (dispatch.Result, bool) {
		delivery := opts
		delivery.DeliveryID = "hq-wake-" + id
		return dispatch.Deliver(io, delivery, payload), true
	}
}

func nativeWakeSubmit(pane, payload, id string) (dispatch.Result, bool) {
	submit := prepareNativeWake(pane)
	if submit == nil {
		return dispatch.Result{}, false
	}
	return submit(payload, id)
}
