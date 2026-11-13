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

func nativeWakeSubmit(pane, payload, id string) (dispatch.Result, bool) {
	io := nativeWakeIO(pane, "codex")
	if io.SubmitNative == nil {
		return dispatch.Result{}, false
	}
	io.Hold = func() string {
		if nativeWakeTarget() != pane {
			return "hq-target-changed"
		}
		return ""
	}
	return dispatch.Deliver(io, dispatch.Opts{Pane: pane, HasComposer: true, DeliveryID: "hq-wake-" + id, IdleWakeInput: true}, payload), true
}
