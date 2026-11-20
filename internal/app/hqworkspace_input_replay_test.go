package app

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

func TestSPL018WorkspaceEarlyHoldPreservesPriorDelivery(t *testing.T) {
	target := hqworkspace.Conversation{Agent: "codex", HomeID: "home", SessionID: "thread", Pane: "%18", Loc: "test:1.0"}
	attachment := agentclient.Attachment{
		Version: agentclient.ProtocolVersion, Agent: target.Agent, Client: "codex",
		PID: 123, Instance: strings.Repeat("a", 32), HomeID: target.HomeID, SessionID: target.SessionID,
		Pane: target.Pane, ServerEpoch: "18", BindingEpoch: 20, InputReady: true,
	}

	for _, scenario := range []struct {
		prior     string
		uncertain bool
	}{
		{prior: "fresh"},
		{prior: "unclaimed"},
		{prior: "claimed", uncertain: true},
		{prior: "accepted", uncertain: true},
		{prior: "refused"},
	} {
		t.Run(scenario.prior, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			oldCandidates, oldDispatch := workspaceGoalCandidates, workspaceDispatch
			t.Cleanup(func() { workspaceGoalCandidates, workspaceDispatch = oldCandidates, oldDispatch })
			workspaceGoalCandidates = func() []workspaceCandidate {
				return []workspaceCandidate{{Conversation: target, Status: "idle"}}
			}

			const id, text = "same-action-id", "synthetic workspace reply"
			if scenario.prior != "fresh" {
				r, err := agentclient.Prepare(attachment, text, id, false, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				if scenario.prior != "unclaimed" {
					if err := agentclient.ClaimNative(r); err != nil {
						t.Fatal(err)
					}
				}
				if scenario.prior == "accepted" || scenario.prior == "refused" {
					if err := agentclient.FinishNative(r, scenario.prior, ""); err != nil {
						t.Fatal(err)
					}
				}
			}

			for _, gate := range []struct {
				premise bool
				want    string
			}{
				{want: "stale-premise"},
				{premise: true, want: "unsupported-reply-target"},
			} {
				t.Run(gate.want, func(t *testing.T) {
					workspaceDispatch = func(string, ...string) dispatch.IO {
						io := dispatch.IO{
							Paste: func(string) error { t.Fatal("early hold pasted input"); return nil },
							Enter: func() error { t.Fatal("early hold submitted input"); return nil },
						}
						if !gate.premise {
							io.SubmitNative = func(dispatch.Opts, string, func() string) dispatch.Result {
								t.Fatal("early hold reached native submit")
								return dispatch.Result{}
							}
						}
						return io
					}

					got := deliverWorkspaceIntent(target, text, id, func() bool { return gate.premise }, true)
					if scenario.uncertain {
						if got.State != string(dispatch.StateFailed) || got.Evidence != string(dispatch.InputOutcomeUnconfirmed) || got.SafeHold || got.BeforeSubmit {
							t.Fatalf("prior native evidence became a retryable refusal: %+v", got)
						}
					} else if got.State != gate.want || !got.SafeHold || !got.BeforeSubmit {
						t.Fatalf("proven early refusal was lost: %+v", got)
					}
				})
			}
		})
	}
}
