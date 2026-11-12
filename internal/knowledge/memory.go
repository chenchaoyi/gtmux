package knowledge

// MemoryRule is optional, reviewed retrieval metadata. Legacy entries remain
// readable without it. The ledger still owns both prose and metadata; there is
// no second mutable knowledge store or automatic verification timestamp.
type MemoryRule struct {
	Actions         []string          `json:"actions,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Exceptions      []string          `json:"exceptions,omitempty"`
	Scope           string            `json:"scope,omitempty"` // local, project, hq, global
	Project         string            `json:"project,omitempty"`
	ToolVersions    map[string]string `json:"tool_versions,omitempty"` // exact versions, not guessed ranges
	Evidence        []string          `json:"evidence,omitempty"`
	Confidence      string            `json:"confidence,omitempty"` // reported, supported, tentative, contested
	VerifiedAt      int64             `json:"verified_at,omitempty"`
	ReviewAfter     int64             `json:"review_after,omitempty"`
	ProtectedReason string            `json:"protected_reason,omitempty"`
}
