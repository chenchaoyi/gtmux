package transcript

import (
	"encoding/json"
	"testing"
)

func TestASA009DSHDoesNotBorrowOpenCodeIdentity(t *testing.T) {
	for _, tc := range []struct {
		provider, args string
		known          bool
	}{
		{"dsh", `{"id":"opaque-call-42"}`, false},
		{"dsh", `{"name":"design-kpis","id":"opaque-call-42"}`, false},
		{"dsh", `{"name":"design-kpis"}`, true},
		{"opencode", `{"id":"design-kpis"}`, true},
	} {
		t.Run(tc.provider+tc.args, func(t *testing.T) {
			step := providerAction(t, tc.provider, "skill", json.RawMessage(tc.args))
			encoded, _ := json.Marshal(step)
			t.Logf("ASA_DSH_FACTS=%s", encoded)
			if tc.known {
				if step.Action != "skill" || step.Detail != "design-kpis" || step.SubjectCount != 1 {
					t.Fatal("known provider identity regressed", step)
				}
			} else if step.Action != "" || step.SubjectCount != 0 {
				t.Fatal("DSH borrowed unsupported OpenCode id", step)
			}
		})
	}
}
