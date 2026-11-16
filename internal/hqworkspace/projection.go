package hqworkspace

// MatterProjection keeps current assessment, latest source and next responsibility
// separate. An unassessed receipt never overwrites an assessment or a question.
type MatterProjection struct {
	Assessment         *OverviewPreview `json:"assessment,omitempty"`
	LatestReceipt      *OverviewPreview `json:"latest_receipt,omitempty"`
	NextResponsibility GoalStatus       `json:"next_responsibility"`
}

func ProjectMatter(d Database, m Matter, now int64) MatterProjection {
	out := MatterProjection{NextResponsibility: GoalResponsibility(d, m, now)}
	if m.Assessment != "" {
		body, clipped := overviewExcerpt(m.Assessment, 768)
		out.Assessment = &OverviewPreview{Kind: "assessment", Body: body, At: m.AssessmentAt, Revision: m.AssessmentRevision, Truncated: clipped}
	}
	var latest Observation
	for _, q := range d.Reconciliations {
		o := q.Observation
		if o.Kind == "goal-followup" || CanonicalMatter(d, o.MatterID) != m.ID {
			continue
		}
		if o.OccurredAt > latest.OccurredAt || o.OccurredAt == latest.OccurredAt && o.ID > latest.ID {
			latest = o
		}
	}
	if latest.ID != "" {
		body, clipped := overviewExcerpt(latest.Body, 768)
		out.LatestReceipt = &OverviewPreview{MessageID: latest.ID, SourceRef: latest.Ref, Kind: latest.Kind, Body: body, At: latest.OccurredAt, Revision: latest.MatterRevision, Truncated: clipped}
	}
	return out
}
