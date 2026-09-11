// Package mine is the LLM-free transcript miner behind HQ's knowledge distillation
// (openspec change hq-transcript-mining). It reads the coding agents' session logs on
// this machine from a byte watermark, subtracts everything the MACHINE wrote (tool
// results, harness blocks, gtmux's own wake lines and `gtmux send` payloads, compaction
// summaries, pastes), and emits two kinds of CANDIDATE for HQ to judge:
//
//   - a correction-shaped exchange: a typed human line that answers an assistant reply
//     and carries a correction lexicon hit or emphatic punctuation, with the tail of what
//     the assistant had just said as context;
//   - a recurring tool error: a normalized error signature seen in two or more distinct
//     sessions, with its count.
//
// Both are LEADS, high-recall by design — precision is the distill pass's job, and by
// the time a line reaches the lexicon the deterministic subtractions above have removed
// well over 99% of the bytes. The package is a leaf: it returns candidates and keeps its
// own ledger; it never writes a knowledge entry and never calls a model.
package mine

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Candidate kinds.
const (
	KindCorrection = "correction"
	KindError      = "recurring-error"
)

// Candidate is one lead for HQ. ID is stable across passes (the ledger keys on it).
type Candidate struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	At      int64  `json:"at"`                // unix seconds of the human line / last sighting
	Session string `json:"session,omitempty"` // agent session id
	Project string `json:"project,omitempty"` // basename of the session's cwd
	Agent   string `json:"agent,omitempty"`
	// Line is the human line (correction) or the error signature (recurring-error).
	Line string `json:"line"`
	// Context is the tail of the assistant text the human answered (correction only).
	Context string `json:"context,omitempty"`
	// Count / Sessions: how often and in how many distinct sessions (recurring-error).
	Count    int `json:"count,omitempty"`
	Sessions int `json:"sessions,omitempty"`
}

// Options bound a pass.
type Options struct {
	// Roots are the log roots to scan; empty means the default agent roots.
	Roots []Root
	// Since bounds CORRECTION candidates to lines at or after this time (zero = no bound).
	// The error tally is always over everything scanned — counts are the point.
	Since time.Time
	// Now is the pass timestamp (tests pin it).
	Now time.Time
	// DryRun scans and returns candidates but writes nothing to the ledger.
	DryRun bool
	// MachineHeads are normalized heads of text the machine typed into panes (gtmux
	// send payloads, wake lines) — a typed line whose head is one of these is not the
	// human. Built by the caller from the audit journal (see HeadsFromAudit).
	MachineHeads map[string]bool
}

// Report is what a pass did.
type Report struct {
	At         int64       `json:"at"`
	Files      int         `json:"files"`      // files opened this pass
	Bytes      int64       `json:"bytes"`      // bytes read this pass
	Lines      int         `json:"lines"`      // human-typed lines seen (after subtraction)
	Candidates []Candidate `json:"candidates"` // NEW candidates (not in the ledger before)
	Skipped    int         `json:"skipped"`    // candidates suppressed because already emitted
}

// Root is one agent's log tree. Agent selects the reader (claude | codex | opencode |
// kimi); Glob matches log files under Dir (a relative pattern for filepath.Glob).
type Root struct {
	Agent string
	Dir   string
	Glob  string
}

type reader func(path string, start int64, o readOpts) (readResult, int64, error)

func readerFor(agent string) reader {
	switch agent {
	case "claude":
		return readClaude
	case "codex":
		return readCodex
	case "opencode":
		return readOpencode
	case "kimi":
		return readKimi
	}
	return nil
}

// Run executes one pass against the ledger in dir (created on first use).
func Run(dir string, o Options) (Report, error) {
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	led, err := loadLedger(dir)
	if err != nil {
		return Report{}, err
	}
	rep := Report{At: o.Now.Unix()}
	roots := o.Roots
	if len(roots) == 0 {
		roots = DefaultRoots()
	}
	var files []logFile
	for _, r := range roots {
		rd := readerFor(r.Agent)
		if rd == nil {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(r.Dir, r.Glob))
		for _, m := range matches {
			files = append(files, logFile{agent: r.Agent, path: m, read: rd})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })

	sinceUnix := int64(0)
	if !o.Since.IsZero() {
		sinceUnix = o.Since.Unix()
	}
	var out []Candidate
	for _, f := range files {
		fi, err := os.Stat(f.path)
		if err != nil || fi.IsDir() {
			continue
		}
		src := led.Sources[f.path]
		start := src.Offset
		if fi.Size() < start { // rewritten, not appended — start over
			start = 0
		}
		if fi.Size() == start {
			continue // nothing new
		}
		carry := src.Carry
		if start == 0 {
			carry = carryState{}
		}
		res, end, err := f.read(f.path, start, readOpts{
			sinceUnix: sinceUnix, machine: o.MachineHeads, carry: carry,
		})
		if err != nil {
			continue // one unreadable log must not stop the pass
		}
		rep.Files++
		rep.Bytes += end - start
		rep.Lines += res.humanLines
		for _, c := range res.corrections {
			c.Agent = f.agent
			if led.Emitted[c.ID] {
				rep.Skipped++
				continue
			}
			out = append(out, c)
		}
		for sig, hit := range res.errors {
			e := led.Errors[sig]
			if e == nil {
				e = &errorTally{First: hit.at, Line: hit.line, Sessions: map[string]bool{}}
				led.Errors[sig] = e
			}
			e.Count += hit.count
			e.Last = maxInt64(e.Last, hit.at)
			e.Sessions[hit.session] = true
		}
		led.Sources[f.path] = sourceMark{Offset: end, Size: fi.Size(), MTime: fi.ModTime().Unix(), Carry: res.carry}
	}
	// Recurring errors: emit once when the signature crosses two distinct sessions.
	for sig, e := range led.Errors {
		if len(e.Sessions) < 2 || e.EmittedAt != 0 {
			continue
		}
		out = append(out, Candidate{
			Kind: KindError, ID: sig, At: e.Last, Line: e.Line,
			Count: e.Count, Sessions: len(e.Sessions),
		})
		e.EmittedAt = o.Now.Unix()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })
	rep.Candidates = out
	if o.DryRun {
		return rep, nil
	}
	for _, c := range out {
		led.Emitted[c.ID] = true
	}
	led.Passes = append(led.Passes, passMark{At: rep.At, Files: rep.Files, Bytes: rep.Bytes, Candidates: len(out)})
	return rep, led.save(dir)
}

// DefaultRoots are every agent whose logs gtmux knows how to read. The roots honour the
// same overrides the transcript package does ($CODEX_HOME, $KIMI_CODE_HOME).
func DefaultRoots() []Root {
	return []Root{
		{Agent: "claude", Dir: filepath.Join(state.Home(), ".claude", "projects"), Glob: filepath.Join("*", "*.jsonl")},
		{Agent: "codex", Dir: filepath.Join(transcript.CodexHome(), "sessions"), Glob: filepath.Join("*", "*", "*", "*.jsonl")},
		{Agent: "opencode", Dir: transcript.OpencodeDir(), Glob: "*.jsonl"},
		{Agent: "kimi", Dir: filepath.Join(transcript.KimiHome(), "sessions"), Glob: filepath.Join("*", "*", "agents", "main", "wire.jsonl")},
	}
}

// HeadKey normalizes text to the key the machine-head set uses: whitespace collapsed,
// first headRunes runes. Both sides (audit summary, typed line) go through it.
func HeadKey(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > headRunes {
		r := []rune(s)
		s = string(r[:headRunes])
	}
	return s
}

// headRunes is the compare width. Audit summaries are budgeted at 200 bytes for a send,
// so a shorter head is what both sides can be trusted to share.
const headRunes = 60

func candidateID(session, uuid string) string {
	h := sha1.Sum([]byte(session + "\x00" + uuid))
	return hex.EncodeToString(h[:])[:12]
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

type logFile struct {
	agent, path string
	read        reader
}
