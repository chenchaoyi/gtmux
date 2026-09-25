package knowledge

import (
	"strings"
	"testing"
)

// structural names the rules whose example is a title or a title-and-body pair rather than
// a passage, so the generic round trip below cannot feed it to the matcher. Each has its
// own test underneath.
var structural = map[string]bool{
	"title-echoed-in-body":         true,
	"implementation-name-as-title": true,
}

// The table is only worth having if every rule is complete, so this is the shape check:
// both languages, a tier the renderer knows, and, for a mechanical rule, an example pair
// that its own matcher agrees with. A rule whose before does not trip it is a rule that
// does not say what it claims to.
func TestEveryRuleIsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Rules() {
		if r.ID == "" || seen[r.ID] {
			t.Fatalf("rule %q: missing or duplicate id", r.ID)
		}
		seen[r.ID] = true
		for name, txt := range map[string]Text{"what": r.What, "why": r.Why, "before": r.Before, "after": r.After} {
			if strings.TrimSpace(txt.En) == "" || strings.TrimSpace(txt.Zh) == "" {
				t.Errorf("%s: %s is missing a language half", r.ID, name)
			}
		}
		switch r.Tier {
		case TierStrong, TierWeak:
			if !r.Mechanical() {
				t.Errorf("%s: tier %s promises a matcher and has none", r.ID, r.Tier)
			}
		case TierJudge:
			if r.Mechanical() {
				t.Errorf("%s: a judgement rule must not carry a matcher", r.ID)
			}
		default:
			t.Errorf("%s: unknown tier %q", r.ID, r.Tier)
		}
	}
}

func TestAMechanicalRuleAgreesWithItsOwnExample(t *testing.T) {
	for _, r := range Rules() {
		if !r.Mechanical() || structural[r.ID] {
			continue
		}
		read := func(s string) string { return r.find(entryProse{All: voiceProse(s)}) }
		// The jargon list is Chinese words, so an English example cannot trip it. One
		// half finding its own before is the honest bar.
		if read(r.Before.En) == "" && read(r.Before.Zh) == "" {
			t.Errorf("%s: neither example of the mistake trips its own matcher", r.ID)
		}
		if d := read(r.After.En); d != "" {
			t.Errorf("%s: the fixed English example still reads as a defect: %s", r.ID, d)
		}
		if d := read(r.After.Zh); d != "" {
			t.Errorf("%s: the fixed Chinese example still reads as a defect: %s", r.ID, d)
		}
	}
}

func TestAGuessIsAFindingUnlessItIsFiledAsOne(t *testing.T) {
	guess := "推送失败估计是 relay 那边 token 缓存的问题。"
	if d := voiceCheck(guess); d == "" {
		t.Error("a guess stated as a fact read as fine")
	}
	// --hypothesis is the honest way to shelve a lead, so the rule steps aside for one.
	e := proseOf(knowledgeOp{ID: "pitfalls/x", Title: "push fails", Body: guess, Status: StatusHypothesis})
	for _, f := range mechanicalFindings(e) {
		if strings.Contains(f.detail, "guess") {
			t.Errorf("a hypothesis was flagged for guessing: %s", f.detail)
		}
	}
}

// The base this shipped into had the id's slug in 457 of its 720 titles, and it was not
// drift: the machine index never printed the id, so the title was the only place an agent
// could read one. The index prints it now and the render drops the copy, so the rule is a
// judgement for the next entry rather than 457 findings nobody would work down.
func TestTheRenderDropsASlugTheTitleRepeats(t *testing.T) {
	for _, tc := range []struct{ name, title, id, want string }{
		{"a leading slug with a real title after it",
			"alias-makes-ops-noop 交互式别名让脚本里的 rm 什么也没干", "pitfalls/alias-makes-ops-noop",
			"交互式别名让脚本里的 rm 什么也没干"},
		{"separated by a colon",
			"alias-makes-ops-noop: interactive aliases do nothing", "pitfalls/alias-makes-ops-noop",
			"interactive aliases do nothing"},
		{"a retitled supersede kept the old slug's stem",
			"alias-makes-ops-noop 别名让 rm 静默", "pitfalls/alias-makes-ops-noop-3",
			"别名让 rm 静默"},
		{"a title that is only its slug keeps it — half a line is worse",
			"alias-makes-ops-noop", "pitfalls/alias-makes-ops-noop", "alias-makes-ops-noop"},
		{"a title that never carried one is untouched",
			"交互式别名让脚本里的 rm 什么也没干", "pitfalls/alias-makes-ops-noop",
			"交互式别名让脚本里的 rm 什么也没干"},
	} {
		if got := titleWithoutSlug(tc.title, tc.id); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestATitleNamingTheImplementationIsReported(t *testing.T) {
	op := knowledgeOp{ID: "pitfalls/wake-origin", Topic: "pitfalls",
		Title: "hqNudge 为 false 时 sourceKey 为空",
		Body:  "关掉唤醒之后事件不再记来源。"}
	var got string
	for _, f := range mechanicalFindings(proseOf(op)) {
		if f.check == checkTitle {
			got = f.detail
		}
	}
	if got == "" {
		t.Fatal("a title made of identifiers was not reported")
	}
	for _, want := range []string{"hqNudge", "sourceKey"} {
		if !strings.Contains(got, want) {
			t.Errorf("the finding does not name %s: %s", want, got)
		}
	}
	// A product spelled that way by its owner is not an identifier.
	op.Title = "macOS 26 上合盖后 tmux 会话仍在跑"
	for _, f := range mechanicalFindings(proseOf(op)) {
		if f.check == checkTitle {
			t.Errorf("macOS was read as an implementation name: %s", f.detail)
		}
	}
	// So is a name the author marked as code.
	op.Title = "关掉 `hqNudge` 之后事件不再记来源"
	for _, f := range mechanicalFindings(proseOf(op)) {
		if f.check == checkTitle {
			t.Errorf("a name in backticks was read as a defect: %s", f.detail)
		}
	}
}

func TestABodyThatRestatesTheTitleIsReported(t *testing.T) {
	op := knowledgeOp{ID: "pitfalls/restore", Topic: "pitfalls",
		Title: "restore 会往没跑过 agent 的 pane 里注入 agent",
		Body:  "restore 会往没跑过 agent 的 pane 里注入 agent。原因是存档记了命令。"}
	if got := findTitleEcho(proseOf(op)); got == "" {
		t.Error("a body opening with the title again read as fine")
	}
	op.Body = "存档记的是 pane 最后跑过的命令，只开过 shell 的 pane 也有这么一条。"
	if got := findTitleEcho(proseOf(op)); got != "" {
		t.Errorf("a body that adds something was flagged: %s", got)
	}
}

func TestStyleTextCarriesEveryRuleAndBothExamples(t *testing.T) {
	for _, zh := range []bool{true, false} {
		out := StyleText(zh)
		for _, r := range Rules() {
			want := r.What.En
			if zh {
				want = r.What.Zh
			}
			if !strings.Contains(out, want) {
				t.Errorf("zh=%v: the printed table is missing rule %s", zh, r.ID)
			}
		}
	}
}
