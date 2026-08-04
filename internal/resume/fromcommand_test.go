package resume

import "testing"

func TestFromCommand(t *testing.T) {
	cases := []struct {
		line      string
		wantAgent string
		wantID    string
	}{
		// The real line a resurrect save recorded for a live pane.
		{"claude --resume d644ae48-4379-41f7-abf9-fe4bb23627df", "claude", "d644ae48-4379-41f7-abf9-fe4bb23627df"},
		{"claude", "claude", ""},                         // started fresh — no id to read back
		{"claude --resume", "claude", ""},                // flag with nothing after it
		{"claude --resume=abc-123", "claude", "abc-123"}, // --flag=value form
		{"claude --dangerously-skip-permissions", "claude", ""},
		{"/opt/homebrew/bin/claude --resume xyz", "claude", "xyz"}, // absolute path
		{"node /usr/local/lib/node_modules/claude --resume xyz", "claude", "xyz"},
		{"HTTPS_PROXY=http://127.0.0.1:7897 claude --resume xyz", "claude", "xyz"},
		{"codex resume abc", "codex", "abc"},           // subcommand, not a flag
		{"opencode --session s-1", "opencode", "s-1"},  // a different flag entirely
		{"cursor-agent --resume c-1", "cursor", "c-1"}, // alias resolves to the canonical key
		{"bash", "", ""},
		{"vim notes.md", "", ""},
		{"", "", ""},
		{"   ", "", ""},
	}
	for _, c := range cases {
		agent, id := FromCommand(c.line)
		if agent != c.wantAgent || id != c.wantID {
			t.Errorf("FromCommand(%q) = %q/%q, want %q/%q", c.line, agent, id, c.wantAgent, c.wantID)
		}
	}
}

// FromCommand is the inverse of Command: whatever restore builds, restore must be
// able to read back out of the save that recorded it running.
func TestFromCommandRoundTripsCommand(t *testing.T) {
	for _, r := range []Record{
		{Agent: "claude", SessionID: "369b78a0-5a4a-4da7-bffa-90bfaaf374e8"},
		{Agent: "codex", SessionID: "c-1"},
		{Agent: "opencode", SessionID: "o-1"},
		{Agent: "cursor", SessionID: "cu-1"},
		{Agent: "kiro", SessionID: "k-1"},
	} {
		cmd, ok := Command(r) // no Cwd → no shell wrapper, i.e. what ps would show
		if !ok {
			t.Fatalf("Command(%+v) not ok", r)
		}
		agent, id := FromCommand(cmd)
		if agent != r.Agent || id != r.SessionID {
			t.Errorf("round trip of %q = %q/%q, want %q/%q", cmd, agent, id, r.Agent, r.SessionID)
		}
	}
}
