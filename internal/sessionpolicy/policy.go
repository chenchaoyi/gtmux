// Package sessionpolicy owns the explicit per-conversation permissions for
// verified desktop Codex sessions. Its state is local to this Mac, outside HQ
// archives: detecting a conversation never enrolls it for observation or learning.
package sessionpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Settings is additive to the radar contract only for verified desktop rows.
// Consent times fence off events and transcript content from prior intervals.
type Settings struct {
	HQ                bool  `json:"hq"`
	Notify            bool  `json:"notify"`
	Knowledge         bool  `json:"knowledge"`
	FollowSince       int64 `json:"follow_since,omitempty"`
	KnowledgeSince    int64 `json:"knowledge_since,omitempty"`
	Revision          int64 `json:"revision"`
	KnowledgeRevision int64 `json:"knowledge_revision,omitempty"`
}

type stored struct {
	SessionID string `json:"session_id"`
	Settings
}

var ErrConflict = errors.New("conversation settings changed; reload before saving")
var ErrUnverified = errors.New("not a verified desktop Codex conversation")

// Desktop requires the session's own explicit originator, not a cwd or app name.
func Desktop(id string) bool { return transcript.CodexClient(id) == "chatgpt_desktop" }

func path(id string) string {
	sum := sha256.Sum256([]byte("codex\x00" + id))
	return filepath.Join(state.Dir(), "session-follow", hex.EncodeToString(sum[:])+".json")
}

func read(id string) (Settings, error) {
	b, err := os.ReadFile(path(id))
	if os.IsNotExist(err) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var v stored
	if err = json.Unmarshal(b, &v); err != nil {
		return Settings{}, err
	}
	if v.SessionID != id || v.Revision < 0 || (v.HQ && v.FollowSince <= 0) || (v.Knowledge && v.KnowledgeSince <= 0) || (!v.HQ && (v.Notify || v.Knowledge)) {
		return Settings{}, errors.New("invalid conversation policy")
	}
	return v.Settings, nil
}

// Get fails closed on corrupt/unreadable permissions and leaves a diagnostic.
func Get(id string) Settings {
	v, err := read(id)
	if err != nil {
		diag.For("session-follow").Warn("policy.read.failed", "could not read conversation permissions; using status-only", "agent_session", id, "error", err)
	}
	return v
}

// Save uses a cross-process lock and revision check, then atomically replaces the
// one-session document. A stale screen cannot silently overwrite another device.
func Save(id string, next Settings) (Settings, error) { return saveAt(id, next, time.Now().Unix()) }
func saveAt(id string, next Settings, now int64) (Settings, error) {
	if !Desktop(id) {
		return Settings{}, ErrUnverified
	}
	file := path(id)
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return Settings{}, err
	}
	lock, err := os.OpenFile(file+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return Settings{}, err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return Settings{}, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck
	old, err := read(id)
	if err != nil {
		return Settings{}, fmt.Errorf("read conversation settings: %w", err)
	}
	if next.Revision != old.Revision {
		return Settings{}, ErrConflict
	}
	if !next.HQ {
		next.Notify, next.Knowledge = false, false
	}
	next.FollowSince, next.KnowledgeSince = old.FollowSince, old.KnowledgeSince
	next.KnowledgeRevision = old.KnowledgeRevision
	if next.HQ && !old.HQ {
		next.FollowSince = now + 1
	}
	if next.Knowledge && !old.Knowledge {
		next.KnowledgeSince = now + 1
		next.KnowledgeRevision = old.Revision + 1
	}
	if !next.HQ {
		next.FollowSince = 0
	}
	if !next.Knowledge {
		next.KnowledgeSince = 0
		next.KnowledgeRevision = 0
	}
	next.Revision = old.Revision + 1
	b, err := json.Marshal(stored{SessionID: id, Settings: next})
	if err != nil {
		return Settings{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(file), ".policy-*")
	if err != nil {
		return Settings{}, err
	}
	defer os.Remove(temp.Name())
	_, err = temp.Write(b)
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temp.Name(), file)
	}
	if err != nil {
		return Settings{}, err
	}
	return next, nil
}

// ObserveEvent is the common HQ debt/pull gate. Legacy events may resolve their
// originator by ID; unknown clients and all real tmux rows retain their behavior.
func ObserveEvent(pane, agent, id, client string, at int64) bool {
	if pane != "" || id == "" || (client != "chatgpt_desktop" && agent != "Codex" && agent != "codex") {
		return true
	}
	if client != "chatgpt_desktop" && !Desktop(id) {
		return true
	}
	s := Get(id)
	return s.HQ && at >= s.FollowSince
}

// LearnEvent is the retention boundary for desktop activity in the HQ feed.
func LearnEvent(pane, agent, id, client string, at int64) bool {
	if pane != "" || id == "" || (client != "chatgpt_desktop" && agent != "Codex" && agent != "codex") {
		return true
	}
	if client != "chatgpt_desktop" && !Desktop(id) {
		return true
	}
	p := Get(id)
	return p.HQ && p.Knowledge && at >= p.KnowledgeSince
}
