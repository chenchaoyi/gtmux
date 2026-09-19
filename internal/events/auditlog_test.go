package events

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Each audit act is written to the journal and the log by one call, and the log keeps who,
// what and how it ended, never the text: a wake's lines and a send's message carry pane
// names and prompts, and a retirement's reason is its author's words.
func TestAuditActsReachTheLogWithoutTheirText(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMUX_PANE", "")
	t.Setenv("GTMUX_ACTOR", "")
	t.Chdir(t.TempDir())
	now := time.Now().Unix()

	AuditWakeDelivered("%4", "» ◆ gtmux·done │ %7 finished the private-roadmap task · #81a11d", now)
	AuditWakeDropped(DropUnconfirmed, "» ◆ gtmux·asks │ %9 wants the staging password", now)
	AuditSend("%7", "refused-draft", "please rewrite the private-roadmap doc", now)
	AuditSend("%7", "landed", "please rewrite the private-roadmap doc", now)
	AuditKnowledge("retire pitfalls/tunnel-dns: the corp resolver was fixed in March", now)
	AuditReap("t-42", "%7", "worktree removed; branch deleted", now)

	b, err := os.ReadFile(filepath.Join(state.LogsDir(), time.Now().Format("2006-01-02")+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	for _, secret := range []string{"private-roadmap", "staging password", "corp resolver", "worktree removed"} {
		if strings.Contains(log, secret) {
			t.Errorf("the log carries the act's text %q:\n%s", secret, log)
		}
	}
	for _, want := range []string{
		`"event":"act.wake.delivered"`, `"batch":"#81a11d"`,
		`"event":"act.wake.dropped"`, `"outcome":"failed"`,
		`"event":"act.send"`, `"outcome":"refused"`, `"reason":"refused-draft"`,
		`"event":"act.knowledge"`, `"target":"pitfalls/tunnel-dns"`, `"verb":"retire"`,
		`"event":"act.reap"`, `"task":"t-42"`, `"actor":"user"`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the log is missing %s:\n%s", want, log)
		}
	}
	if n := strings.Count(log, `"event":"act.send"`); n != 2 {
		t.Errorf("two sends, %d act.send entries", n)
	}
	// The journal still has the text: HQ reads it there.
	var journal []string
	for _, r := range Read(0, now+1) {
		journal = append(journal, r.Summary)
	}
	if !strings.Contains(strings.Join(journal, "\n"), "private-roadmap") {
		t.Error("the journal lost the send's head, which HQ reads")
	}
}

func TestBatchTagIsOnlyATrailingHexID(t *testing.T) {
	for in, want := range map[string]string{
		"» a · #81a11d":     "#81a11d",
		"» a #tag in text":  "",
		"» a · #NOTHEX":     "",
		"":                  "",
		"» a · #81a11d\n\n": "#81a11d",
	} {
		if got := batchTag(in); got != want {
			t.Errorf("batchTag(%q) = %q, want %q", in, got, want)
		}
	}
}
