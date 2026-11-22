package app

import (
	"errors"
	"os"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/notify"
)

var workspaceAttentionNotice = hq.WatchWorkspaceAttention

const workspaceAttentionGrace = 60

func workspaceForReconciliation(store *hqworkspace.Store, now int64) (*hqworkspace.Store, error) {
	existing := store.ExistingOnly()
	snapshot, err := existing.Read()
	if err == nil {
		workspaceAttention(snapshot, now)
		return existing, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// A new workspace can bootstrap. If existing desktop attention still
	// refers to the lost source, wait for recovery rather than recreating it.
	for _, scope := range []string{"workspace", "relay"} {
		active, stateErr := notify.HasActiveAttention(scope)
		if stateErr != nil {
			return nil, stateErr
		}
		if active {
			return nil, err
		}
	}
	return store, nil
}

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
