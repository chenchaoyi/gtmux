package mine

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// The ledger is what makes a daily pass safe: nothing is read twice, nothing is emitted
// twice, and a footgun keeps counting after its lesson was written.
//
//	sources.json — per log file: byte offset after the last complete line, size, mtime
//	emitted.json — every candidate id ever emitted (dismissed or filed — never re-emitted)
//	errors.json  — signature → count, distinct sessions, first/last seen, emitted-at
//	passes.jsonl — one line per pass
type ledger struct {
	Sources map[string]sourceMark
	Emitted map[string]bool
	Errors  map[string]*errorTally
	Passes  []passMark
}

type sourceMark struct {
	PolicyRevision int64      `json:"policy_revision,omitempty"`
	PolicySince    int64      `json:"policy_since,omitempty"`
	Offset         int64      `json:"offset"`
	Size           int64      `json:"size"`
	MTime          int64      `json:"mtime"`
	Carry          carryState `json:"carry,omitempty"`
}

type errorTally struct {
	Line      string          `json:"line"`
	Count     int             `json:"count"`
	Sessions  map[string]bool `json:"sessions"`
	First     int64           `json:"first"`
	Last      int64           `json:"last"`
	EmittedAt int64           `json:"emitted_at,omitempty"`
}

type passMark struct {
	At         int64 `json:"at"`
	Files      int   `json:"files"`
	Bytes      int64 `json:"bytes"`
	Candidates int   `json:"candidates"`
}

func loadLedger(dir string) (*ledger, error) {
	l := &ledger{Sources: map[string]sourceMark{}, Emitted: map[string]bool{}, Errors: map[string]*errorTally{}}
	if err := readJSON(filepath.Join(dir, "sources.json"), &l.Sources); err != nil {
		return nil, err
	}
	var ids []string
	if err := readJSON(filepath.Join(dir, "emitted.json"), &ids); err != nil {
		return nil, err
	}
	for _, id := range ids {
		l.Emitted[id] = true
	}
	if err := readJSON(filepath.Join(dir, "errors.json"), &l.Errors); err != nil {
		return nil, err
	}
	for _, e := range l.Errors {
		if e.Sessions == nil {
			e.Sessions = map[string]bool{}
		}
	}
	return l, nil
}

const maxEmittedIDs = 8192

func lockMineDir(dir string) (func(), error) {
	if err := state.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(dir, "mine.lock")
	var f *os.File
	var err error
	if state.IsManagedPath(lockPath) {
		f, err = state.OpenPrivateLock(lockPath)
	} else {
		f, err = state.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	}
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

func (l *ledger) boundedEmittedIDs() []string {
	var activeErrIDs []string
	var otherIDs []string
	for id := range l.Emitted {
		if e, ok := l.Errors[id]; ok && e.EmittedAt != 0 {
			activeErrIDs = append(activeErrIDs, id)
		} else {
			otherIDs = append(otherIDs, id)
		}
	}
	sort.Strings(activeErrIDs)
	sort.Strings(otherIDs)
	if len(activeErrIDs)+len(otherIDs) > maxEmittedIDs {
		keepOther := maxEmittedIDs - len(activeErrIDs)
		if keepOther <= 0 {
			otherIDs = nil
			if len(activeErrIDs) > maxEmittedIDs {
				activeErrIDs = activeErrIDs[len(activeErrIDs)-maxEmittedIDs:]
			}
		} else if len(otherIDs) > keepOther {
			otherIDs = otherIDs[len(otherIDs)-keepOther:]
		}
		l.Emitted = make(map[string]bool, len(activeErrIDs)+len(otherIDs))
		for _, id := range activeErrIDs {
			l.Emitted[id] = true
		}
		for _, id := range otherIDs {
			l.Emitted[id] = true
		}
	}
	ids := append(activeErrIDs, otherIDs...)
	sort.Strings(ids)
	return ids
}

func (l *ledger) save(dir string) error {
	if err := state.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ids := l.boundedEmittedIDs()
	if err := writeJSON(filepath.Join(dir, "sources.json"), l.Sources); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "emitted.json"), ids); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "errors.json"), l.Errors); err != nil {
		return err
	}
	var newestPass int64
	f, err := state.OpenFile(filepath.Join(dir, "passes.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	for _, p := range l.Passes {
		if p.At > newestPass {
			newestPass = p.At
		}
		b, _ := json.Marshal(p)
		if _, err := f.Write(append(b, '\n')); err != nil {
			f.Close()
			return err
		}
	}
	f.Close()
	l.Passes = nil
	if newestPass > 0 {
		_ = state.WriteInt64Marker(filepath.Join(dir, "last-pass"), newestPass)
	}
	return rotatePasses(filepath.Join(dir, "passes.jsonl"))
}

// The ledger's bounds (openspec change `diagnostics`, design 5.3). It grew without one:
// sources.json kept a mark for every transcript ever read, long after the agent deleted
// it, and errors.json every signature ever seen.
const (
	// signatureTTL: a signature unseen this long is pruned. If it comes back it is counted
	// afresh, which is right: a footgun quiet for three months and back is news again.
	signatureTTL = 90 * 24 * time.Hour
	// passesMaxLines: past this many passes, passes.jsonl keeps the newest passesKeep.
	// A pass runs daily, so this is about two years, cut back to one.
	passesMaxLines = 730
	passesKeep     = 365
)

// prune drops what the ledger no longer needs: signatures unseen for signatureTTL, and
// the read marks of log files that no longer exist. It reports how many of each it
// dropped.
func (l *ledger) prune(now time.Time) (signatures, sources int) {
	cutoff := now.Add(-signatureTTL).Unix()
	for sig, e := range l.Errors {
		if e.Last > 0 && e.Last < cutoff {
			delete(l.Errors, sig)
			signatures++
		}
	}
	for path := range l.Sources {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			delete(l.Sources, path)
			sources++
		}
	}
	if signatures+sources > 0 {
		diag.For("hq").Act("act.cleanup", diag.Caller(), "mine", diag.OK,
			"pruned the transcript miner's ledger", "signatures", signatures, "sources", sources, "reason", "mine")
	}
	return signatures, sources
}

// rotatePasses keeps passes.jsonl to its newest passesKeep lines once it passes
// passesMaxLines. The last line is what the status view reads; the rest is history.
func rotatePasses(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := bytes.SplitAfter(b, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= passesMaxLines {
		return nil
	}
	kept := bytes.Join(lines[len(lines)-passesKeep:], nil)
	return writeAtomicLedgerBytes(path, kept)
}

// Status is the ledger as a read: for `gtmux knowledge mine --status` and doctor.
type Status struct {
	Dir        string     `json:"dir"`
	LastPass   int64      `json:"last_pass"` // 0 = never
	Passes     int        `json:"passes"`
	Sources    int        `json:"sources"`
	Emitted    int        `json:"emitted"`
	Errors     int        `json:"errors"`
	TopErrors  []TopError `json:"top_errors"`
	LastReport passMark   `json:"last"`
}

// TopError is one recurring signature for the status view.
type TopError struct {
	Line     string `json:"line"`
	Count    int    `json:"count"`
	Sessions int    `json:"sessions"`
	Emitted  bool   `json:"emitted"`
}

// ReadStatus never fails on a missing ledger — "never run" is a state to render.
func ReadStatus(dir string, top int) Status {
	st := Status{Dir: dir}
	l, err := loadLedger(dir)
	if err != nil {
		return st
	}
	st.Sources, st.Emitted, st.Errors = len(l.Sources), len(l.Emitted), len(l.Errors)
	if f, err := os.Open(filepath.Join(dir, "passes.jsonl")); err == nil {
		dec := json.NewDecoder(f)
		for {
			var p passMark
			if dec.Decode(&p) != nil {
				break
			}
			st.Passes++
			st.LastPass, st.LastReport = p.At, p
		}
		f.Close()
	}
	for _, e := range l.Errors {
		st.TopErrors = append(st.TopErrors, TopError{Line: e.Line, Count: e.Count, Sessions: len(e.Sessions), Emitted: e.EmittedAt != 0})
	}
	sortTop(st.TopErrors)
	if len(st.TopErrors) > top {
		st.TopErrors = st.TopErrors[:top]
	}
	return st
}

// LastPassAt is the cheap gate the sensor reads before opening any log.
func LastPassAt(dir string) int64 {
	if ts := state.ReadInt64Marker(filepath.Join(dir, "last-pass")); ts > 0 {
		return ts
	}
	return ReadStatus(dir, 0).LastPass
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(b, v)
}

// writeJSON is temp+rename so a crash leaves the old file or the new one, never a torn one.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return writeAtomicLedgerBytes(path, b)
}

func writeAtomicLedgerBytes(path string, data []byte) error {
	if state.IsManagedPath(path) {
		return state.WritePrivateFile(path, data)
	}
	dir := filepath.Dir(path)
	if err := state.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := state.CreateTemp(dir, ".mine-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func sortTop(t []TopError) {
	for i := 1; i < len(t); i++ {
		for j := i; j > 0 && (t[j].Sessions > t[j-1].Sessions || (t[j].Sessions == t[j-1].Sessions && t[j].Count > t[j-1].Count)); j-- {
			t[j], t[j-1] = t[j-1], t[j]
		}
	}
}
