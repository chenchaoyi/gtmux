// Package dshfeed reconciles public messages from verified terminal conversations.
// It never turns a saved Web history into a pane attachment or input capability.
package dshfeed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

type progress struct {
	Version         int    `json:"version"`
	SourceSeq       int64  `json:"source_seq"`
	ReviewedThrough int64  `json:"reviewed_through"`
	SourceKey       string `json:"source_key"`
	SourceSize      int64  `json:"source_size"`
	Generation      int64  `json:"generation"`
}

func progressPath(home, id string) string {
	key := sha256.Sum256([]byte(home + "\x00" + id))
	return filepath.Join(state.Dir(), "conversation-messages", hex.EncodeToString(key[:16])+".json")
}
func readProgress(home, id string) (progress, error) {
	p := progress{Version: 1, SourceSeq: -1, ReviewedThrough: -1}
	err := agentclient.ReadOwnedJSON(progressPath(home, id), &p, 8192)
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil || p.Version != 1 || p.SourceSeq < -1 || p.ReviewedThrough < -1 {
		return p, errors.New("invalid message progress")
	}
	return p, nil
}
func saveProgress(home, id string, p progress) error {
	path := progressPath(home, id)
	if err := state.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := state.CreateTemp(filepath.Dir(path), ".progress-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(mustJSON(p)); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

type reader struct {
	cursor    dsh.Cursor
	used      time.Time
	lastError time.Time
}
type Reconciler struct {
	mu      sync.Mutex
	readers map[string]*reader
}

var Default = &Reconciler{}
var scheduled atomic.Bool

// Schedule permits one bounded worker; slow filesystem work never delays the HQ
// wake drain. No goroutine or reader is created per transcript HTTP request.
func Schedule() {
	if !scheduled.CompareAndSwap(false, true) {
		return
	}
	go func() { defer scheduled.Store(false); Default.Tick(agentclient.All()) }()
}

var errBudget = errors.New("message tick budget")

func (x *Reconciler) Tick(attachments []agentclient.Attachment) {
	if !x.mu.TryLock() {
		return
	}
	defer x.mu.Unlock()
	lease, err := state.TryLease("conversation-messages")
	if err != nil {
		return
	}
	defer lease.Close()
	if x.readers == nil {
		x.readers = map[string]*reader{}
	}
	for i, a := range attachments {
		if i >= 16 {
			break
		}
		if a.Agent != "dsh" || a.HomeID != dsh.HomeID() {
			continue
		}
		session, err := dsh.Sessions.Resolve(a.SessionID)
		if err != nil || session.Origin == "subagent" {
			continue
		}
		key := a.HomeID + "\x00" + a.SessionID
		r := x.readers[key]
		if r == nil {
			if len(x.readers) >= 16 {
				var oldest string
				var at time.Time
				for k, v := range x.readers {
					if oldest == "" || v.used.Before(at) {
						oldest, at = k, v.used
					}
				}
				delete(x.readers, oldest)
			}
			r = &reader{}
			x.readers[key] = r
		}
		r.used = time.Now()
		p, err := readProgress(a.HomeID, a.SessionID)
		if err != nil {
			x.gap(a, r, "message progress is unreadable")
			continue
		}
		fi, err := os.Stat(session.Path)
		if err != nil {
			x.gap(a, r, "message source is unavailable")
			continue
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		source := fmt.Sprintf("%s:%d:%d", session.Path, st.Dev, st.Ino)
		changed := p.SourceKey != "" && (p.SourceKey != source || fi.Size() < p.SourceSize)
		if changed {
			x.gap(a, r, "message source was replaced or truncated; review coverage reset")
			p = progress{Version: 1, SourceSeq: -1, ReviewedThrough: -1}
			r.cursor = dsh.Cursor{}
		}
		p.SourceKey, p.SourceSize = source, fi.Size()
		records, public := 0, 0
		hadCursor := r.cursor.File != nil
		err = dsh.Scan(session.Path, &r.cursor, func() {
			if hadCursor && !changed {
				x.gap(a, r, "message source was rewritten; review coverage reset")
				p.SourceSeq = -1
				p.ReviewedThrough = -1
				p.Generation = time.Now().UnixNano()
			}
		}, func(line []byte) error {
			var e dsh.Event
			if err := json.Unmarshal(line, &e); err != nil {
				return err
			}
			if e.Type == "session" || e.Seq <= p.SourceSeq {
				return nil
			}
			if records >= 4096 || public >= 100 {
				return errBudget
			}
			records++
			m, visible, err := dsh.PublicMessage(e)
			if err != nil {
				return err
			}
			if visible && m.Time >= a.StartedAt && m.Time >= a.BindingEpoch {
				sum := sha256.Sum256([]byte(key + "\x00" + source + fmt.Sprintf(":%d:%d", p.Generation, m.Seq)))
				messageID := hex.EncodeToString(sum[:16])
				if !events.PublicMessages.Contains(messageID) {
					summary := []rune(strings.TrimSpace(m.Text))
					if len(summary) > 240 {
						summary = summary[:240]
					}
					if err := events.AppendChecked(events.Record{Ts: time.Now().Unix(), Event: "ConversationMessage", Pane: a.Pane, Agent: "DeepSeek Harness", AgentKey: "dsh", HomeID: a.HomeID, AgentSession: a.SessionID, MessageID: messageID, MessageSeq: m.Seq, MessageRole: m.Role, Summary: string(summary)}); err != nil {
						return err
					}
				}
				public++
			}
			p.SourceSeq = e.Seq
			return nil
		})
		if err != nil && !errors.Is(err, errBudget) {
			x.gap(a, r, "public message source could not be reconciled")
		}
		if err := saveProgress(a.HomeID, a.SessionID, p); err != nil {
			x.gap(a, r, "message progress could not be saved")
		}
	}
}
func (x *Reconciler) gap(a agentclient.Attachment, r *reader, reason string) {
	if time.Since(r.lastError) < time.Minute {
		return
	}
	r.lastError = time.Now()
	events.Append(events.Record{Ts: time.Now().Unix(), Event: "ConversationGap", Pane: a.Pane, Agent: "DeepSeek Harness", AgentKey: "dsh", HomeID: a.HomeID, AgentSession: a.SessionID, Summary: reason, Severity: "important"})
}
func Reviewed(home, id string) (int64, error) {
	p, err := readProgress(home, id)
	return p.ReviewedThrough, err
}

// Review is an explicit local declaration, separate from reading messages or
// consuming the HQ event stream. It never grants follow, input or notification.
func Review(home, id string, through int64, page dsh.MessagePage) error {
	found := false
	for _, m := range page.Messages {
		if m.Seq == through {
			found = true
		}
	}
	session, resolveErr := dsh.Sessions.Resolve(id)
	if !found || resolveErr != nil || session.Revision != page.Revision || home != dsh.HomeID() || through < 0 || through > page.NextSeq || through > page.SourceSeq {
		return errors.New("review cursor exceeds the returned message page")
	}
	lease, err := state.TryLease("conversation-messages")
	if err != nil {
		return err
	}
	defer lease.Close()
	p, err := readProgress(home, id)
	if err != nil {
		return err
	}
	if through > p.ReviewedThrough {
		p.ReviewedThrough = through
	}
	return saveProgress(home, id, p)
}
