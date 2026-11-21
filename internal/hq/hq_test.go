package hq

import (
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// A fresh seed writes a VERSIONED, managed AGENTS.md (the marker + playbook + LOCAL
// import), the CLAUDE.md import, and a seed-once LOCAL.md; a re-run at the SAME
// version is idempotent (no rewrite, no backup) — versioned-hq-playbook.
func TestSeedHQHomeIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	res, err := seedHQHome("")
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
	res, err = seedHQHome("")
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

func TestStorageBoundaryUpgradePreservesLocalRulesAndExistingRecords(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			if err := os.MkdirAll(filepath.Join(state.HQHome(), "notes"), 0700); err != nil {
				t.Fatal(err)
			}
			prior := playbookMarker(hqPlaybookVersion-1, lang) + "\nprevious managed charter\n"
			local := []byte("personal operator rules\n")
			note := filepath.Join(state.HQHome(), "notes", "unique-existing.txt")
			for path, data := range map[string][]byte{hqInstructionsPath(): []byte(prior), hqLocalPath(): local, note: []byte("unique existing record")} {
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var result seedResult
			if err := upgradePlaybookIfNewer(&result, ""); err != nil || !result.Upgraded {
				t.Fatalf("upgrade: %+v %v", result, err)
			}
			body, err := os.ReadFile(hqInstructionsPath())
			if err != nil || parsePlaybookLang(string(body)) != lang {
				t.Fatal("upgrade lost the charter language", err)
			}
			wants := []string{"~/.config/gtmux/", "~/.local/share/gtmux/"}
			if lang == "en" {
				wants = append(wants, "project-owned storage", "task temporary storage", "evidence references", "not automatically moved or deleted")
			} else {
				wants = append(wants, "项目自有目录", "任务临时目录", "证据引用", "既有材料不自动搬移或删除")
			}
			for _, want := range wants {
				if !strings.Contains(string(body), want) {
					t.Errorf("%s: missing storage boundary %q", lang, want)
				}
			}
			for path, expected := range map[string]string{hqLocalPath(): string(local), note: "unique existing record", result.BackupPath: prior} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != expected {
					t.Errorf("upgrade changed preserved material: %v", err)
				}
			}
			var again seedResult
			if err := upgradePlaybookIfNewer(&again, ""); err != nil || again.Upgraded {
				t.Fatalf("upgrade did not converge: %+v %v", again, err)
			}
		})
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
func TestStartupPreservesIndependentRules(t *testing.T) {
	for name, body := range map[string]string{
		"unversioned":    "Unversioned rules and personal edits",
		"after-local":    playbookMarker(67, "en") + "\nold charter\n@LOCAL.md\nMy additional policy\n",
		"personal-block": playbookMarker(67, "en") + "\n<!-- user:standing -->\nMy policy\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			if err := state.WritePrivateFile(hqInstructionsPath(), []byte(body)); err != nil {
				t.Fatal(err)
			}
			result, err := seedHQHome("")
			if err != nil || !result.Preserved || result.Upgraded {
				t.Fatalf("startup should preserve the entry: %+v %v", result, err)
			}
			if got, err := os.ReadFile(hqInstructionsPath()); err != nil || string(got) != body {
				t.Fatalf("existing active entry changed: %v", err)
			}
		})
	}
}

func TestParsePlaybookVersion(t *testing.T) {
	cases := map[string]int{
		playbookMarker(3, "en") + "\n\nbody": 3,
		playbookMarker(1, "en"):              1,
		"no marker here":                     0,
		"<!-- gtmux-hq-playbook vX -->":      0, // malformed → 0
		"prefix gtmux-hq-playbook v12 rest":  12,
	}
	for body, want := range cases {
		if got := parsePlaybookVersion(body); got != want {
			t.Errorf("parsePlaybookVersion(%q) = %d, want %d", body, got, want)
		}
	}
}

func TestSeedHQHomePreservesFullLegacyClaude(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const prior = "FULL OLD PLAYBOOK + user notes"
	if err := state.WritePrivateFile(hqClaudePointerPath(), []byte(prior)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := seedHQHome("zh"); err != nil {
			t.Fatal(err)
		}
		if body, err := os.ReadFile(hqClaudePointerPath()); err != nil || string(body) != prior {
			t.Fatalf("legacy Claude rules must stay active: %v", err)
		}
		if body, err := os.ReadFile(hqInstructionsPath()); err != nil || string(body) != playbookIn("zh") {
			t.Fatalf("managed entry should be available beside legacy Claude rules: %v", err)
		}
	}
	if en, zh := hqPolicyWarning(); en == "" || zh == "" {
		t.Fatal("missing legacy layout notice")
	}
}

// A home with only AGENTS.md (e.g. a prior Codex seed) gains ONLY the cheap CLAUDE.md
// import — never a second full copy — so Claude reads the same canonical file.
func TestSeedHQHome_AgentsOnlyAddsPointer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hqInstructionsPath(), []byte(playbookIn("en")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := seedHQHome(""); err != nil {
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
	write("AGENTS.md", playbookIn("en"))
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
	if _, err := seedHQHome(""); err != nil {
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
	if _, err := seedHQHome(""); err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(hqInstructionsPath())
	if err != nil {
		t.Fatal(err)
	}
	s := string(agents)
	for _, want := range []string{
		"Delegate project investigation",
		"read-only",
		"worker reports are evidence",
		"direct user instructions",
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

func TestSeedBestPracticesIsPortable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := seedHQHome(""); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(hqKnowledgeDir(), "best-practices.md"))
	if err != nil || !strings.Contains(string(body), "keep THIS file portable") {
		t.Fatalf("best-practices seed lost its portability guidance: %v", err)
	}
}

// The startup briefing is SPLIT: hqBriefingPrompt() is a MINIMAL, agent-agnostic
// TRIGGER. The startup runbook carries the format. Pin both so a future edit can't (a) bloat the trigger back into a
// fragile multi-line paste, or (b) drop the briefing content from the playbook.
func TestHQBriefingPrompt(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
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
	if body := playbookText("en", "startup.md"); !strings.Contains(body, "needs-you") || !strings.Contains(body, "gtmux digest --json") {
		t.Fatal("startup runbook lost the briefing sources or order")
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
	if _, err := seedHQHome(""); err != nil {
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
	// gtmux is never the evidence that a supervisor is running.
	//
	// Sitting in the HQ home at a bare prompt and typing `gtmux hq` makes the pane's
	// foreground command "gtmux" — not a shell, so the old rule read it as a live agent,
	// in the very pane the person was typing in. They were told a supervisor was already
	// running and taken to their own shell, and none was started (report, 2026-09-07).
	for _, c := range []string{"gtmux", "gtmux hq", "/usr/local/bin/gtmux", "gtmux  hq --rotate"} {
		if agentAliveByCmd(c) {
			t.Errorf("agentAliveByCmd(%q) = true — that is gtmux's own process, not a supervisor", c)
		}
	}
}

// The first-person guard (hq-first-person): the seeds and playbook speak to ANY
// user — the author's stack, undefined internal markers, and biography-as-your-
// memory phrasing are banned, so a neutrality regression is a red build, not
// review archaeology.
func TestSeedsAndPlaybookCarryNoAuthorLeakage(t *testing.T) {
	banned := []string{
		"Apple developer", "Cloudflare", "Appium", "(B2)",
		"review-pr-518", "menubar-width",
		"of yours", "your own knowledge base",
		"WRITE THE CHINESE",
	}
	check := func(name, body string) {
		for _, tok := range banned {
			if strings.Contains(body, tok) {
				t.Errorf("%s contains banned author-leak token %q", name, tok)
			}
		}
	}
	for _, lang := range []string{"en", "zh"} {
		check("charter "+lang, playbookIn(lang))
		for _, name := range runbookNames {
			check(lang+"/"+name, playbookText(lang, name))
		}
	}
	for name, body := range knowledge.TopicSeeds {
		check("seed "+name, body)
	}
	for name, body := range hqNotesSeeds() {
		check("notes seed "+name, body)
	}
	prevL := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prevL) })
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		check("LOCAL.md template ("+lang+")", hqLocalTemplate())
	}
}

// The board seeds in the user's language (hq-first-person): zh gets the Chinese
// skeleton, anything else the English one, and the guidance below the fold is
// shared.
func TestBoardSeedFollowsLanguage(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
	i18n.SetLang("zh")
	zh := boardSeed()
	if !strings.Contains(zh, "## ① 现状 — 在跑的 pane") || !strings.Contains(zh, "## ② 交接记录 — 新的在最上面") {
		t.Fatalf("zh board must carry the Chinese skeleton:\n%s", zh)
	}
	i18n.SetLang("en")
	en := boardSeed()
	if !strings.Contains(en, "## ① Now — live panes") || !strings.Contains(en, "## ② Handoff log — newest first") {
		t.Fatalf("en board must carry the English skeleton:\n%s", en)
	}
	for _, b := range []string{zh, en} {
		if !strings.Contains(b, "KEEP THIS BOARD IN THE LANGUAGE IT WAS SEEDED IN") {
			t.Fatal("both seeds must carry the keep-seeded-language rule")
		}
	}
}

// The perception vocabulary retired by retire-perception-spool must not creep back
// into what HQ is taught: there is no spool daemon to tail and no feed-degraded
// class to expect — a reintroduction here would teach HQ to reason about machinery
// that no longer exists.
func TestPlaybookDropsRetiredPerceptionVocabulary(t *testing.T) {
	for _, banned := range []string{"hq-feed", "feed-degraded", "spool daemon"} {
		for _, charter := range []string{playbookIn("en"), playbookIn("zh"), playbookText("en", "startup.md"), playbookText("zh", "startup.md")} {
			if strings.Contains(charter, banned) {
				t.Errorf("a charter reintroduces retired vocabulary %q", banned)
			}
		}
		for name, seed := range knowledge.TopicSeeds {
			if strings.Contains(seed, banned) {
				t.Errorf("seed %s reintroduces retired vocabulary %q", name, banned)
			}
		}
	}
}

// The managed playbook follows the user's language (bilingual-playbook): the
// marker records which charter was installed, generatedPlaybook picks by
// i18n.Lang, and a language switch upgrades a current-version install.
func TestGeneratedPlaybookFollowsLanguage(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })

	i18n.SetLang("en")
	en := playbookIn(i18n.Lang())
	if !strings.Contains(en, "lang:en") {
		t.Error("en playbook marker must record lang:en")
	}
	if !strings.Contains(en, strings.SplitN(playbookText("en", "entry.md"), "\n", 2)[0]) {
		t.Error("en playbook must carry the English charter")
	}

	i18n.SetLang("zh")
	zh := playbookIn(i18n.Lang())
	if !strings.Contains(zh, "lang:zh") {
		t.Error("zh playbook marker must record lang:zh")
	}
	if !strings.Contains(zh, strings.SplitN(playbookText("zh", "entry.md"), "\n", 2)[0]) {
		t.Error("zh playbook must carry the Chinese charter")
	}
}

// A same-version install in the OTHER language regenerates (backup kept); a
// same-version same-language install is a no-op; a legacy marker without lang
// regenerates once.
func TestShellLanguageNeverRewritesTheCharter(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}

	i18n.SetLang("en")
	if err := os.WriteFile(hqInstructionsPath(), []byte(playbookIn(i18n.Lang())), 0o644); err != nil {
		t.Fatal(err)
	}
	// Same version, same language → nothing at all.
	if _, err := ensureRunbooks("en"); err != nil {
		t.Fatal(err)
	}
	var r seedResult
	if err := upgradePlaybookIfNewer(&r, ""); err != nil || r.Upgraded || r.LangDiffers != "" {
		t.Fatalf("same version, same language must be silent: %+v err=%v", r, err)
	}

	// A terminal set to another language must NOT translate the user's charter. This
	// used to rewrite the file, so running `gtmux hq` from a differently-configured
	// shell replaced the document the supervisor works from.
	i18n.SetLang("zh")
	r = seedResult{}
	if err := upgradePlaybookIfNewer(&r, ""); err != nil {
		t.Fatal(err)
	}
	if r.Upgraded {
		t.Error("a shell language difference rewrote the charter")
	}
	if r.LangDiffers != "en" {
		t.Errorf("the mismatch must be recorded for the notice, got %q", r.LangDiffers)
	}
	body, _ := os.ReadFile(hqInstructionsPath())
	if !strings.Contains(string(body), "lang:en") {
		t.Error("the charter language changed when nobody asked")
	}
}

// The route out: asking for it by name is what changes the charter.
func TestExplicitLangRewritesTheCharter(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	i18n.SetLang("en")
	if err := os.WriteFile(hqInstructionsPath(), []byte(playbookIn(i18n.Lang())), 0o644); err != nil {
		t.Fatal(err)
	}

	var r seedResult
	if err := upgradePlaybookIfNewer(&r, "zh"); err != nil || !r.Upgraded {
		t.Fatalf("--lang must rewrite: %+v err=%v", r, err)
	}
	if r.LangSwitch != "zh" {
		t.Errorf("LangSwitch = %q, want zh", r.LangSwitch)
	}
	if r.BackupPath == "" {
		t.Error("the replaced charter must be backed up")
	}
	body, _ := os.ReadFile(hqInstructionsPath())
	if !strings.Contains(string(body), "lang:zh") {
		t.Error("the charter should now be zh")
	}
}

// A version upgrade must keep the charter in the language it is already in — the
// upgrade is gtmux's business, the language is the user's.
func TestVersionUpgradeKeepsTheCharterLanguage(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	// An OLD zh charter, and a shell reading English.
	old := strings.Replace(playbookIn("zh"), fmt.Sprintf("v%d", hqPlaybookVersion), "v1", 1)
	if err := os.WriteFile(hqInstructionsPath(), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	i18n.SetLang("en")

	var r seedResult
	if err := upgradePlaybookIfNewer(&r, ""); err != nil || !r.Upgraded {
		t.Fatalf("an old version must upgrade: %+v err=%v", r, err)
	}
	body, _ := os.ReadFile(hqInstructionsPath())
	if !strings.Contains(string(body), "lang:zh") {
		t.Error("the version upgrade translated the charter into the shell's language")
	}
	if r.LangSwitch != "" {
		t.Errorf("no language change was asked for, but LangSwitch = %q", r.LangSwitch)
	}
	// A legacy marker with no language recorded has nothing to preserve.
	legacy := playbookMarker(hqPlaybookVersion, i18n.Lang())
	legacy = strings.Replace(legacy, " lang:"+i18n.Lang(), "", 1)
	if err := os.WriteFile(hqInstructionsPath(), []byte(legacy+"\n\nold body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = seedResult{}
	if err := upgradePlaybookIfNewer(&r, ""); err != nil || !r.Upgraded {
		t.Fatalf("a langless legacy marker must regenerate: %+v err=%v", r, err)
	}
}

// The notice for a mismatch must answer all four questions a person has, including
// the way out — a message that only reports a problem is half a message.
func TestMismatchNoticeSaysNothingChangedAndHow(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
	i18n.SetLang("en")
	out := captureStdout(t, func() { printSeedNotice(seedResult{LangDiffers: "zh"}) })
	for _, want := range []string{"zh", "en", "Nothing was changed", "gtmux hq --lang en"} {
		if !strings.Contains(out, want) {
			t.Errorf("notice %q is missing %q", out, want)
		}
	}
}

// LOCAL.md seeds in the user's language, like the board and the charter — and
// seed-once means a later language switch never rewrites an existing file.
func TestLocalTemplateFollowsLanguage(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })

	i18n.SetLang("zh")
	if !strings.Contains(hqLocalTemplate(), "你的 HQ 指令") {
		t.Error("zh LOCAL.md template must open in Chinese")
	}
	i18n.SetLang("en")
	if !strings.Contains(hqLocalTemplate(), "Your HQ instructions") {
		t.Error("en LOCAL.md template must open in English")
	}

	// Seed-once survives a language switch: an existing LOCAL.md is never rewritten.
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	if !seedHQLocal() {
		t.Fatal("first seed should create LOCAL.md")
	}
	before, _ := os.ReadFile(hqLocalPath())
	i18n.SetLang("zh")
	if seedHQLocal() {
		t.Error("an existing LOCAL.md must not be re-seeded")
	}
	after, _ := os.ReadFile(hqLocalPath())
	if string(before) != string(after) {
		t.Error("a language switch rewrote the user's LOCAL.md")
	}
}

// A same-version language switch is announced as what it is, not as "v32 → v32".
func TestExplicitSwitchNoticeSaysItWasAskedFor(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // printSeedNotice resolves the HQ home
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
	i18n.SetLang("en")
	out := captureStdout(t, func() {
		printSeedNotice(seedResult{Upgraded: true, LangSwitch: "zh", FromVersion: hqPlaybookVersion, ToVersion: hqPlaybookVersion, BackupPath: "/tmp/x"})
	})
	if !strings.Contains(out, "as you asked") || !strings.Contains(out, "zh") {
		t.Errorf("the notice must say the rewrite was requested: %q", out)
	}
}
