package events

import (
	"encoding/json"
	"os"
	"testing"
)

func appendRecords(t *testing.T, path string, recs ...Record) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, r := range recs {
		b, _ := json.Marshal(r)
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
	}
}

func send(seq, ts int64, pane, actor, payload string) Record {
	return Record{Seq: seq, Ts: ts, Event: AuditEventSend, Pane: pane, Actor: actor, Summary: "landed: " + payload}
}

// The point of the index: a call reads what was appended since the last one, not the
// journal. The whole journal was about 110ms on a real 16 MB file, every chat poll.
func TestTheIndexReadsOnlyWhatWasAppended(t *testing.T) {
	withJournal(t, []Record{
		send(1, 1000, "%19", "hq", "先停一下"),
		send(2, 1001, "%19", "agent:%31", "把这个做完"),
		{Seq: 3, Ts: 1002, Event: "Stop", Pane: "%19"},
	})
	x := &SendIndex{}
	if got := x.Senders("%19", 0); got["先停一下"] != "hq" || got["把这个做完"] != "agent:%31" {
		t.Fatalf("the first read missed a delivery: %v", got)
	}
	before := x.parsed
	appendRecords(t, Path(), send(4, 1003, "%19", "hq", "发版吧"))
	if got := x.Senders("%19", 0); got["发版吧"] != "hq" {
		t.Fatalf("an appended delivery was not seen: %v", got)
	}
	if n := x.parsed - before; n != 1 {
		t.Errorf("the second call decoded %d records, want only the 1 appended", n)
	}
	// Nothing appended: nothing decoded.
	before = x.parsed
	x.Senders("%19", 0)
	if n := x.parsed - before; n != 0 {
		t.Errorf("an unchanged journal decoded %d records", n)
	}
}

// Rotation renames the active file and starts a new one. The index has to notice it is
// no longer reading the same file, and still see what moved into the older generation.
func TestTheIndexFollowsARotation(t *testing.T) {
	withJournal(t, []Record{send(1, 1000, "%19", "hq", "先停一下")})
	x := &SendIndex{}
	x.Senders("%19", 0)
	if err := os.Rename(Path(), rotatedPath()); err != nil {
		t.Fatal(err)
	}
	appendRecords(t, Path(), send(2, 2000, "%19", "agent:%31", "继续做下一个"))
	got := x.Senders("%19", 0)
	if got["先停一下"] != "hq" {
		t.Errorf("a delivery that rotated into the older generation was lost: %v", got)
	}
	if got["继续做下一个"] != "agent:%31" {
		t.Errorf("a delivery in the new generation was not seen: %v", got)
	}
}

// A writer can be mid-line when the index reads. Half a record must wait for its newline
// rather than be decoded, fail, and be skipped for good.
func TestAHalfWrittenLineWaitsForItsEnd(t *testing.T) {
	withJournal(t, nil)
	b, _ := json.Marshal(send(1, 1000, "%19", "hq", "先停一下"))
	f, err := os.OpenFile(Path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	half := len(b) / 2
	_, _ = f.Write(b[:half])
	x := &SendIndex{}
	if got := x.Senders("%19", 0); len(got) != 0 {
		t.Fatalf("half a record produced %v", got)
	}
	_, _ = f.Write(append(b[half:], '\n'))
	if got := x.Senders("%19", 0); got["先停一下"] != "hq" {
		t.Errorf("the record was lost once it was complete: %v", got)
	}
}

// The window still applies: a delivery before it cannot claim a prompt after it, or an
// old "继续" from HQ would put HQ's name on the commander's own "继续" months later.
func TestADeliveryBeforeTheWindowDoesNotCount(t *testing.T) {
	withJournal(t, []Record{send(1, 1000, "%19", "hq", "继续"), send(2, 5000, "%19", "hq", "发版吧")})
	x := &SendIndex{}
	got := x.Senders("%19", 4000)
	if _, ok := got["继续"]; ok {
		t.Errorf("a delivery before the window was used: %v", got)
	}
	if got["发版吧"] != "hq" {
		t.Errorf("a delivery inside the window was dropped: %v", got)
	}
	if got := x.Senders("%20", 0); len(got) != 0 {
		t.Errorf("another pane's deliveries leaked in: %v", got)
	}
}
