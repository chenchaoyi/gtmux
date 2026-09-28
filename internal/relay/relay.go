package relay

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

type Request struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Pane      string `json:"pane"`
	Session   string `json:"session"`
	PanePID   string `json:"pane_pid"`
	TaskID    string `json:"task_id,omitempty"`
	Body      string `json:"body"`
	For       string `json:"for"`
	Blocking  bool   `json:"blocking"`
	State     string `json:"state"`
	CreatedAt int64  `json:"created_at"`
	ClaimedAt int64  `json:"claimed_at,omitempty"`
	Reply     string `json:"reply,omitempty"`
	Delivered bool   `json:"delivered,omitempty"`
}

var ErrConflict = errors.New("request ID already belongs to different content")
var ErrState = errors.New("request is not in that state")
var ErrMissing = errors.New("request not found")

func dir() string { return filepath.Join(state.Dir(), "relay") }
func validID(id string) bool {
	if len(id) < 3 || len(id) > 80 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}
func path(id string) string { return filepath.Join(dir(), id+".json") }
func transaction(fn func() error) error {
	if err := os.MkdirAll(dir(), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir(), ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
func read(id string) (Request, error) {
	if !validID(id) {
		return Request{}, ErrMissing
	}
	b, err := os.ReadFile(path(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Request{}, ErrMissing
		}
		return Request{}, err
	}
	var r Request
	err = json.Unmarshal(b, &r)
	return r, err
}
func write(r Request) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir(), ".request-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path(r.ID)); err != nil {
		return err
	}
	d, err := os.Open(dir())
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func Put(r Request) (Request, bool, error) {
	if !validID(r.ID) || !((r.Kind == "report") || (r.Kind == "ask")) || strings.TrimSpace(r.Body) == "" || r.Pane == "" || r.Session == "" {
		return Request{}, false, errors.New("invalid request")
	}
	var out Request
	var fresh bool
	err := transaction(func() error {
		old, e := read(r.ID)
		if e == nil {
			if old.Kind != r.Kind || old.Pane != r.Pane || old.Session != r.Session || old.PanePID != r.PanePID || old.TaskID != r.TaskID || old.Body != r.Body || old.For != r.For || old.Blocking != r.Blocking {
				return ErrConflict
			}
			out = old
			return nil
		}
		if !errors.Is(e, ErrMissing) {
			return e
		}
		r.State = "pending"
		r.CreatedAt = time.Now().Unix()
		if e = write(r); e != nil {
			return e
		}
		out = r
		fresh = true
		return nil
	})
	return out, fresh, err
}
func Get(id string) (Request, error) { return read(id) }
func List() []Request {
	entries, _ := os.ReadDir(dir())
	var out []Request
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := read(strings.TrimSuffix(e.Name(), ".json"))
		if err == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out
}
func Claim(id string) (Request, error) {
	var out Request
	err := transaction(func() error {
		r, e := read(id)
		if e != nil {
			return e
		}
		if r.State != "pending" && !(r.State == "claimed" && time.Now().Unix()-r.ClaimedAt > 300) {
			return ErrState
		}
		r.State = "claimed"
		r.ClaimedAt = time.Now().Unix()
		if e = write(r); e == nil {
			out = r
		}
		return e
	})
	return out, err
}
func Resolve(id, reply string) (Request, error) {
	var out Request
	err := transaction(func() error {
		r, e := read(id)
		if e != nil {
			return e
		}
		if r.State == "replied" && r.Reply == reply {
			out = r
			return nil
		}
		if r.State != "claimed" {
			return ErrState
		}
		if r.For == "user" && strings.TrimSpace(reply) != "" {
			return errors.New("user-directed requests must be answered by the user in the source pane")
		}
		if strings.TrimSpace(reply) == "" {
			r.State = "closed"
		} else {
			r.State = "replied"
			r.Reply = reply
		}
		if e = write(r); e == nil {
			out = r
		}
		return e
	})
	return out, err
}
func MarkDelivered(id string) error {
	return transaction(func() error {
		r, e := read(id)
		if e != nil {
			return e
		}
		if r.State != "replied" {
			return ErrState
		}
		r.Delivered = true
		return write(r)
	})
}
