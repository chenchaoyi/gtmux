package app

import (
	"sort"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

var workspaceWakeDeadlineNotice = hq.WatchWakeDeadlines

const workspaceDeadlineGrace = 60

// workspaceWakeDeadlines only projects explicit clocks on durable obligations.
// The read-only notice does not mark receipt, authorize work, or settle a matter.
func workspaceWakeDeadlines(data hqworkspace.Database, now int64) {
	var overdue []hq.WakeDeadline
	for _, matter := range data.Matters {
		owner := matter.Responsibility
		if owner == nil {
			continue
		}
		var due int64
		if owner.Hold != nil && owner.Hold.Kind == "external" {
			due = owner.Hold.NextCheck
		} else if owner.Drive != nil {
			observation := data.Reconciliations[owner.Drive.ObservationID].Observation
			if observation.DriveProof != nil {
				due = observation.DriveProof.NextCheck
			}
		}
		if due <= 0 || now-due < workspaceDeadlineGrace {
			continue
		}
		goal := hqworkspace.GoalResponsibility(data, matter, now)
		if goal.State == "verify" || goal.State == "owes-hq" {
			overdue = append(overdue, hq.WakeDeadline{FollowupID: goal.FollowupID, MatterID: matter.ID, Due: due})
		}
	}
	sort.Slice(overdue, func(i, j int) bool {
		if overdue[i].Due == overdue[j].Due {
			return overdue[i].FollowupID < overdue[j].FollowupID
		}
		return overdue[i].Due < overdue[j].Due
	})
	workspaceWakeDeadlineNotice(now, overdue)
}
