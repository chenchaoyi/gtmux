// Package sessionpolicy owns the explicit per-conversation permissions for
// verified external Codex sessions. Its state is local to this Mac, outside HQ
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
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Settings is additive to the radar contract only for verified external Codex rows.
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
var ErrUnverified = errors.New("not a verified Codex conversation")

// Desktop requires the session's own explicit originator, not a cwd or app name.
func Desktop(id string) bool { return transcript.CodexClient(id) == "chatgpt_desktop" }

func Eligible(id string) bool { _, ok := transcript.CodexIdentityFor(id); return ok }

func path(id string) string {
	sum := sha256.Sum256([]byte("codex\x00" + transcript.CodexHomeID() + "\x00" + id))
	return filepath.Join(state.Dir(), "session-follow", hex.EncodeToString(sum[:])+".json")
}

func read(id string) (Settings, error) {
	b, err := os.ReadFile(path(id))
	// Only the original default home may read pre-scope grants. A configured
	// alternate home with a copied thread ID cannot inherit them.
	if os.IsNotExist(err) && transcript.DefaultCodexHome() {
		sum := sha256.Sum256([]byte("codex\x00" + id))
		b, err = os.ReadFile(filepath.Join(state.Dir(), "session-follow", hex.EncodeToString(sum[:])+".json"))
	}
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
	if !Eligible(id) {
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
func ObserveEvent(pane, agent, id, client string, at int64, homes ...string) bool {
	if pane == "" && len(homes) > 0 && !ValidHome(homes[0]) {
		return false
	}
	if pane != "" || id == "" || (client != "chatgpt_desktop" && agent != "Codex" && agent != "codex") {
		return true
	}
	if !RequiresConsent(id, client) {
		return true
	}
	s := Get(id)
	return s.HQ && at >= s.FollowSince
}

// LearnEvent is the retention boundary for desktop activity in the HQ feed.
func LearnEvent(pane, agent, id, client string, at int64, homes ...string) bool {
	if pane == "" && len(homes) > 0 && !ValidHome(homes[0]) {
		return false
	}
	if pane != "" || id == "" || (client != "chatgpt_desktop" && agent != "Codex" && agent != "codex") {
		return true
	}
	if !RequiresConsent(id, client) {
		return true
	}
	p := Get(id)
	return p.HQ && p.Knowledge && at >= p.KnowledgeSince
}

// ValidHome binds a reader/settings request to this server's configured Codex home.
// An absent scope remains compatible with older clients; the caller's server is
// still the boundary. Explicit foreign scopes always fail before reading content.
func ValidHome(home string) bool { return home == "" || home == transcript.CodexHomeID() }

// ValidRequest keeps old ID-only clients from silently selecting a same-ID copy.
func ValidRequest(id, home string) bool {
	if !ValidHome(home) {
		return false
	}
	if home != "" {
		return true
	}
	for _, r := range native.Observations(id) {
		if r.Agent == "codex" && (r.HomeID != "" && r.HomeID != transcript.CodexHomeID() || r.HomeID == "" && !transcript.DefaultCodexHome()) {
			return false
		}
	}
	return true
}
