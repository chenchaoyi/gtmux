package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// The seeded playbook teaches BOTH disciplines folded into v7 (hq-knowledge-
// distillation): the distill ritual (a `[CONTROL gtmux:distill]` record + the triggered
// Iterate clause) AND the perception self-heal discipline (verify-by-pull before
// nagging, restart only via a dispatched worker). Pins the PROMPT half + the single bump.
func TestPlaybookTeachesDistill(t *testing.T) {
	pb := hqInstructions
	for _, want := range []string{
		"[CONTROL gtmux:distill]", "DISTILL:", "Iterate (now TRIGGERED", // distill
		"PERCEPTION SELF-HEAL DISCIPLINE", "VERIFY BY PULL", // perception self-heal
	} {
		if !strings.Contains(pb, want) {
			t.Errorf("seeded playbook must teach both folded disciplines; missing %q", want)
		}
	}
	// The seed version bump is single-sourced here; the code-only disk/feed work adds none.
	if hqPlaybookVersion < 7 {
		t.Errorf("hqPlaybookVersion = %d, want ≥ 7", hqPlaybookVersion)
	}
}

// v8 (hq-capture-loop) welds CAPTURE into the loop as a first-class step: the mandatory
// capture verdict scoped to correction/crash/recurrence, the opportunistic-silent
// done/resolved default, the `⟣ 📓` register glyph, consult as a hard precondition, and
// the board-vs-KB weld. Pins the PROMPT half + the bump so existing homes adopt it.
func TestPlaybookTeachesCaptureLoop(t *testing.T) {
	pb := hqInstructions
	for _, want := range []string{
		"SENSE → JUDGE → CAPTURE? → REPORT", // the loop shape
		"CAPTURE?",                         // the mandatory-verdict step
		"⟣ 📓 captured:",                    // the capture register glyph
		"recurrence",                       // the third forced closure class
		"OPPORTUNISTIC and SILENT",         // done/resolved default
		"Consult (a HARD PRECONDITION",     // consult hardened
		"BOARD vs KNOWLEDGE BASE — welded", // the definition weld
		"\"I noted the board\" can NEVER",  // the anti-confusion clause
	} {
		if !strings.Contains(pb, want) {
			t.Errorf("v8 playbook must teach the capture-loop; missing %q", want)
		}
	}
	if hqPlaybookVersion < 8 {
		t.Errorf("hqPlaybookVersion = %d, want ≥ 8 (hq-capture-loop)", hqPlaybookVersion)
	}
}

// A fresh seed writes a VERSIONED, managed AGENTS.md (the marker + playbook + LOCAL
// import), the CLAUDE.md import, and a seed-once LOCAL.md; a re-run at the SAME
// version is idempotent (no rewrite, no backup) — versioned-hq-playbook.
func TestSeedHQHomeIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	res, err := seedHQHome()
	if err != nil || !res.Seeded {
		t.Fatalf("first seed = (%+v, %v), want Seeded", res, err)
	}
	b, err := os.ReadFile(hqInstructionsPath())
	if err != nil || !strings.Contains(string(b), "gtmux digest --json") {
		t.Fatalf("AGENTS.md should teach the digest toolbox: %v %q", err, b)
	}
	if v := parsePlaybookVersion(string(b)); v != hqPlaybookVersion {
		t.Fatalf("fresh AGENTS.md version = %d, want %d", v, hqPlaybookVersion)
	}
	if !strings.Contains(string(b), "@LOCAL.md") {
		t.Error("managed AGENTS.md should import @LOCAL.md")
	}
	cb, err := os.ReadFile(hqClaudePointerPath())
	if err != nil || !strings.Contains(string(cb), "@AGENTS.md") {
		t.Fatalf("CLAUDE.md should be the @AGENTS.md import: %v %q", err, cb)
	}
	if !fileExists(hqLocalPath()) {
		t.Fatal("LOCAL.md should be seeded")
	}

	// A re-run at the same version changes nothing and reports no upgrade.
	res, err = seedHQHome()
	if err != nil {
		t.Fatal(err)
	}
	if res.Upgraded || res.Seeded {
		t.Errorf("re-seed at same version should be a no-op, got %+v", res)
	}
	if _, err := os.Stat(hqInstructionsPath() + ".bak-v" + itoa(hqPlaybookVersion)); err == nil {
		t.Error("an idempotent re-seed must not create a backup")
	}
}

// itoa is a tiny helper so the test doesn't import strconv just for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// A newer shipped version UPGRADES an installed playbook: the prior is backed up, the
// file is regenerated at the new version, and LOCAL.md is left untouched.
func TestSeedHQHome_UpgradesOnNewerVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	// Simulate an OLDER installed playbook (version 0 — e.g. a legacy hand-edited one)
	// carrying user content, plus a personalized LOCAL.md.
	if err := os.WriteFile(hqInstructionsPath(), []byte("OLD UNVERSIONED PLAYBOOK + my edits"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqLocalPath(), []byte("MY LOCAL OVERRIDES"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := seedHQHome()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Upgraded || !res.Migrated || res.FromVersion != 0 || res.ToVersion != hqPlaybookVersion {
		t.Fatalf("expected a v0→v%d migration, got %+v", hqPlaybookVersion, res)
	}
	// The prior content is backed up, not destroyed.
	bak, err := os.ReadFile(hqInstructionsPath() + ".bak-v0")
	if err != nil || string(bak) != "OLD UNVERSIONED PLAYBOOK + my edits" {
		t.Fatalf("prior playbook must be backed up verbatim: %v %q", err, bak)
	}
	// AGENTS.md is regenerated at the shipped version.
	if b, _ := os.ReadFile(hqInstructionsPath()); parsePlaybookVersion(string(b)) != hqPlaybookVersion {
		t.Errorf("AGENTS.md not regenerated to v%d", hqPlaybookVersion)
	}
	// LOCAL.md (the user's personalization) is NEVER overwritten.
	if lb, _ := os.ReadFile(hqLocalPath()); string(lb) != "MY LOCAL OVERRIDES" {
		t.Errorf("LOCAL.md must never be overwritten, got %q", lb)
	}
}

// An upgrade must NOT touch the situation board or the knowledge base.
func TestSeedHQHome_UpgradeLeavesMemoryUntouched(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	board := filepath.Join(hqNotesDir(), "board.md")
	if err := os.WriteFile(board, []byte("MY CURATED BOARD"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Force an upgrade by rolling the installed version back.
	if err := os.WriteFile(hqInstructionsPath(), []byte("unversioned"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	if bb, _ := os.ReadFile(board); string(bb) != "MY CURATED BOARD" {
		t.Errorf("upgrade clobbered the situation board: %q", bb)
	}
}

func TestParsePlaybookVersion(t *testing.T) {
	cases := map[string]int{
		playbookMarker(3) + "\n\nbody":      3,
		playbookMarker(1):                   1,
		"no marker here":                    0,
		"<!-- gtmux-hq-playbook vX -->":     0, // malformed → 0
		"prefix gtmux-hq-playbook v12 rest": 12,
	}
	for body, want := range cases {
		if got := parsePlaybookVersion(body); got != want {
			t.Errorf("parsePlaybookVersion(%q) = %d, want %d", body, got, want)
		}
	}
}

// A legacy home (full playbook in CLAUDE.md, no AGENTS.md) is MIGRATED
// (hq-perception-v2): backed up — never destroyed — then regenerated as the
// managed layout, so a live HQ brain can no longer run stale policy forever.
func TestSeedHQHome_LegacyClaudeMigrates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(hqClaudePointerPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqClaudePointerPath(), []byte("FULL OLD PLAYBOOK + user notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := seedHQHome()
	if err != nil {
		t.Fatal(err)
	}
	if !r.Migrated || r.ToVersion != hqPlaybookVersion || r.BackupPath == "" {
		t.Fatalf("legacy home must report a migration: %+v", r)
	}
	// The old content is preserved in the named backup…
	if bb, _ := os.ReadFile(r.BackupPath); string(bb) != "FULL OLD PLAYBOOK + user notes" {
		t.Errorf("backup must preserve the legacy playbook verbatim: %q", bb)
	}
	// …and the home is now the managed layout: versioned AGENTS.md + pointer + LOCAL.md.
	ab, _ := os.ReadFile(hqInstructionsPath())
	if parsePlaybookVersion(string(ab)) != hqPlaybookVersion {
		t.Error("migrated AGENTS.md must carry the shipped version marker")
	}
	if cb, _ := os.ReadFile(hqClaudePointerPath()); string(cb) != hqClaudePointer {
		t.Errorf("CLAUDE.md must become the @AGENTS.md pointer: %q", cb)
	}
	if !fileExists(hqLocalPath()) {
		t.Error("migration must seed LOCAL.md for the user's personalization")
	}
	// Idempotent: a second run neither re-migrates nor touches the backup.
	r2, err := seedHQHome()
	if err != nil {
		t.Fatal(err)
	}
	if r2.Migrated || r2.Upgraded {
		t.Fatalf("second run must be a no-op: %+v", r2)
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
	for _, f := range []string{"accounts.md", "workflows.md", "best-practices.md", "pitfalls.md", "corrections.md"} {
		if _, err := os.Stat(filepath.Join(hqKnowledgeDir(), f)); err != nil {
			t.Errorf("missing knowledge file %s", f)
		}
	}
	// the README lists the corrections topic (the learning-loop landing place)
	if !strings.Contains(string(rd), "corrections.md") {
		t.Error("knowledge README should list corrections.md")
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
		"COLUMN-ALIGNED TABLE",    // status reports are a scannable table, not prose
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

// The startup briefing is SPLIT: hqBriefingPrompt() is a MINIMAL, agent-agnostic
// TRIGGER, and the briefing's content + format live in the seeded playbook (AGENTS.md
// "## First turn"). Pin both so a future edit can't (a) bloat the trigger back into a
// fragile multi-line paste, or (b) drop the briefing content from the playbook.
func TestHQBriefingPrompt(t *testing.T) {
	// The trigger stays a short ONE-LINER that names the startup signal + points at the
	// playbook — in each language — never the old format spec. (Short = submits reliably.)
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		got := hqBriefingPrompt()
		if strings.Contains(got, "\n") {
			t.Errorf("[%s] briefing trigger must be a single line: %q", lang, got)
		}
		for _, w := range []string{"» gtmux·startup", "AGENTS.md"} {
			if !strings.Contains(got, w) {
				t.Errorf("[%s] briefing trigger missing %q: %q", lang, w, got)
			}
		}
	}
	i18n.SetLang("en")

	// The briefing CONTENT (both halves + the report format) must live in the playbook,
	// so any agent gets it from its own convention file — not from a big injected prompt.
	for _, w := range []string{
		"## First turn", "» gtmux·startup", "STARTUP BRIEFING", // the first-turn section
		"gtmux HQ supervisor",                                              // self-introduction
		"gtmux digest --json", "gtmux usage --json", "gtmux limits --json", // report sources
		"Policy #1",            // references the existing format spec (not duplicated)
		"COLUMN-ALIGNED TABLE", // that format lives in Policy #1
		"needs-you",
	} {
		if !strings.Contains(hqInstructions, w) {
			t.Errorf("playbook (hqInstructions) missing briefing content %q", w)
		}
	}
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

// The situation board (HQ's durable posture) is seeded write-when-absent and never
// clobbered on re-seed — the same discipline as the knowledge scaffold.
func TestSeedHQNotesBoard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := seedHQHome(); err != nil {
		t.Fatal(err)
	}
	board := filepath.Join(hqNotesDir(), "board.md")
	b, err := os.ReadFile(board)
	if err != nil || !strings.Contains(string(b), "situation board") {
		t.Fatalf("board.md missing/incomplete: %v %q", err, b)
	}
	// curated content is never overwritten
	if err := os.WriteFile(board, []byte("MY FLEET POSTURE"), 0o644); err != nil {
		t.Fatal(err)
	}
	if seedHQNotes() {
		t.Error("re-seed should create nothing when board.md exists")
	}
	if b2, _ := os.ReadFile(board); string(b2) != "MY FLEET POSTURE" {
		t.Errorf("re-seed clobbered the curated board: %q", b2)
	}
}

// The chief-of-staff upgrade must be encoded in the seed: the persistent situation
// board, the severity-filtered reads, the decision-authority tiers, graded escalation +
// reconcile, and the correction→charter learning loop. Pins spec⇄code.
func TestHQPlaybookChiefOfStaff(t *testing.T) {
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
		"Situation board",        // §1: persistent posture
		"board.md",               // the board file HQ maintains
		"--severity important",   // §1: triage the escalation subset first…
		"--severity notable",     // …but fleet changes are a stream of their own
		"DECISION AUTHORITY",     // §2: the autonomy matrix
		"REVERSIBLE",             // §2: the may-decide condition
		"ESCALATE",               // §2: the must-escalate condition
		"GRADED ESCALATION",      // §3: graded channels
		"RECONCILE before",       // §3: reconcile-before-relay
		"CRITICAL",               // §3: only critical rings
		"LEARN FROM CORRECTIONS", // §4: the learning loop
		"corrections.md",         // §4: the landing place
		"CHARTER-LEVEL",          // §4: flag charter-level lessons for a seed/spec update
	} {
		if !strings.Contains(s, want) {
			t.Errorf("chief-of-staff seed missing %q", want)
		}
	}
	// bilingual: the zh anchors are present too
	for _, wantZH := range []string{
		"态势板",       // situation board
		"授权分层",      // decision-authority tiers
		"分级升级",      // graded escalation
		"对账核销",      // reconcile
		"纠正→守则学习闭环", // learning loop
	} {
		if !strings.Contains(s, wantZH) {
			t.Errorf("chief-of-staff seed missing zh anchor %q", wantZH)
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

// The v2 seed must carry the wake protocol (hq-perception-v2): pull-on-wake with no
// background-tail requirement, the signal register, enrollment, graded done
// judgment, and the tick brief bound.
func TestHQPlaybookWakeProtocol(t *testing.T) {
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
		"» gtmux·",            // the injected signal-line language
		"WAKE → PULL → JUDGE", // the short-turn loop
		"--since-seq",         // pull-on-wake primitive
		"Signal register",     // wakes answer in a distinct register
		"⟣ ◈",                 // the tick brief glyph
		"Enrollment 建联",       // goal-aware dossiers at start + per newcomer
		"DONE JUDGMENT",       // graded done responses
		"crash",               // a dead turn is never a finish
		"don't tail",          // no background-tail requirement (agent-agnostic HQ)
	} {
		if !strings.Contains(s, want) {
			t.Errorf("AGENTS.md v2 playbook missing %q", want)
		}
	}
	// The v1 background-tail REQUIREMENT must be gone (the spool daemon may still
	// be mentioned, but never as a "run it as a background task" instruction).
	if strings.Contains(s, "run it as a BACKGROUND") {
		t.Error("v2 playbook must not require a background hq-feed tail")
	}
}

// The three reads must be named in the seed, and none of them sold as "the attention
// stream" (hq-attention-stream). This is the half of the fix that actually reaches HQ:
// raising a user instruction to `notable` changes nothing if the playbook still tells
// HQ that `--severity important` is the whole picture.
func TestHQPlaybookThreeReads(t *testing.T) {
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
		"THREE reads",          // they are not interchangeable
		"--since-seq",          // the unfiltered delta…
		"RECONCILE",            // …is what you reconcile with
		"--severity notable",   // the fleet-change stream
		"FLEET-CHANGE",         //
		"--severity important", // the escalation stream…
		"SUBSET",               // …explicitly not the whole picture
		"triage shortcut",      // the rule that generalizes past this bug
		"世界模型",                 // the zh anchor for that rule
	} {
		if !strings.Contains(s, want) {
			t.Errorf("v4 playbook missing %q", want)
		}
	}
	// The claim this change exists to kill: no filtered read is "the attention stream".
	if strings.Contains(s, "attention stream") {
		t.Error("v4 playbook must not present any filtered read as THE attention stream")
	}
}

// agentAliveByCmd pins the "is the HQ agent still alive in its pane?" decision that
// gates focus-vs-relaunch: a shell (or empty) foreground command means the supervisor
// exited (user quit it) → `gtmux hq` relaunches instead of focusing a dead prompt.
func TestAgentAliveByCmd(t *testing.T) {
	dead := []string{"", "  ", "zsh", "-zsh", "bash", "-bash", "fish", "sh", "dash"}
	for _, c := range dead {
		if agentAliveByCmd(c) {
			t.Errorf("agentAliveByCmd(%q) = true, want false (a shell/empty = agent exited)", c)
		}
	}
	alive := []string{"claude", "node", "codex", "python3", "go", "vim"}
	for _, c := range alive {
		if !agentAliveByCmd(c) {
			t.Errorf("agentAliveByCmd(%q) = false, want true (a non-shell foreground = agent running)", c)
		}
	}
}
