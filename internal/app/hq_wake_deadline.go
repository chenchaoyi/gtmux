package app

import (
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

var workspaceAttentionNotice = hq.WatchWorkspaceAttention

const workspaceAttentionGrace = 60

// A check time only schedules a recheck. An explicit question or a blocked
// attempt for the exact current intent is evidence for an independent notice.
func workspaceAttention(data hqworkspace.Database, now int64) {
	var alerts []hq.WorkspaceAttention
	for _, matter := range data.Matters {
		if matter.State == "needs-input" && matter.UserInput != nil &&
			hqworkspace.GoalResponsibility(data, matter, now).State == "waiting-user" {
			alerts = append(alerts, hq.WorkspaceAttention{
				MatterID: matter.ID, EvidenceID: matter.UserInput.ID, NeedsUser: true,
			})
			continue
		}
		owner := matter.Responsibility
		if owner == nil || owner.Intent == nil {
			continue
		}
		attempt := data.IntentAttempts[owner.Intent.ID]
		if attempt.MatterID != matter.ID || attempt.StepID != owner.StepID || attempt.Intent != *owner.Intent ||
			(attempt.State != "held" && attempt.State != "uncertain") || attempt.At <= 0 ||
			now-attempt.At < workspaceAttentionGrace {
			continue
		}
		goal := hqworkspace.GoalResponsibility(data, matter, now)
		if goal.State == "verify" || goal.State == "owes-hq" {
			alerts = append(alerts, hq.WorkspaceAttention{
				MatterID: matter.ID, EvidenceID: owner.Intent.ID + "@" + attempt.State,
			})
		}
	}
	workspaceAttentionNotice(now, alerts)
}
