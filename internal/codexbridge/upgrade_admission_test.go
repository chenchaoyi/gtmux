package codexbridge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCFR009PostUpgradeUsesActualCancellationBeforeDeadline(t *testing.T) {
	for _, kind := range []string{"cancelled", "expired", "live", "unbounded"} {
		t.Run(kind, func(t *testing.T) {
			var epoch connectionEpoch
			incumbent := epoch.reserve()
			if !epoch.activate(incumbent) {
				t.Fatal("fixture incumbent missing")
			}
			next := epoch.reserve()
			ctx := context.Background()
			cancel := func() {}
			switch kind {
			case "cancelled":
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(time.Hour))
				cancel()
			case "expired":
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
			case "live":
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(time.Hour))
			}
			defer cancel()
			active, err := completeAdmission(ctx, &epoch, next)
			if kind == "live" {
				if err != nil || !active || !epoch.current(next) || epoch.current(incumbent) {
					t.Fatal(active, err)
				}
				return
			}
			if active || epoch.current(next) || !epoch.current(incumbent) {
				t.Fatal("rejected completion changed incumbent", active, err)
			}
			want, reason := error(context.DeadlineExceeded), "admission-deadline"
			if kind == "cancelled" {
				want, reason = context.Canceled, "admission-cancelled"
			}
			if !errors.Is(err, want) || admissionReason(err) != reason {
				t.Fatal(err, admissionReason(err), reason)
			}
		})
	}
}
