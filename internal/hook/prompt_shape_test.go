package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Agents disagree about the TYPE of a UserPromptSubmit prompt, not just its content:
// Claude sends a string, Kimi 0.41.0 sends an array of content parts. The payload
// decode ignores its error on purpose (one unknown field must not cost the rest of the
// payload), so a mismatch is silent — on Kimi every prompt arrived with no text, which
// is no goal, no summary, and a `goal-changed` wake that cannot say what changed.
//
// Found by running a real Kimi session and teeing what it actually sent, not from its
// docs, which describe the field only as "the text submitted by the user".
func TestTextOfPromptReadsEitherShape(t *testing.T) {
	cases := []struct {
		name, raw, want string
	}{
		{"claude sends a string", `"fix the flaky test"`, "fix the flaky test"},
		{"kimi sends content parts", `[{"type":"text","text":"ship the kimi integration"}]`, "ship the kimi integration"},
		{"several parts join", `[{"type":"text","text":"one"},{"type":"text","text":"two"}]`, "one\ntwo"},
		{"non-text parts contribute nothing", `[{"type":"image_url","imageUrl":{"url":"x"}},{"type":"text","text":"look"}]`, "look"},
		{"an untyped part is taken as text", `[{"text":"bare"}]`, "bare"},
		{"absent", ``, ""},
		{"null", `null`, ""},
		{"empty array", `[]`, ""},
		{"a shape nobody sends", `{"unexpected":1}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := textOfPrompt(json.RawMessage(c.raw)); got != c.want {
				t.Errorf("textOfPrompt(%s) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

// The whole point is the CALL SITE: a prompt that decodes to nothing is silent, so
// assert the goal marker the hook writes rather than the helper alone.
func TestKimiPromptReachesTheGoalMarker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TMUX_PANE", "%77")
	payload := `{"hook_event_name":"UserPromptSubmit","session_id":"session_x","cwd":"/proj",` +
		`"prompt":[{"type":"text","text":"ship the kimi integration"}],"is_steer":false}`
	Run(strings.NewReader(payload), []string{"--agent", "kimi", "UserPromptSubmit"})

	b, err := os.ReadFile(filepath.Join(home, ".local", "share", "gtmux", "goal", "%77"))
	if err != nil {
		t.Fatalf("no goal marker written: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "ship the kimi integration" {
		t.Errorf("goal marker = %q, want the prompt text — Kimi's array-shaped prompt did not reach it", got)
	}
}
