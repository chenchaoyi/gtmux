// Package restorejob owns bounded, private restore receipts. It never runs tmux.
package restorejob

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const MaxSessions = 256
const MaxAgents = 2048
const MaxJobs = 128
const WorkerBudget = 30 * time.Minute
const maxBytes = 4 << 20

type Session struct {
	Name      string `json:"name"`
	Windows   int    `json:"windows"`
	Panes     int    `json:"panes"`
	Agents    int    `json:"agents"`
	Resumable int    `json:"resumable"`
	State     string `json:"state"`
}
type Target struct{ Agent, SessionID, HomeID string }
type Plan struct {
	Targets    map[string]Target `json:"-"`
	InputStamp string            `json:"-"`
	SavePath   string            `json:"-"`
	Revision   string            `json:"revision"`
	ServerTime int64             `json:"server_time"`
	SavedAt    int64             `json:"saved_at"`
	ResumeMode string            `json:"resume_mode"`
	Sessions   []Session         `json:"sessions"`
}
type Result struct {
	Loc   string `json:"loc"`
	Pane  string `json:"pane"`
	State string `json:"state"`
}
type Job struct {
	ID        string    `json:"id"`
	Revision  string    `json:"revision"`
	CreatedAt int64     `json:"created_at"`
	UpdatedAt int64     `json:"updated_at"`
	Phase     string    `json:"phase"`
	Code      string    `json:"code,omitempty"`
	Sessions  []Session `json:"sessions"`
	Agents    []Result  `json:"agents,omitempty"`
}
type Snapshot struct {
	Plan Plan `json:"plan"`
	Job  *Job `json:"job,omitempty"`
}
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }
func fail(code string) error   { return &Error{code} }
func Terminal(phase string) bool {
	return phase == "completed" || phase == "partial" || phase == "failed" || phase == "interrupted"
}

var idPattern = regexp.MustCompile(`^([0-9]{10})-[A-Za-z0-9-]{16,64}$`)

func ValidID(id string) bool { return idPattern.MatchString(id) }
func freshID(id string, now time.Time) bool {
	m := idPattern.FindStringSubmatch(id)
	if m == nil {
		return false
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	age := now.Unix() - n
	return age >= -60 && age <= int64((24*time.Hour).Seconds())
}

type Store struct{ Now func() time.Time }

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
func path() string { return filepath.Join(state.Dir(), "restore-jobs.json") }
func read() ([]Job, error) {
	f, e := state.OpenPrivateRead(path())
	if errors.Is(e, os.ErrNotExist) {
		return []Job{}, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if e != nil || len(b) > maxBytes {
		return nil, fail("storage_unavailable")
	}
	var rows []Job
	if json.Unmarshal(b, &rows) != nil || rows == nil || len(rows) > MaxJobs {
		return nil, fail("storage_unavailable")
	}
	seen := map[string]bool{}
	for _, j := range rows {
		if !ValidID(j.ID) || seen[j.ID] || j.Revision == "" || j.CreatedAt <= 0 || len(j.Sessions) > MaxSessions || len(j.Agents) > MaxAgents {
			return nil, fail("storage_unavailable")
		}
		seen[j.ID] = true
	}
	return rows, nil
}
func write(rows []Job) error {
	b, e := json.Marshal(rows)
	if e != nil || len(b) > maxBytes {
		return fail("storage_unavailable")
	}
	return state.WritePrivateFile(path(), b)
}
func transaction(fn func(*[]Job) error) error {
	lease, e := metadataLease()
	if e != nil {
		return fail("storage_unavailable")
	}
	defer lease.Close()
	rows, e := read()
	if e != nil {
		return fail("storage_unavailable")
	}
	return fn(&rows)
}

// A free worker lease is evidence that no registered worker can continue. Never
// restart it implicitly: a partial restore may already have sent startup commands.
func (s Store) reconcile(rows []Job) bool {
	changed := false
	for i := range rows {
		j := &rows[i]
		if Terminal(j.Phase) || s.now().Unix()-j.CreatedAt < 60 {
			continue
		}
		lease, e := state.TryLease("restore-worker-" + j.ID)
		if e != nil {
			continue
		}
		lease.Close()
		j.Phase = "interrupted"
		j.Code = "worker_interrupted"
		j.UpdatedAt = s.now().Unix()
		changed = true
	}
	return changed
}
func (s Store) Latest() (*Job, error) {
	var out *Job
	e := transaction(func(rows *[]Job) error {
		if s.reconcile(*rows) {
			if e := write(*rows); e != nil {
				return e
			}
		}
		if len(*rows) > 0 {
			j := (*rows)[len(*rows)-1]
			out = &j
		}
		return nil
	})
	return out, e
}
func (s Store) Get(id string) (*Job, error) {
	if !ValidID(id) {
		return nil, fail("invalid_request")
	}
	var out *Job
	e := transaction(func(rows *[]Job) error {
		if s.reconcile(*rows) {
			if e := write(*rows); e != nil {
				return e
			}
		}
		for _, j := range *rows {
			if j.ID == id {
				out = &j
				return nil
			}
		}
		return fail("not_found")
	})
	return out, e
}

// Start persists BEFORE launch. launch must only start a detached child; it must
// not wait for that child to claim (the metadata lease is still held here).
func (s Store) Start(id, revision string, preview func() (Plan, error), launch func(string) error) (*Job, error) {
	if !ValidID(id) || len(revision) != 64 {
		return nil, fail("invalid_request")
	}
	var out *Job
	e := transaction(func(rows *[]Job) error {
		for _, j := range *rows {
			if j.ID == id {
				if j.Revision != revision {
					return fail("request_changed")
				}
				out = &j
				return nil
			}
		}
		if !freshID(id, s.now()) {
			return fail("request_expired")
		}
		s.reconcile(*rows)
		for _, j := range *rows {
			if !Terminal(j.Phase) {
				out = &j
				return fail("restore_busy")
			}
		}
		plan, e := preview()
		if e != nil {
			return e
		}
		if plan.Revision != revision {
			return fail("plan_changed")
		}
		if len(plan.Sessions) == 0 {
			return fail("nothing_to_restore")
		}
		if len(plan.Sessions) > MaxSessions {
			return fail("plan_too_large")
		}
		kept := (*rows)[:0]
		for _, j := range *rows {
			if !Terminal(j.Phase) || s.now().Unix()-j.CreatedAt < 48*3600 {
				kept = append(kept, j)
			}
		}
		*rows = kept
		if len(*rows) >= MaxJobs {
			return fail("capacity")
		}
		j := Job{ID: id, Revision: revision, CreatedAt: s.now().Unix(), UpdatedAt: s.now().Unix(), Phase: "queued", Sessions: plan.Sessions}
		*rows = append(*rows, j)
		if e := write(*rows); e != nil {
			return fail("storage_unavailable")
		}
		out = &j
		if e := launch(id); e != nil {
			j.Phase = "failed"
			j.Code = "launch_failed"
			(*rows)[len(*rows)-1] = j
			out = &j
			if write(*rows) != nil {
				return fail("storage_unavailable")
			}
		}
		return nil
	})
	return out, e
}

// Claim is once-only, including when the prior worker has exited. A worker holds
// its kernel lease until all restore work and final publication have ended.
func (s Store) Claim(id string) (*Job, *state.Lease, error) {
	if !ValidID(id) {
		return nil, nil, fail("invalid_request")
	}
	lease, e := state.TryLease("restore-worker-" + id)
	if e != nil {
		return nil, nil, fail("already_claimed")
	}
	var out *Job
	e = transaction(func(rows *[]Job) error {
		for i := range *rows {
			j := &(*rows)[i]
			if j.ID != id {
				continue
			}
			if j.Phase != "queued" {
				return fail("already_claimed")
			}
			j.Phase = "preparing"
			j.UpdatedAt = s.now().Unix()
			out = j
			return write(*rows)
		}
		return fail("not_found")
	})
	if e != nil {
		lease.Close()
		return nil, nil, e
	}
	return out, lease, nil
}
func (s Store) Update(id, phase, code string, sessions []Session, agents []Result) error {
	if len(sessions) > MaxSessions || len(agents) > MaxAgents {
		return fail("result_too_large")
	}
	return transaction(func(rows *[]Job) error {
		for i := range *rows {
			j := &(*rows)[i]
			if j.ID != id {
				continue
			}
			if Terminal(j.Phase) {
				return fail("already_finished")
			}
			j.Phase = phase
			j.Code = code
			j.UpdatedAt = s.now().Unix()
			if sessions != nil {
				j.Sessions = sessions
			}
			if agents != nil {
				j.Agents = agents
			}
			return write(*rows)
		}
		return fmt.Errorf("restore job not found")
	})
}

func metadataLease() (*state.Lease, error) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		l, e := state.TryLease("restore-jobs")
		if !errors.Is(e, state.ErrLeaseHeld) || !time.Now().Before(deadline) {
			return l, e
		}
		time.Sleep(5 * time.Millisecond)
	}
}
