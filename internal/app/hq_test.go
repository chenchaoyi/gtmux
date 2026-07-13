package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// Back-compat: a pre-AGENTS.md home has a FULL (possibly edited) CLAUDE.md —
// seeding must leave it untouched and only ADD the canonical AGENTS.md.
func TestSeedHQHomeBackCompat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(hqClaudePointerPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqClaudePointerPath(), []byte("FULL OLD PLAYBOOK + user notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	seeded, err := seedHQHome()
	if err != nil || !seeded {
		t.Fatalf("seed over old home = (%v, %v), want (true, nil) — AGENTS.md added", seeded, err)
	}
	if cb, _ := os.ReadFile(hqClaudePointerPath()); string(cb) != "FULL OLD PLAYBOOK + user notes" {
		t.Errorf("old CLAUDE.md must never be clobbered into a pointer: %q", cb)
	}
	if b, _ := os.ReadFile(hqInstructionsPath()); !strings.Contains(string(b), "gtmux digest --json") {
		t.Errorf("AGENTS.md should be added alongside: %q", b)
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
	if err != nil || !strings.Contains(string(env), "TUN") {
		t.Errorf("environment.md should note Clash TUN mode: %v", err)
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
