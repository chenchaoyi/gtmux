// The ADVICE LEDGER (hq-counsel): what HQ proposed, and what became of it.
//
// Why this exists at all. The supervisor's other ledgers record what it KNOWS (the
// knowledge base), what it DISPATCHED (tasks), and what it SAW (events). None of them
// record what it ADVISED. Without that number neither the commander nor HQ can answer the
// question that decides whether a chief of staff is worth having: is its judgement getting
// better, and is it still speaking up at all? A supervisor that has quietly stopped
// offering a view looks, in every other ledger, exactly like one that has nothing to say.
//
// Shape, borrowed from the knowledge ledger because the reasons are the same: append-only
// JSONL, one operation per line, folded into a live set at read time. An outcome never
// edits the advice it belongs to; it is its own record, so the history of a reversal
// survives. Writes are gated to the HQ home, like knowledge mutations: this is the
// supervisor's own record of its own work, and the commander is never asked to file
// anything.
package advice

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// schemaV is stamped on every record.
const schemaV = 1

// Operations.
const (
	opGive = "give"
	opMark = "mark"
)

// Outcomes. An advice with none of these is still open.
const (
	// Taken: the commander did what was advised, or changed course because of it.
	Taken = "taken"
	// Declined: they considered it and went another way. Their own words are kept when
	// they gave them, because the reason is the part worth reading later.
	Declined = "declined"
	// Moot: events settled it before the commander did. Neither a hit nor a miss, and
	// counting it as either would bend the tally.
	Moot = "moot"
)

// Bounds. They refuse loudly rather than truncate: a half-recorded piece of advice is
// worse than none, because it reads as complete.
const (
	whatMax  = 400
	whyMax   = 600
	aboutMax = 200
	wordsMax = 600
)

// record is one line of the ledger.
type record struct {
	V  int    `json:"v"`
	Op string `json:"op"`
	ID string `json:"id"`
	At int64  `json:"at"`
	// give
	What  string `json:"what,omitempty"`
	Why   string `json:"why,omitempty"`
	About string `json:"about,omitempty"`
	// mark
	Outcome string `json:"outcome,omitempty"`
	Words   string `json:"words,omitempty"`
}

// Advice is one folded piece of advice with whatever became of it.
type Advice struct {
	ID      string `json:"id"`
	At      int64  `json:"at"`
	What    string `json:"what"`
	Why     string `json:"why,omitempty"`
	About   string `json:"about,omitempty"`
	Outcome string `json:"outcome,omitempty"`
	Words   string `json:"words,omitempty"`
	MarkAt  int64  `json:"markedAt,omitempty"`
}

// Open reports whether this advice is still waiting on an outcome.
func (a Advice) Open() bool { return a.Outcome == "" }

// Dir is where the ledger lives: beside the knowledge base, inside HQ's home.
func Dir() string { return state.HQHome() }

// Path is the ledger file.
func Path() string { return filepath.Join(Dir(), "advice.jsonl") }

// FromHQHome reports whether this process is running as the supervisor, by the same
// cwd-keyed rule the knowledge verbs and `events --ack` use.
func FromHQHome() bool {
	cwd, err := os.Getwd()
	return err == nil && cwd == state.HQHome()
}

// read returns every record in order, skipping lines that do not parse (a truncated write
// must not take the whole ledger down with it).
func read() ([]record, error) {
	f, err := os.Open(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r record
		if json.Unmarshal([]byte(line), &r) != nil || r.ID == "" {
			continue
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

// append writes one record.
func appendRecord(r record) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(Path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// fold turns the operation log into the live set, oldest first.
func fold(rs []record) []Advice {
	byID := map[string]*Advice{}
	var order []string
	for _, r := range rs {
		switch r.Op {
		case opGive:
			if _, seen := byID[r.ID]; !seen {
				order = append(order, r.ID)
			}
			byID[r.ID] = &Advice{ID: r.ID, At: r.At, What: r.What, Why: r.Why, About: r.About}
		case opMark:
			if a, ok := byID[r.ID]; ok {
				// The LAST mark wins: the commander is allowed to change their mind,
				// and the earlier record stays in the log as history.
				a.Outcome, a.Words, a.MarkAt = r.Outcome, r.Words, r.At
			}
		}
	}
	out := make([]Advice, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out
}

// List returns every piece of advice, oldest first.
func List() ([]Advice, error) {
	rs, err := read()
	if err != nil {
		return nil, err
	}
	return fold(rs), nil
}

// nextID is the next free short id: a1, a2, … Short because HQ types it by hand when it
// marks an outcome, and a uuid there would be a transcription error waiting to happen.
func nextID(live []Advice) string {
	max := 0
	for _, a := range live {
		var n int
		if _, err := fmt.Sscanf(a.ID, "a%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("a%d", max+1)
}

// Give records a piece of advice and returns its id.
func Give(what, why, about string, now int64) (string, error) {
	what, why, about = strings.TrimSpace(what), strings.TrimSpace(why), strings.TrimSpace(about)
	if what == "" {
		return "", errors.New("advice: nothing to record (say what you advised)")
	}
	if len(what) > whatMax {
		return "", fmt.Errorf("advice: what is %d bytes, over the %d limit", len(what), whatMax)
	}
	if len(why) > whyMax {
		return "", fmt.Errorf("advice: why is %d bytes, over the %d limit", len(why), whyMax)
	}
	if len(about) > aboutMax {
		return "", fmt.Errorf("advice: about is %d bytes, over the %d limit", len(about), aboutMax)
	}
	live, err := List()
	if err != nil {
		return "", err
	}
	id := nextID(live)
	return id, appendRecord(record{V: schemaV, Op: opGive, ID: id, At: now, What: what, Why: why, About: about})
}

// Mark records what became of one piece of advice.
func Mark(id, outcome, words string, now int64) error {
	switch outcome {
	case Taken, Declined, Moot:
	default:
		return fmt.Errorf("advice: unknown outcome %q (taken | declined | moot)", outcome)
	}
	words = strings.TrimSpace(words)
	if len(words) > wordsMax {
		return fmt.Errorf("advice: their words are %d bytes, over the %d limit", len(words), wordsMax)
	}
	live, err := List()
	if err != nil {
		return err
	}
	found := false
	for _, a := range live {
		if a.ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("advice: no advice called %q", id)
	}
	return appendRecord(record{V: schemaV, Op: opMark, ID: id, At: now, Outcome: outcome, Words: words})
}

// Tally is the count over a window: the answer to "am I still advising, and is any of it
// landing".
type Tally struct {
	SinceSec int64 `json:"sinceSec,omitempty"`
	Given    int   `json:"given"`
	Taken    int   `json:"taken"`
	Declined int   `json:"declined"`
	Moot     int   `json:"moot"`
	Open     int   `json:"open"`
}

// TakenRate is taken over the advice that got an answer either way. Advice still open, and
// advice overtaken by events, are not in the denominator: neither one is a verdict on the
// judgement behind it.
func (t Tally) TakenRate() (float64, bool) {
	d := t.Taken + t.Declined
	if d == 0 {
		return 0, false
	}
	return float64(t.Taken) / float64(d), true
}

// Count tallies the advice given at or after `since` (0 = all of it).
func Count(live []Advice, since int64) Tally {
	t := Tally{SinceSec: since}
	for _, a := range live {
		if since > 0 && a.At < since {
			continue
		}
		t.Given++
		switch a.Outcome {
		case Taken:
			t.Taken++
		case Declined:
			t.Declined++
		case Moot:
			t.Moot++
		default:
			t.Open++
		}
	}
	return t
}

// Recent returns the newest n first, for a listing.
func Recent(live []Advice, n int, openOnly bool) []Advice {
	var out []Advice
	for _, a := range live {
		if openOnly && !a.Open() {
			continue
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At > out[j].At })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// ParseSince turns "30d" / "12h" / "all" into a unix second floor.
func ParseSince(s string, now int64) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "all" {
		return 0, nil
	}
	var n int
	var unit rune
	if _, err := fmt.Sscanf(s, "%d%c", &n, &unit); err != nil || n <= 0 {
		return 0, fmt.Errorf("advice: cannot read a window from %q (try 30d, 12h, or all)", s)
	}
	switch unit {
	case 'd':
		return now - int64(n)*86400, nil
	case 'h':
		return now - int64(n)*3600, nil
	default:
		return 0, fmt.Errorf("advice: unknown unit %q in %q (d or h)", string(unit), s)
	}
}

// Now is the clock, a var so tests can pin it.
var Now = func() int64 { return time.Now().Unix() }
