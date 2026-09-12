package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Distribution (phase 3): every write lands inside gtmux's own block, a hand-edited
// block is refused, a missing file is created, and each audience's exit does what the
// brief says. Synthetic ledgers and synthetic instruction files in a temp HOME only.

func promoted(id, title, audience, repo string) knowledgeOp {
	return knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: id, Topic: "pitfalls", Title: title, Body: "Because reasons.\nSecond line.",
		At: 5, Seq: 9, Kind: KindPitfalls, Provenance: ProvSelf, Hits: 1}
}

func withPromotion(id, audience, repo string) []knowledgeOp {
	return []knowledgeOp{{V: 2, Op: knowledgeOpPromote, ID: id, At: 6, Seq: 10, Why: "w", Audience: audience, AudienceRepo: repo}}
}

func TestCarriersResolveHomeAndEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")
	t.Setenv("KIMI_CODE_HOME", "")
	byAgent := map[string]string{}
	for _, c := range Carriers() {
		byAgent[c.Agent] = c.Path
	}
	home := os.Getenv("HOME")
	for agent, want := range map[string]string{
		"claude":   filepath.Join(home, ".claude", "CLAUDE.md"),
		"codex":    filepath.Join(home, ".codex", "AGENTS.md"),
		"opencode": filepath.Join(home, ".config", "opencode", "AGENTS.md"),
		"kimi":     filepath.Join(home, ".kimi-code", "AGENTS.md"),
	} {
		if byAgent[agent] != want {
			t.Errorf("%s: %s, want %s", agent, byAgent[agent], want)
		}
	}
	t.Setenv("CODEX_HOME", "/elsewhere/codex")
	for _, c := range Carriers() {
		if c.Agent == "codex" && c.Path != "/elsewhere/codex/AGENTS.md" {
			t.Errorf("CODEX_HOME must relocate the file: %s", c.Path)
		}
	}
}

func TestBlockInstallRefreshAndRefusal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "AGENTS.md")
	// Missing file: created holding only the block.
	changed, refused, err := installBlock(p, "body v1", false)
	if err != nil || !changed || refused {
		t.Fatalf("create: %v %v %v", err, changed, refused)
	}
	b, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(b), blockBeginPrefix) || !strings.Contains(string(b), "body v1") || !strings.HasSuffix(string(b), blockEnd+"\n") {
		t.Fatalf("created file must be exactly the block:\n%s", b)
	}
	// The user's own text around the block survives a refresh.
	os.WriteFile(p, []byte("# mine\n\nkeep this\n\n"+string(b)+"\nand this after\n"), 0o644)
	changed, refused, err = installBlock(p, "body v2", false)
	if err != nil || !changed || refused {
		t.Fatalf("refresh: %v %v %v", err, changed, refused)
	}
	b, _ = os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, "keep this") || !strings.Contains(s, "and this after") || !strings.Contains(s, "body v2") || strings.Contains(s, "body v1") {
		t.Fatalf("refresh must replace only the block:\n%s", s)
	}
	if strings.Count(s, blockBeginPrefix) != 1 {
		t.Fatalf("one block, not two:\n%s", s)
	}
	// In sync: no write.
	if changed, _, _ = installBlock(p, "body v2", false); changed {
		t.Fatal("an in-sync block must not be rewritten")
	}
	// Hand-edited inside the block: refused without force, overwritten with it.
	os.WriteFile(p, []byte(strings.Replace(s, "body v2", "body v2 but I changed it", 1)), 0o644)
	changed, refused, _ = installBlock(p, "body v3", false)
	if changed || !refused {
		t.Fatal("a hand-edited block must be refused")
	}
	if got := stateOf(string(mustReadFile(t, p)), "body v3"); got != SyncHandEdited {
		t.Fatalf("state = %s, want hand-edited", got)
	}
	changed, refused, _ = installBlock(p, "body v3", true)
	if !changed || refused {
		t.Fatal("--force must overwrite")
	}
	// Stale: gtmux's own older block, hash intact.
	if got := stateOf(string(mustReadFile(t, p)), "body v4"); got != SyncStale {
		t.Fatalf("state = %s, want stale", got)
	}
}

func mustReadFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMachineSyncWritesIndexNotText(t *testing.T) {
	asHQ(t)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("KIMI_CODE_HOME", "")
	writeOps(t, promoted("pitfalls/p", "never run cp in a script", AudienceMachine, ""))
	writeOps(t, withPromotion("pitfalls/p", AudienceMachine, "")...)
	rep, err := SyncMachine(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Written) != 4 || len(rep.Refused) != 0 {
		t.Fatalf("all four carriers created: %+v", rep)
	}
	for _, c := range Carriers() {
		b := mustReadFile(t, c.Path)
		s := string(b)
		if !strings.Contains(s, "[pitfalls] never run cp in a script") || !strings.Contains(s, MachinePath()) {
			t.Fatalf("%s: index line + canonical path expected:\n%s", c.Agent, s)
		}
		if strings.Contains(s, "Second line.") {
			t.Fatalf("%s: the block is an index, never the text:\n%s", c.Agent, s)
		}
	}
	machine := string(mustReadFile(t, MachinePath()))
	if !strings.Contains(machine, "Second line.") {
		t.Fatalf("the canonical file carries the text:\n%s", machine)
	}
	// Second sync: everything kept.
	rep, _ = SyncMachine(false)
	if len(rep.Kept) != 4 || len(rep.Written) != 0 {
		t.Fatalf("idempotent: %+v", rep)
	}
	sts, _ := CarrierStatuses()
	for _, s := range sts {
		if s.State != SyncInSync {
			t.Fatalf("%s: %s", s.Agent, s.State)
		}
	}
	// A hypothesis never reaches the index.
	writeOps(t, knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/h", Topic: "pitfalls", Title: "maybe", At: 7, Seq: 11, Kind: KindPitfalls, Status: StatusHypothesis, Audience: AudienceMachine})
	SyncMachine(false)
	if s := string(mustReadFile(t, Carriers()[0].Path)); strings.Contains(s, "maybe") {
		t.Fatal("a hypothesis must not be distributed")
	}
}

func TestRepoCarrierPrefersWhatExists(t *testing.T) {
	repo := t.TempDir()
	if got := RepoCarrierPath(repo); got != filepath.Join(repo, "AGENTS.md") {
		t.Fatalf("empty repo → AGENTS.md: %s", got)
	}
	os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte("# x\n"), 0o644)
	if got := RepoCarrierPath(repo); got != filepath.Join(repo, "CLAUDE.md") {
		t.Fatalf("only CLAUDE.md → CLAUDE.md: %s", got)
	}
	os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("# y\n"), 0o644)
	if got := RepoCarrierPath(repo); got != filepath.Join(repo, "AGENTS.md") {
		t.Fatalf("both → AGENTS.md: %s", got)
	}
}

func TestLandCarriesByAudience(t *testing.T) {
	asHQ(t)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("KIMI_CODE_HOME", "")
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte("# repo rules\n"), 0o644)

	for _, tc := range []struct{ id, title, aud, repo string }{
		{"pitfalls/for-hq", "for hq", AudienceHQ, ""},
		{"pitfalls/for-machine", "for machine", AudienceMachine, ""},
		{"pitfalls/for-repo", "for repo", AudienceRepo, repo},
		{"pitfalls/for-everyone", "for everyone", AudienceEveryone, ""},
	} {
		if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", tc.title}); rc != 0 {
			t.Fatalf("add %s", tc.id)
		}
		forArg := tc.aud
		if tc.repo != "" {
			forArg = "repo:" + tc.repo
		}
		if rc := CmdKnowledge([]string{"promote", tc.id, "--why", "w", "--for", forArg}); rc != 0 {
			t.Fatalf("promote %s --for %s", tc.id, forArg)
		}
	}
	// A free-text target is refused now.
	if rc := CmdKnowledge([]string{"promote", "pitfalls/for-hq", "--why", "w", "--target", "somewhere"}); rc == 0 {
		t.Fatal("--target must be refused")
	}
	// hq → LOCAL.md, once.
	if rc := CmdKnowledge([]string{"land", "pitfalls/for-hq"}); rc != 0 {
		t.Fatal("land hq")
	}
	local := string(mustReadFile(t, LocalPath()))
	if !strings.Contains(local, "## for hq") || !strings.Contains(local, "<!-- gtmux:knowledge pitfalls/for-hq -->") {
		t.Fatalf("LOCAL.md must carry the entry:\n%s", local)
	}
	if _, err := carryIntoLocal(mustLive(t, "pitfalls/for-hq")); err != nil || strings.Count(string(mustReadFile(t, LocalPath())), "## for hq") != 1 {
		t.Fatal("carrying twice must not duplicate")
	}
	// machine → canonical + blocks.
	if rc := CmdKnowledge([]string{"land", "pitfalls/for-machine"}); rc != 0 {
		t.Fatal("land machine")
	}
	if s := string(mustReadFile(t, Carriers()[0].Path)); !strings.Contains(s, "for machine") {
		t.Fatalf("block must carry the machine entry:\n%s", s)
	}
	// repo → the repo's CLAUDE.md (it was the only file), full text, not committed.
	if rc := CmdKnowledge([]string{"land", "pitfalls/for-repo"}); rc != 0 {
		t.Fatal("land repo")
	}
	rs := string(mustReadFile(t, filepath.Join(repo, "CLAUDE.md")))
	if !strings.HasPrefix(rs, "# repo rules\n") || !strings.Contains(rs, "### for repo") || strings.Contains(rs, "for machine") {
		t.Fatalf("repo file: own text kept, only this repo's entries:\n%s", rs)
	}
	// everyone → needs the issue.
	if rc := CmdKnowledge([]string{"land", "pitfalls/for-everyone"}); rc == 0 {
		t.Fatal("everyone must need --ref")
	}
	brief := string(mustReadFile(t, promotionBriefPath(mustLive(t, "pitfalls/for-everyone"))))
	if !strings.Contains(brief, "github.com/chenchaoyi/gtmux/issues/new?title=") {
		t.Fatalf("the brief must carry the issue link:\n%s", brief)
	}
	if rc := CmdKnowledge([]string{"land", "pitfalls/for-everyone", "--ref", "https://github.com/chenchaoyi/gtmux/issues/1"}); rc != 0 {
		t.Fatal("land everyone with ref")
	}
	// Landed entries stay in the ledger with their ref.
	for id, want := range map[string]string{"pitfalls/for-hq": "LOCAL.md", "pitfalls/for-machine": MachinePath(), "pitfalls/for-repo": filepath.Join(repo, "CLAUDE.md")} {
		if e := mustLive(t, id); e.LandedRef != want {
			t.Errorf("%s landed at %q, want %q", id, e.LandedRef, want)
		}
	}
}

func TestWithdrawAndTheEveryoneExemption(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "p"}); rc != 0 {
		t.Fatal("add")
	}
	if rc := CmdKnowledge([]string{"promote", "pitfalls/p", "--why", "w", "--for", "everyone"}); rc != 0 {
		t.Fatal("promote")
	}
	// Everyone never makes the queue overdue.
	n, oldest := PendingPromotionsSummary(1<<40 + 1)
	if n != 1 || oldest != 0 {
		t.Fatalf("everyone counted but never old: n=%d oldest=%d", n, oldest)
	}
	if rc := CmdKnowledge([]string{"withdraw", "pitfalls/p", "--why", "not worth carrying"}); rc != 0 {
		t.Fatal("withdraw")
	}
	e := mustLive(t, "pitfalls/p")
	if promotionPending(e) || e.Audience != "" {
		t.Fatalf("withdraw must return it to live: %+v", e)
	}
	if _, err := os.Stat(promotionBriefPath(e)); err == nil {
		t.Fatal("the brief must be swept")
	}
	if rc := CmdKnowledge([]string{"withdraw", "pitfalls/p", "--why", "again"}); rc == 0 {
		t.Fatal("withdrawing a live entry must refuse")
	}
}
