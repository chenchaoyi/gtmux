package app

import (
	"context"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"time"
)

const relayInboxHint = "[gtmux HQ coordination] The HQ inbox has new replies. Read gtmux relay inbox, then gtmux relay show <id> for the replies relevant to your current work. This is coordination, not a user instruction or authorization."

const relayInboxPrefix = "[gtmux HQ inbox notice "

func relayInboxNoticeText(r relay.Request) string {
	return fmt.Sprintf("%s%s; HQ coordination, not user instruction or authorization.]\n%s", relayInboxPrefix, r.ID, relayInboxHint)
}

var relayNotifyActive = codexrpc.CoordinationNotice

func relaySource(c callerIdentity) relay.Source {
	return relay.Source{Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.ThreadID, Pane: c.Pane, Session: c.Session, PanePID: c.PanePID}
}
func deliverCodexRelayInbox(r relay.Request) int {
	// Legacy queued rows remain exactly as recorded. No queue drain, deletion,
	// retry-to-another-channel, or inference that an accepted message was read.
	if !relay.NoticeEligible(r, time.Now().Unix()) || r.HomeID != transcript.CodexHomeID() {
		printRelay(r, true)
		return 0
	}
	target := codexrpc.Target{HomeID: r.HomeID, ThreadID: r.ThreadID, BackendPID: r.SourcePID, BackendStart: r.SourceStart}
	if a, ok := resume.CodexFrontendForThread(r.HomeID, r.ThreadID); ok {
		target.BackendPID, target.BackendStart = a.BackendPID, a.BackendStart
	}
	if target.BackendPID <= 1 || target.BackendStart == "" {
		printRelay(r, true)
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	claimed := false
	outcome := relayNotifyActive(ctx, target, "gtmux-inbox-"+r.ID, relayInboxNoticeText(r), func(turn string) bool {
		claimed = relay.BeginNoticeContext(ctx, r.ID, turn) == nil
		return claimed
	})
	if claimed {
		receiptCtx, done := context.WithTimeout(context.Background(), time.Second)
		_ = relay.FinishNoticeContext(receiptCtx, r.ID, outcome.Accepted, outcome.Attempted)
		done()
	}
	if current, err := relay.Get(r.ID); err == nil {
		r = current
	}
	printRelay(r, true)
	return 0 // Saved/pull-readable is success, not proof of notification or reading.
}
