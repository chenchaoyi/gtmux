package knowledge

import (
	"os"
	"strings"
	"testing"
)

// The knowledge base speaks two languages (kb-bilingual). Everything here runs on a
// synthetic ledger in a temp HOME.

func TestInferLangReadsTheText(t *testing.T) {
	// A bare-id title defers to the body.
	if got := inferLang("%63", "这台机器的磁盘分诊报告在 notes/reports/"); got != "zh" {
		t.Fatalf("a title with no letters defers to the body: got %s", got)
	}
	for text, want := range map[string]string{
		"办公网会 TLS reset wrangler，重试就好":                  "zh",
		"the office network TLS-resets wrangler; retry": "en",
		"`gtmux reap %63` 可回收，它占 302MB":                 "zh",
		"": "en",
		"internal/knowledge/lint.go orphan 断链 near-duplicate stale": "zh",
	} {
		if got := inferLang(text, ""); got != want {
			t.Errorf("inferLang(%q) = %s, want %s", text, got, want)
		}
	}
}

// A record written before the field says nothing about its language: the fold infers it
// and marks it assumed; the file is not touched.
func TestOldRecordsGetTheirLanguageInferredNotRewritten(t *testing.T) {
	asHQ(t)
	writeOps(t, knowledgeOp{V: 1, Op: knowledgeOpAdd, ID: "pitfalls/old", Topic: "pitfalls", Title: "旧条目，没有语言字段", At: 1})
	old := mustLive(t, "pitfalls/old")
	if old.Lang != "zh" || !old.LangAssumed {
		t.Fatalf("inferred zh + assumed, got %+v", old)
	}
	raw := readLedger(t)
	if strings.Contains(raw, `"lang"`) {
		t.Fatal("the ledger was rewritten")
	}
}

// add states or infers the language and may carry the other half; a wrong pairing is
// refused before anything is written.
func TestAddCarriesTheOtherHalf(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "锁屏时别指定 udid 装机", "--alt-lang", "en", "--alt-title", "Never install by udid on a locked phone"}); rc != 0 {
		t.Fatal("bilingual add failed")
	}
	e := mustLive(t, "pitfalls/udid")
	if e.Lang != "zh" || e.LangAssumed || e.Alt == nil || e.Alt.Lang != "en" || e.Alt.Title != "Never install by udid on a locked phone" {
		t.Fatalf("language lost: %+v alt=%+v", e, e.Alt)
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "same language twice", "--lang", "en", "--alt-lang", "en", "--alt-title", "x"}); rc == 0 {
		t.Fatal("an alternate in the entry's own language must be refused")
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "half a half", "--alt-lang", "zh"}); rc == 0 {
		t.Fatal("an alternate without a title must be refused")
	}
	if _, ok := findLive(mustLiveAll(t), "pitfalls/same-language-twice"); ok {
		t.Fatal("a refused add must not land")
	}
}

// alt writes the other half of an existing entry, and its stated --lang on the entry
// corrects an inferred one; supersede drops the alternate (the content changed).
func TestAltVerbAndSupersedeDropsTheHalf(t *testing.T) {
	asHQ(t)
	writeOps(t, knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "howto/release", Topic: "howto", Title: "发版：打 tag，等 app job，装机", At: 1})
	if rc := CmdKnowledge([]string{"alt", "howto/release", "--lang", "en", "--title", "Cut a release: tag, wait for the app job, install"}); rc != 0 {
		t.Fatal("alt failed")
	}
	e := mustLive(t, "howto/release")
	if e.Alt == nil || e.Alt.Lang != "en" || e.LangAssumed {
		t.Fatalf("alt not applied or inference not settled: %+v alt=%+v", e, e.Alt)
	}
	if rc := CmdKnowledge([]string{"alt", "howto/release", "--lang", "zh", "--title", "同语言"}); rc == 0 {
		t.Fatal("alt in the entry's own language must be refused")
	}
	if rc := CmdKnowledge([]string{"supersede", "howto/release", "--title", "发版：打 tag，等两个 job，装机"}); rc != 0 {
		t.Fatal("supersede failed")
	}
	live := mustLiveAll(t)
	var succ knowledgeOp
	for _, op := range live {
		if op.Topic == "howto" {
			succ = op
		}
	}
	if succ.ID == "" || succ.Alt != nil {
		t.Fatalf("the successor must start monolingual: %+v", succ)
	}
}

// pick: the reader's language when the entry has it, else the source with a tag.
func TestPickFollowsTheReader(t *testing.T) {
	op := knowledgeOp{Lang: "zh", Title: "中", Body: "正文", Alt: &altHalf{Lang: "en", Title: "en", Body: "body"}}
	if tt, b, tag := pick(op, "en"); tt != "en" || b != "body" || tag != "" {
		t.Fatalf("en reader should get the english half: %q %q %q", tt, b, tag)
	}
	if tt, _, tag := pick(op, "zh"); tt != "中" || tag != "" {
		t.Fatalf("zh reader gets the source: %q %q", tt, tag)
	}
	mono := knowledgeOp{Lang: "zh", Title: "中"}
	if tt, _, tag := pick(mono, "en"); tt != "中" || tag != "zh" {
		t.Fatalf("no english half: source with a tag, got %q %q", tt, tag)
	}
	if _, _, tag := pick(mono, "fr"); tag != "" {
		t.Fatal("an unknown reader language reads the source untagged")
	}
}

func mustLiveAll(t *testing.T) []knowledgeOp {
	t.Helper()
	live, err := liveKnowledge()
	if err != nil {
		t.Fatal(err)
	}
	return live
}

func readLedger(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(knowledgeLedgerPath())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Files on this machine render in the base's majority language, whatever process wrote
// them; an entry written in the other language shows its alternate half when it has one
// and its source, tagged, when it does not.
func TestMachineFilesRenderInTheBasesLanguage(t *testing.T) {
	asHQ(t) // renderTopic looks for a legacy file under the HQ home
	zh1 := knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/a", Topic: "pitfalls", Title: "甲", Body: "正文甲", At: 1, Lang: "zh", Audience: AudienceMachine}
	zh2 := knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/b", Topic: "pitfalls", Title: "乙", At: 2, Lang: "zh"}
	en := knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/c", Topic: "pitfalls", Title: "written in English", Body: "body", At: 3, Lang: "en",
		Alt: &altHalf{Lang: "zh", Title: "用中文写的", Body: "中文正文"}, Audience: AudienceMachine}
	enOnly := knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/d", Topic: "pitfalls", Title: "English only", At: 4, Lang: "en", Audience: AudienceMachine}
	zh3 := knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/e", Topic: "pitfalls", Title: "丙", At: 5, Lang: "zh"}
	live := foldKnowledge([]knowledgeOp{zh1, zh2, zh3, en, enOnly})
	if machineLang(live) != "zh" {
		t.Fatalf("majority is zh, got %s", machineLang(live))
	}
	topic := renderTopic("pitfalls", "", live)
	if !strings.Contains(topic, "**用中文写的**") || strings.Contains(topic, "**written in English**") {
		t.Fatalf("the English entry's Chinese half should render: %s", topic)
	}
	if !strings.Contains(topic, "**English only [en]**") {
		t.Fatalf("an entry with no Chinese half renders its source, tagged: %s", topic)
	}
	idx := machineIndex(live)
	if !strings.Contains(idx, "用中文写的") || !strings.Contains(idx, "English only") {
		t.Fatalf("the index block follows the same rule: %s", idx)
	}
	if machineLang(foldKnowledge([]knowledgeOp{en, enOnly})) != "en" {
		t.Fatal("an English-majority base renders English")
	}
}

// The everyone exit goes to a public English tracker: the issue takes the English half
// when the entry has one.
func TestEveryoneExitPrefersEnglish(t *testing.T) {
	op := knowledgeOp{ID: "pitfalls/x", Topic: "pitfalls", Title: "中文标题", Body: "中文正文", Lang: "zh",
		Alt:      &altHalf{Lang: "en", Title: "English title", Body: "English body"},
		Audience: AudienceEveryone, PromotedAt: 1788998800, PromoteWhy: "holds everywhere"}
	u := IssueURL(op)
	if !strings.Contains(u, "English+title") || strings.Contains(u, "%E4%B8%AD%E6%96%87%E6%A0%87%E9%A2%98") {
		t.Fatalf("issue title should be the English half: %s", u)
	}
	core := renderPromotionCore(op)
	if !strings.Contains(core, "# promotion: English title") || !strings.Contains(core, "English body") {
		t.Fatalf("the brief for everyone is English: %s", core)
	}
	mono := op
	mono.Alt = nil
	if !strings.Contains(renderPromotionCore(mono), "# promotion: 中文标题") {
		t.Fatal("with no English half the brief keeps the source")
	}
}

// Lint counts entries with no other half; the API rows carry the language fields.
func TestMonolingualIsCountedAndTheAPICarriesLanguage(t *testing.T) {
	ops := []knowledgeOp{
		{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/a", Topic: "pitfalls", Title: "甲", At: 1, Lang: "zh"},
		{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/b", Topic: "pitfalls", Title: "乙", At: 2, Lang: "zh", Alt: &altHalf{Lang: "en", Title: "B", Body: "b body"}},
	}
	rep := lint(ops, 10)
	if rep.Counts["monolingual"] != 1 {
		t.Fatalf("one entry lacks its other half: %+v", rep.Counts)
	}
	live := foldKnowledge(ops)
	row := rowOf(live[1])
	if row.Lang != "zh" || row.AltLang != "en" || row.AltTitle != "B" || row.LangAssumed {
		t.Fatalf("row lacks the language fields: %+v", row)
	}
	asHQ(t)
	writeOps(t, ops...)
	full, ok := KnowledgeEntry("pitfalls/b")
	if !ok || full.AltBody != "b body" {
		t.Fatalf("the entry read carries the alternate body: %+v", full)
	}
	old := rowOf(foldKnowledge([]knowledgeOp{{V: 1, Op: knowledgeOpAdd, ID: "pitfalls/o", Topic: "pitfalls", Title: "旧", At: 1}})[0])
	if old.Lang != "zh" || !old.LangAssumed {
		t.Fatalf("an old record's row says its language was inferred: %+v", old)
	}
}
