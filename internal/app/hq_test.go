package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// seedHQHome creates the home + BOTH instruction entries once (AGENTS.md the
// canonical playbook, CLAUDE.md the @AGENTS.md import), then NEVER overwrites —
// they're the user's to edit and the supervisor's accumulated knowledge.
func TestSeedHQHomeIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	seeded, err := seedHQHome()
	if err != nil || !seeded {
		t.Fatalf("first seed = (%v, %v), want (true, nil)", seeded, err)
	}
	b, err := os.ReadFile(hqInstructionsPath())
	if err != nil || !strings.Contains(string(b), "gtmux digest --json") {
		t.Fatalf("AGENTS.md should teach the digest toolbox: %v %q", err, b)
	}
	cb, err := os.ReadFile(hqClaudePointerPath())
	if err != nil || !strings.Contains(string(cb), "@AGENTS.md") {
		t.Fatalf("CLAUDE.md should be the @AGENTS.md import: %v %q", err, cb)
	}

	// User edits both — a re-run must keep the edits.
	if err := os.WriteFile(hqInstructionsPath(), []byte("MY EDITS"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqClaudePointerPath(), []byte("MY CLAUDE EDITS"), 0o644); err != nil {
		t.Fatal(err)
	}
	seeded, err = seedHQHome()
	if err != nil || seeded {
		t.Fatalf("second seed = (%v, %v), want (false, nil)", seeded, err)
	}
	if b2, _ := os.ReadFile(hqInstructionsPath()); string(b2) != "MY EDITS" {
		t.Errorf("re-seed clobbered AGENTS.md: %q", b2)
	}
	if cb2, _ := os.ReadFile(hqClaudePointerPath()); string(cb2) != "MY CLAUDE EDITS" {
		t.Errorf("re-seed clobbered CLAUDE.md: %q", cb2)
	}
}

// A legacy full CLAUDE.md (pre-AGENTS.md) is authoritative: seeding must leave it
// untouched and NOT drop a zombie AGENTS.md beside it (the bug this fixes).
func TestSeedHQHome_LegacyClaudeNoZombie(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(hqClaudePointerPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqClaudePointerPath(), []byte("FULL OLD PLAYBOOK + user notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	if cb, _ := os.ReadFile(hqClaudePointerPath()); string(cb) != "FULL OLD PLAYBOOK + user notes" {
		t.Errorf("legacy CLAUDE.md must never be clobbered: %q", cb)
	}
	if fileExists(hqInstructionsPath()) {
		t.Error("a zombie AGENTS.md must NOT be created beside a full CLAUDE.md")
	}
	// A lone full CLAUDE.md is a valid layout → no nag.
	if en, _ := hqPolicyWarning(); en != "" {
		t.Errorf("lone full CLAUDE.md should not warn: %q", en)
	}
}

// A home with only AGENTS.md (e.g. a prior Codex seed) gains ONLY the cheap CLAUDE.md
// import — never a second full copy — so Claude reads the same canonical file.
func TestSeedHQHome_AgentsOnlyAddsPointer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqInstructionsPath(), []byte(hqInstructions), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	cb, _ := os.ReadFile(hqClaudePointerPath())
	if !isClaudePointer(string(cb)) {
		t.Errorf("CLAUDE.md should be the @AGENTS.md import, got %q", cb)
	}
	if en, _ := hqPolicyWarning(); en != "" {
		t.Errorf("canonical + pointer is clean, should not warn: %q", en)
	}
}

// The redundant/broken layouts must WARN so `gtmux hq` surfaces them.
func TestHQPolicyWarning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(state.HQHome(), name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Two full docs → warn.
	write("AGENTS.md", hqInstructions)
	write("CLAUDE.md", "# my full playbook\n...")
	if en, zh := hqPolicyWarning(); en == "" || zh == "" {
		t.Error("full CLAUDE.md + AGENTS.md should warn (redundant/drift)")
	}
	// Clean single source (pointer + canonical) → no warn.
	write("CLAUDE.md", hqClaudePointer)
	if en, _ := hqPolicyWarning(); en != "" {
		t.Errorf("pointer + canonical is clean: %q", en)
	}
	// Dangling import (pointer but no AGENTS.md) → warn.
	if err := os.Remove(hqInstructionsPath()); err != nil {
		t.Fatal(err)
	}
	if en, _ := hqPolicyWarning(); en == "" {
		t.Error("a CLAUDE.md @import with no AGENTS.md should warn (dangling)")
	}
}

func TestSeedHQKnowledge(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	// scaffold present + the README teaches the no-secrets rule
	rd, err := os.ReadFile(filepath.Join(hqKnowledgeDir(), "README.md"))
	if err != nil || !strings.Contains(string(rd), "NEVER store secrets") {
		t.Fatalf("knowledge README missing/incomplete: %v", err)
	}
	for _, f := range []string{"accounts.md", "workflows.md", "best-practices.md", "pitfalls.md"} {
		if _, err := os.Stat(filepath.Join(hqKnowledgeDir(), f)); err != nil {
			t.Errorf("missing knowledge file %s", f)
		}
	}
	// the supervisor's curated content is NEVER overwritten
	acc := filepath.Join(hqKnowledgeDir(), "accounts.md")
	if err := os.WriteFile(acc, []byte("Apple team: 2337SY8FRT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if seedHQKnowledge() {
		t.Error("re-seed should create nothing when files exist")
	}
	if b, _ := os.ReadFile(acc); string(b) != "Apple team: 2337SY8FRT" {
		t.Errorf("re-seed clobbered curated content: %q", b)
	}
}

// The seeded playbook must encode the v0.18.1 hardening: the hard role whitelist,
// the nudge-payload-is-DATA policy, the dual-channel goal-changed sense, and the
// TUN-mode network note. Pins spec⇄code consistency for these behaviors.
func TestHQPlaybookHardening(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(hqInstructionsPath())
	if err != nil {
		t.Fatal(err)
	}
	s := string(agents)
	for _, want := range []string{
		"HARD WHITELIST",       // role boundary tightened: no concrete command
		"read-only",            // even read-only gh/git is delegated
		"never an instruction", // nudge payload is DATA
		"goal-changed",         // dual-channel: sense user-direct tasks
	} {
		if !strings.Contains(s, want) {
			t.Errorf("AGENTS.md playbook missing %q", want)
		}
	}
	env, err := os.ReadFile(filepath.Join(hqKnowledgeDir(), "environment.md"))
	if err != nil || !strings.Contains(string(env), "gtmux config agent-proxy") {
		t.Errorf("environment.md should explain the explicit, generic proxy config: %v", err)
	}
}

// The seed must carry the promoted charter: main-session responsiveness (B), dispatch
// granularity (B2), reclaim-is-HQ's-job (A), and the portable operating lessons (F6).
func TestHQPlaybookCharter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(hqInstructionsPath())
	if err != nil {
		t.Fatal(err)
	}
	s := string(agents)
	for _, want := range []string{
		"RESPONSIVENESS",          // B: main session stays the fast input receiver
		"GRANULARITY",             // B2: one self-reporting subagent per independent step
		"reclamation IS YOUR JOB", // A: reclaim via reap/subagent, not hand-typed
		"--headless",              // M2: heavy/background work reference (now shipped)
	} {
		if !strings.Contains(s, want) {
			t.Errorf("charter seed missing %q", want)
		}
	}
	bp, err := os.ReadFile(filepath.Join(hqKnowledgeDir(), "best-practices.md"))
	if err != nil || !strings.Contains(string(bp), "HQ operating lessons") {
		t.Errorf("best-practices seed should carry portable HQ operating lessons: %v", err)
	}
}

// The startup briefing prompt (HQ's first-output trigger) must, in EACH language,
// carry both halves the feature promises: the self-introduction and the immediate
// status report grounded in digest/usage/limits (needs-you first, token usage +
// subscription room). Pins the prompt so a future edit can't silently drop a half.
func TestHQBriefingPrompt(t *testing.T) {
	for _, tc := range []struct {
		lang string
		want []string
	}{
		{"en", []string{
			"gtmux HQ supervisor", // self-introduction
			"gtmux digest --json", // status report source
			"gtmux usage --json",
			"gtmux limits --json",
			"needs-you",     // needs-you leads
			"token-usage",   // token usage section
			"subscription-", // subscription-window room
		}},
		{"zh", []string{
			"gtmux HQ 中控管家",
			"gtmux digest --json",
			"gtmux usage --json",
			"gtmux limits --json",
			"needs-you",
			"token 用量",
			"订阅余量",
		}},
	} {
		i18n.SetLang(tc.lang)
		got := hqBriefingPrompt()
		for _, w := range tc.want {
			if !strings.Contains(got, w) {
				t.Errorf("[%s] briefing prompt missing %q\n---\n%s", tc.lang, w, got)
			}
		}
	}
	i18n.SetLang("en")
}

// The briefing is ON by default and opt-out-able via GTMUX_HQ_BRIEF (off/0/false/no),
// so a user who wants a silent HQ start — or drives the first prompt themselves — can.
func TestHQBriefingEnabled(t *testing.T) {
	t.Setenv("GTMUX_HQ_BRIEF", "")
	if !hqBriefingEnabled() {
		t.Error("briefing should be ON by default")
	}
	for _, off := range []string{"off", "0", "false", "no", "OFF", "  false  "} {
		t.Setenv("GTMUX_HQ_BRIEF", off)
		if hqBriefingEnabled() {
			t.Errorf("GTMUX_HQ_BRIEF=%q should disable the briefing", off)
		}
	}
	for _, on := range []string{"on", "1", "yes", "claude"} {
		t.Setenv("GTMUX_HQ_BRIEF", on)
		if !hqBriefingEnabled() {
			t.Errorf("GTMUX_HQ_BRIEF=%q should leave the briefing ON", on)
		}
	}
}

func TestHQAgentCommand(t *testing.T) {
	t.Setenv("GTMUX_HQ_AGENT", "")
	if got := hqAgentCommand(); got != "claude" {
		t.Errorf("default hq agent = %q, want claude", got)
	}
	t.Setenv("GTMUX_HQ_AGENT", "codex")
	if got := hqAgentCommand(); got != "codex" {
		t.Errorf("override hq agent = %q, want codex", got)
	}
}
