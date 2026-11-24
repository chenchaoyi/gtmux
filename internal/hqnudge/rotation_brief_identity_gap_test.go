package hqnudge

import (
	"slices"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
)

func TestBoundRotationBriefWaitsOnlyForMissingOriginalProof(t *testing.T) {
	for _, mode := range []string{"preflight", "native-lookup", "final-guard", "changed", "wrong-stamp", "expired", "uncertain"} {
		t.Run(mode, func(t *testing.T) {
			bind, _, f, target := rotationBriefFixture(t)
			original := *target
			missing := rotationBriefTarget{Stamp: original.Stamp, MissingProof: true}
			nativeWakeIdentity = func(pane string) string {
				if target.MissingProof {
					return ""
				}
				return prod.identity(pane)
			}
			calls, accepted := 0, 0
			nativeWakeIO = func(string, ...string) dispatch.IO {
				return dispatch.IO{
					CaptureColor: func() string { return f.capture(bind.Pane) }, Sleep: func() {},
					SubmitNative: func(opts dispatch.Opts, _ string, guard func() string) dispatch.Result {
						calls++
						if calls == 1 && (mode == "native-lookup" || mode == "final-guard" || mode == "uncertain") {
							*target = missing
							if mode == "native-lookup" {
								return dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted,
									Evidence: "codex-frontend-proof-unavailable"}
							}
							if why := guard(); why != briefTargetWaiting {
								t.Fatalf("lost exact target on the final guard: %q", why)
							}
							if mode == "uncertain" {
								return dispatch.Result{Submission: dispatch.SubmissionUnknown}
							}
							return dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted}
						}
						if !opts.MatchesTarget(bind.Session, bind.Home, bind.Instance, bind.ClientPID) || guard() != "" {
							t.Fatal("input had no exact binding")
						}
						accepted++
						return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
							Request: "receipt", Session: bind.Session, Home: bind.Home, Turn: "turn",
						}}
					},
				}
			}
			queueBoundBrief(t, bind)
			before := queuedNames(isRotationBrief)
			switch mode {
			case "preflight", "expired", "changed", "wrong-stamp":
				*target = missing
				if mode == "changed" {
					target.Session = "different-session"
				} else if mode == "wrong-stamp" {
					target.Stamp = "10:20:31"
				}
			}
			Drain(bind.Pane)
			if mode == "expired" {
				f.nano += int64(3 * time.Second)
			}
			Drain(bind.Pane)
			wantRecover := mode == "preflight" || mode == "native-lookup" || mode == "final-guard"
			after := queuedNames(isRotationBrief)
			if accepted != 0 || wantRecover && !slices.Equal(before, after) || !wantRecover && len(after) != 0 {
				t.Fatalf("incorrect pending disposition: %v -> %v, accepted=%d", before, after, accepted)
			}
			wantCalls, wantInputs := 0, 0
			if mode == "native-lookup" || mode == "final-guard" || mode == "uncertain" {
				wantCalls++
			}
			if wantRecover {
				wantCalls++
				wantInputs++
			}
			*target = original
			Drain(bind.Pane)
			Drain(bind.Pane)
			if calls != wantCalls || accepted != wantInputs || f.pastes != 0 || f.enters != 0 ||
				len(auditRecords(t, events.AuditEventWakeAccepted)) != wantInputs {
				t.Fatalf("unexpected replay or input: calls=%d, accepted=%d, paste=%d, enter=%d", calls, accepted, f.pastes, f.enters)
			}
		})
	}
}

func TestBoundRotationBriefUsesExactProofAcrossGenericFingerprintGap(t *testing.T) {
	bind, _, f, _ := rotationBriefFixture(t)
	reads := 0
	nativeWakeIdentity = func(pane string) string {
		reads++
		if reads == 1 {
			return prod.identity(pane)
		}
		return ""
	}
	nativeWakeIO = func(string, ...string) dispatch.IO {
		return dispatch.IO{
			CaptureColor: func() string { return f.capture(bind.Pane) }, Sleep: func() {},
			SubmitNative: func(opts dispatch.Opts, _ string, guard func() string) dispatch.Result {
				if !opts.MatchesTarget(bind.Session, bind.Home, bind.Instance, bind.ClientPID) || guard() != "" {
					t.Fatal("missing generic fingerprint overrode exact bound proof")
				}
				return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
					Request: "receipt", Session: bind.Session, Home: bind.Home, Turn: "turn",
				}}
			},
		}
	}
	queueBoundBrief(t, bind)
	Drain(bind.Pane)
	Drain(bind.Pane)
	if got := len(auditRecords(t, events.AuditEventWakeAccepted)); got != 1 || f.pastes != 0 || f.enters != 0 {
		t.Fatalf("expected one exact native input with no terminal keys, got %d/%d/%d", got, f.pastes, f.enters)
	}
}
