package hq

import (
	"errors"
	"reflect"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

func TestHQLaunchBinary(t *testing.T) {
	cases := map[string]string{
		"claude":   "claude",       // Resume[0]
		"codex":    "codex",        // Resume[0]
		"cursor":   "cursor-agent", // Resume[0] is the binary, not the key
		"opencode": "opencode",
	}
	for key, want := range cases {
		m, ok := agents.For(key)
		if !ok {
			t.Fatalf("registry missing %q", key)
		}
		if got := hqLaunchBinary(m); got != want {
			t.Errorf("hqLaunchBinary(%s)=%q, want %q", key, got, want)
		}
	}
}

// fakeLookPath resolves only the binaries in `have`.
func fakeLookPath(have ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	return func(bin string) (string, error) {
		if set[bin] {
			return "/usr/local/bin/" + bin, nil
		}
		return "", errors.New("not found")
	}
}

func candCmds(cands []hqAgentCandidate) []string {
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.cmd
	}
	return out
}

func TestDetectHQAgents_OnlyInstalledHooked(t *testing.T) {
	// Only claude + codex installed → exactly those, hook-equipped, in registry order.
	got := candCmds(detectHQAgents(fakeLookPath("claude", "codex")))
	if want := []string{"claude", "codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("detect(claude,codex)=%v, want %v", got, want)
	}
	// Only codex installed (the reported case: no Claude login/binary) → just codex.
	got = candCmds(detectHQAgents(fakeLookPath("codex")))
	if want := []string{"codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("detect(codex)=%v, want %v", got, want)
	}
	// Nothing installed → empty (caller falls back to the default).
	if got := detectHQAgents(fakeLookPath()); len(got) != 0 {
		t.Fatalf("detect(none)=%v, want empty", got)
	}
	// A non-hook-equipped agent (grok has a Resume but Hooked=false) is NOT offered even
	// if installed — HQ needs the wake/event stream.
	if got := candCmds(detectHQAgents(fakeLookPath("grok"))); len(got) != 0 {
		t.Fatalf("detect(grok, non-hooked)=%v, want empty", got)
	}
}

func TestHQAgentDefaultIndex(t *testing.T) {
	claudeFirst := []hqAgentCandidate{{key: "claude"}, {key: "codex"}}
	if got := hqAgentDefaultIndex(claudeFirst); got != 0 {
		t.Errorf("claude present → default index 0, got %d", got)
	}
	codexFirst := []hqAgentCandidate{{key: "codex"}, {key: "claude"}}
	if got := hqAgentDefaultIndex(codexFirst); got != 1 {
		t.Errorf("claude at index 1 → default 1, got %d", got)
	}
	noClaude := []hqAgentCandidate{{key: "codex"}, {key: "gemini"}}
	if got := hqAgentDefaultIndex(noClaude); got != 0 {
		t.Errorf("no claude → default first (0), got %d", got)
	}
}

func TestChooseHQAgent(t *testing.T) {
	cands := []hqAgentCandidate{
		{key: "claude", cmd: "claude"},
		{key: "codex", cmd: "codex"},
		{key: "gemini", cmd: "gemini"},
	}
	const def = 0 // claude
	cases := map[string]string{
		"":     "claude", // empty → default
		"\n":   "claude", // just a newline → default
		"  \n": "claude", // whitespace → default
		"1":    "claude",
		"2":    "codex", // the reported fix: user picks the non-default
		"2\n":  "codex",
		" 3 ":  "gemini",
		"9":    "claude", // out of range → default
		"0":    "claude", // 0 is not a 1-based index → default
		"nope": "claude", // garbage → default
		"-1":   "claude",
	}
	for reply, want := range cases {
		if got := chooseHQAgent(cands, def, reply); got != want {
			t.Errorf("chooseHQAgent(%q)=%q, want %q", reply, got, want)
		}
	}
	// A defensive out-of-range default falls back to index 0.
	if got := chooseHQAgent(cands, 99, ""); got != "claude" {
		t.Errorf("bad default index should clamp to 0, got %q", got)
	}
	// No candidates → empty.
	if got := chooseHQAgent(nil, 0, "1"); got != "" {
		t.Errorf("no candidates → empty, got %q", got)
	}
}
