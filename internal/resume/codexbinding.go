package resume

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

const codexBindingTTL = 10 * time.Minute

// CodexBindingTarget identifies the live pane incarnation, not just its cwd.
type CodexBindingTarget struct {
	Pane string `json:"pane"`
	Loc  string `json:"loc"`
	Cwd  string `json:"cwd"`
	PID  int    `json:"pid"`
}

// CodexBindingIntent is a one-use witness of a specific task sent to a pane.
// No prompt text or credentials are stored here.
type CodexBindingIntent struct {
	CodexBindingTarget
	Token     string `json:"token"`
	SHA       string `json:"sha"`
	CreatedAt int64  `json:"createdAt"`
	Incumbent Record `json:"incumbent"`
}

func codexBindingDir() string { return filepath.Join(Dir(), "codex-binding") }

func codexBindingPath(token string) string { return filepath.Join(codexBindingDir(), token+".json") }

// LiveCodexBindingTarget reads identity and foreground command together. Callers
// must not populate a live target from a stale radar snapshot or inherited env.
func LiveCodexBindingTarget(pane string) (CodexBindingTarget, bool) {
	f := strings.Split(tmux.Display(pane, "#{pane_id}\t#{session_name}:#{window_index}.#{pane_index}\t#{pane_current_path}\t#{pane_pid}\t#{pane_current_command}"), "\t")
	if len(f) != 5 || f[0] != pane || f[4] != "codex" {
		return CodexBindingTarget{}, false
	}
	pid, _ := strconv.Atoi(f[3])
	return CodexBindingTarget{f[0], f[1], f[2], pid}, pid > 0 && f[1] != "" && f[2] != ""
}

func bindingSHA(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// PrepareCodexBinding adds an unguessable witness to an unbound pane's delivery.
func PrepareCodexBinding(target CodexBindingTarget, prompt string, now int64) (string, error) {
	if target.Pane == "" || target.Loc == "" || target.Cwd == "" || target.PID <= 0 {
		return "", errors.New("incomplete Codex binding target")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(nonce[:])
	wire := prompt + "\n\n" + transcript.SessionBindingPrefix + token
	previous, _ := Load(target.Loc)
	intent := CodexBindingIntent{target, token, bindingSHA(wire), now, previous}
	b, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(codexBindingDir(), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(codexBindingDir(), ".intent-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(b)
	closeErr := tmp.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := os.Rename(tmp.Name(), codexBindingPath(token)); err != nil {
		return "", err
	}
	return wire, nil
}

// CodexBindingForPrompt accepts only the complete prepared wire payload.
func CodexBindingForPrompt(prompt string, now int64) (CodexBindingIntent, bool) {
	token := transcript.SessionBindingToken(prompt)
	if token == "" {
		return CodexBindingIntent{}, false
	}
	b, err := os.ReadFile(codexBindingPath(token))
	var intent CodexBindingIntent
	if err != nil || json.Unmarshal(b, &intent) != nil || intent.Token != token ||
		now < intent.CreatedAt || now-intent.CreatedAt > int64(codexBindingTTL/time.Second) || intent.SHA != bindingSHA(prompt) {
		return CodexBindingIntent{}, false
	}
	return intent, true
}

// CodexBindingForSession recovers the same witness when a hook omitted its ID.
func CodexBindingForSession(sid string, now int64) (CodexBindingIntent, bool) {
	entries, _ := os.ReadDir(codexBindingDir())
	pending := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(codexBindingDir(), entry.Name()))
		var intent CodexBindingIntent
		if err == nil && json.Unmarshal(b, &intent) == nil {
			if now-intent.CreatedAt > int64(codexBindingTTL/time.Second) {
				_ = os.Remove(filepath.Join(codexBindingDir(), entry.Name()))
			} else if now >= intent.CreatedAt {
				pending = true
			}
		}
	}
	if !pending || transcript.CodexClient(sid) != "terminal" {
		return CodexBindingIntent{}, false
	}
	for _, prompt := range transcript.CodexBindingPrompts(sid) {
		if intent, ok := CodexBindingForPrompt(prompt, now); ok {
			return intent, true
		}
	}
	return CodexBindingIntent{}, false
}

// CompleteCodexBinding refuses stale targets and never overwrites a newer owner.
// Consumption follows Save, so a failed write can be retried by the next poll.
func CompleteCodexBinding(intent CodexBindingIntent, live CodexBindingTarget, sid string, now int64) error {
	if sid == "" || transcript.CodexClient(sid) != "terminal" || live.Pane != intent.Pane ||
		live.Loc != intent.Loc || live.PID != intent.PID || live.PID <= 0 || !hqpane.SameDir(live.Cwd, intent.Cwd) ||
		now < intent.CreatedAt || now-intent.CreatedAt > int64(codexBindingTTL/time.Second) {
		return errors.New("codex binding witness does not identify this live target")
	}
	// Serialize bootstrap consumers at this location. A crash leaves only a
	// short-lived lock; no conversation or existing resume record is removed.
	lockPath := fileFor(live.Loc) + ".binding-lock"
	if info, err := os.Stat(lockPath); err == nil && time.Since(info.ModTime()) > time.Minute {
		_ = os.Remove(lockPath)
	}
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	lock.Close()
	defer os.Remove(lockPath)
	b, err := os.ReadFile(codexBindingPath(intent.Token))
	var saved CodexBindingIntent
	if err != nil || json.Unmarshal(b, &saved) != nil || saved != intent {
		return errors.New("codex binding intent is missing or changed")
	}
	current, _ := Load(live.Loc)
	if current != intent.Incumbent && (current.Agent != "codex" || current.SessionID != sid) {
		return errors.New("codex binding target acquired another owner")
	}
	if err := Save(live.Loc, Record{Agent: "codex", SessionID: sid, Cwd: live.Cwd, UpdatedAt: now}); err != nil {
		return err
	}
	return os.Remove(codexBindingPath(intent.Token))
}
