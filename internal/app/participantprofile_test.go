package app

import (
	"github.com/chenchaoyi/gtmux/internal/agentprofile"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"testing"
)

func TestParticipantProfileBindsHistoricalExternalConversation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	const at = "2026-10-09T00:00:01Z"
	desktopRollout(t, root, "profile-desk", "01", "codex_work_desktop",
		`{"timestamp":"2026-10-09T00:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"Inspect"}}`,
		`{"timestamp":"2026-10-09T00:00:02Z","type":"turn_context","payload":{"model":"old-model","effort":"medium"}}`,
		`{"timestamp":"2026-10-09T00:00:03Z","type":"event_msg","payload":{"type":"task_complete"}}`,
		`{"timestamp":"2026-10-09T00:00:04Z","type":"turn_context","payload":{"model":"later-model","effort":"high"}}`)
	value, err := participantProfile("", "profile-desk", "", at)
	if err != nil {
		t.Fatal(err)
	}
	p := value.(agentprofile.Profile)
	facts := map[string]string{}
	for _, f := range p.Facts {
		facts[f.Key] = f.Value
	}
	if p.SessionID != "profile-desk" || !p.Historical || facts["model"] != "old-model" || facts["effort"] != "medium" || len(p.Chain) != 0 {
		t.Fatalf("wrong message evidence: %+v", p)
	}
	if policy := sessionpolicy.Get("profile-desk"); policy.Revision != 0 || policy.HQ || policy.Knowledge || policy.Notify {
		t.Fatalf("profile read granted authority: %+v", policy)
	}
	for _, tc := range []struct{ sid, home, at string }{
		{"unknown", "", at}, {"profile-desk", "", "2026-10-09T00:00:09Z"},
		{"profile-desk", "", "1969-01-01T00:00:00Z"}, {"profile-desk", "wrong-home", at},
	} {
		if _, err := participantProfile("", tc.sid, tc.home, tc.at); err == nil {
			t.Fatalf("unverified identity accepted: %+v", tc)
		}
	}
}

func TestProfileTurnIntervalRejectsAmbiguity(t *testing.T) {
	const at = "2026-10-09T00:00:01Z"
	turns := []transcript.Turn{{Time: at}, {Time: at}}
	if _, _, err := profileTurnInterval(turns, at); err == nil {
		t.Fatal("ambiguous timestamp accepted")
	}
}
