package agentprofile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/state"
)

type observation struct {
	Agent, Session, Home string
	Facts                []Fact
	Chain                []Process
	ChainAt              int64
	EffortResetAt        int64 `json:"effort_reset_at,omitempty"`
}

func observationPath(agent, session, home string) string {
	if home == "" {
		m, _ := agents.For(agent)
		config := ""
		if m.InstructionsEnv != "" {
			config = os.Getenv(m.InstructionsEnv)
		}
		switch agent {
		case "claude":
			config = os.Getenv("CLAUDE_CONFIG_DIR")
		case "cursor":
			config = os.Getenv("CURSOR_HOME")
		case "gemini":
			config = os.Getenv("GEMINI_CLI_HOME")
		}
		home = state.Home() + "\x00" + config
	}

	h := sha256.Sum256([]byte(agent + "\x00" + home + "\x00" + session))
	return filepath.Join(state.Dir(), "participant-profiles", hex.EncodeToString(h[:])+".json")
}

func loadObservation(agent, session, home string) observation {
	legacy := observationPath(agent, session, home)
	return loadObservations(legacy, versionedObservationPath(legacy), agent, session, home)
}

func versionedObservationPath(legacy string) string {
	return filepath.Join(filepath.Dir(legacy), "v2", filepath.Base(legacy))
}

func readObservation(path, agent, session, home string) observation {
	var v observation
	f, e := os.Open(path)
	if e != nil {
		return v
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil || !s.Mode().IsRegular() || s.Size() > 64<<10 {
		return v
	}
	if json.NewDecoder(f).Decode(&v) != nil || v.Agent != agent || v.Session != session || v.Home != home {
		return observation{}
	}
	return v
}

func modelOption(key string) bool {
	switch key {
	case "effort", "variant", "thinking", "thinking_level", "thinking_budget", "context", "model_alias":
		return true
	}
	return false
}

func observationFact(facts []Fact, key string) Fact {
	for _, f := range facts {
		if f.Key == key {
			return f
		}
	}
	return Fact{}
}

func loadObservations(legacyPath, currentPath, agent, session, home string) observation {
	old := readObservation(legacyPath, agent, session, home)
	current := readObservation(currentPath, agent, session, home)
	if current.Session == "" {
		return old
	}
	if old.Session == "" {
		return current
	}
	// Model-dependent options belong to the selected model. When both snapshots
	// agree on it, an effort-only hook can still update the effort independently.
	oldModel, currentModel := observationFact(old.Facts, "model"), observationFact(current.Facts, "model")
	sameModel := oldModel.Value == currentModel.Value
	preferOldModel := oldModel.At > currentModel.At
	oldPermission := observationFact(old.Facts, "permission").At > observationFact(current.Facts, "permission").At
	var merged Profile
	for i, snapshot := range []observation{old, current} {
		isOld := i == 0
		for _, f := range snapshot.Facts {
			fromSelectedModel := preferOldModel == isOld || (f.Key == "effort" && sameModel)
			if modelOption(f.Key) && !fromSelectedModel || f.Key == "mode" && oldPermission != isOld {
				continue
			}
			// A legacy model refresh can carry an older effort without observing it.
			if isOld && f.Key == "effort" && current.EffortResetAt > 0 && f.At <= current.EffortResetAt {
				continue
			}
			if f.At >= observationFact(merged.Facts, f.Key).At {
				merged.put(f)
			}
		}
	}
	current.Facts = merged.Facts
	if old.ChainAt > current.ChainAt {
		current.Chain, current.ChainAt = old.Chain, old.ChainAt
	}
	return current
}

// Observe is called before hook detachment, so ancestry still describes the origin.
// It is best-effort and never influences lifecycle, receipts, or permissions.
func Observe(agent, session, home, event string, raw []byte, ancestry bool) {
	if _, ok := agents.For(agent); !ok || session == "" || len(session) > 256 {
		return
	}
	legacy := observationPath(agent, session, home)
	path := versionedObservationPath(legacy)
	// Do the potentially slow process inspection outside the shared lock.
	probeAt := time.Now().UnixMilli()
	var chain []Process
	if ancestry && (event == "SessionStart" || event == "UserPromptSubmit" || event == "Resumed") {
		if probeAt-loadObservations(legacy, path, agent, session, home).ChainAt > 60_000 {
			chain = observeChain()
		}
	}
	lease, err := state.TryLeaseIn(filepath.Join(filepath.Dir(path), "locks"), filepath.Base(path)[:2])
	if err != nil {
		return
	}
	defer lease.Close()
	at := time.Now().UnixMilli()
	facts := HookFacts(agent, raw, at)
	v := loadObservations(legacy, path, agent, session, home)
	p := New(agent, session, false)
	p.Facts = v.Facts
	for _, f := range facts {
		if f.Key == "model" {
			if old := observationFact(p.Facts, "model"); old.Value != "" && old.Value != f.Value {
				p.Facts = slices.DeleteFunc(p.Facts, func(option Fact) bool { return modelOption(option.Key) })
				v.EffortResetAt = max(v.EffortResetAt, f.At)
			}
		}
		if f.Key == "effort" && f.Value == "" {
			v.EffortResetAt = max(v.EffortResetAt, f.At)
		}
		p.put(f)
	}
	if agent == "claude" {
		for _, f := range facts {
			if f.Key == "permission" && f.Value != "plan" {
				p.put(Fact{Key: "mode"})
			}
		}
	}
	if len(chain) > 0 && probeAt-v.ChainAt > 60_000 {
		v.Chain, v.ChainAt = chain, probeAt
		for _, f := range packageFacts(agent, chain, probeAt) {
			p.put(f)
		}
	}
	if len(p.Facts) == 0 && len(v.Chain) == 0 && v.EffortResetAt == 0 {
		return
	}
	v.Agent, v.Session, v.Home, v.Facts = agent, session, home, p.Facts
	b, err := json.Marshal(v)
	if err != nil || len(b) > 64<<10 {
		return
	}
	f, err := state.CreateTemp(filepath.Dir(path), ".profile-*")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	_ = f.Chmod(0600)
	_, err = f.Write(b)
	closeErr := f.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(name, path)
	}
}

func Current(p *Profile, home string) {
	v := loadObservation(p.Agent, p.SessionID, home)
	loggedModel := observationFact(p.Facts, "model")
	hookModel := observationFact(v.Facts, "model")
	for _, f := range v.Facts {
		// A later native log record wins over an older hook snapshot. Dependent
		// options from a different model cannot fill holes in its new defaults.
		if loggedModel.Value != "" && hookModel.Value != loggedModel.Value && loggedModel.At > hookModel.At && modelOption(f.Key) {
			continue
		}
		if f.At >= observationFact(p.Facts, f.Key).At {
			p.put(f)
		}
	}
	p.Chain, p.ChainAt = v.Chain, v.ChainAt
}

type limitedOutput struct{ b []byte }

func (w *limitedOutput) Write(b []byte) (int, error) {
	if len(w.b)+len(b) > 8192 {
		return 0, errors.New("process metadata exceeds limit")
	}
	w.b = append(w.b, b...)
	return len(b), nil
}
func ps(ctx context.Context, pid int, format string) string {
	c := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", format)
	var w limitedOutput
	c.Stdout = &w
	if c.Run() != nil {
		return ""
	}
	return strings.TrimSpace(string(w.b))
}

type identity struct {
	pid, parent int
	start, path string
}

func inspect(ctx context.Context, pid int) identity {
	s := strings.Fields(ps(ctx, pid, "ppid=,lstart=,comm="))
	if len(s) < 7 {
		return identity{}
	}
	pp, _ := strconv.Atoi(s[0])
	return identity{pid, pp, strings.Join(s[1:6], " "), strings.Join(s[6:], " ")}
}

func observeChain() []Process {
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	var ids []identity
	var chain []Process
	seen := map[int]bool{}
	pid := os.Getppid()
	for i := 0; i < 8 && pid > 1 && !seen[pid]; i++ {
		seen[pid] = true
		p := inspect(ctx, pid)
		if p.pid == 0 || p.parent == pid {
			return nil
		}
		name := filepath.Base(p.path)
		if name == "launchd" || name == "systemd" || name == "sshd" {
			break
		}
		ids = append(ids, p)
		args := sanitizeArgs(name, ps(ctx, pid, "args="))
		chain = append(chain, Process{Name: name, Path: p.path, Arguments: args})
		if name == "tmux" {
			break
		}
		pid = p.parent
	}
	for _, p := range ids {
		if inspect(ctx, p.pid) != p {
			return nil
		}
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// Arguments are an allowlist, not a secret blacklist. Arbitrary positional text,
// shell bodies, environment assignments and unknown option values never persist.
func sanitizeArgs(name, raw string) []string {
	// ps renders argv as a flat string. It cannot distinguish an option from text
	// inside a prompt, so no option/value pairs are reconstructed from that string.
	if name != "node" {
		return nil
	}
	parts := strings.Fields(raw)
	if len(parts) < 2 {
		return nil
	}
	script := parts[1]
	if filepath.IsAbs(script) && strings.HasSuffix(script, ".js") &&
		(strings.Contains(script, "/@deepseek-ai/dsh/") || strings.Contains(script, "/@deepseek-harness-tui/dsh-tui/")) {
		return []string{script}
	}
	return nil
}

func packageFacts(agent string, chain []Process, at int64) []Fact {
	var out []Fact
	if agent != "dsh" {
		return out
	}
	for _, process := range chain {
		for _, script := range process.Arguments {
			dir := filepath.Dir(script)
			for depth := 0; depth < 6; depth++ {
				path := filepath.Join(dir, "package.json")
				if fi, e := os.Stat(path); e == nil && fi.Mode().IsRegular() && fi.Size() < 256<<10 {
					var value struct {
						Name    string `json:"name"`
						Version string `json:"version"`
					}
					if b, e := os.ReadFile(path); e == nil && json.Unmarshal(b, &value) == nil {
						key := ""
						switch value.Name {
						case "@deepseek-ai/dsh":
							key = "version"
						case "@deepseek-harness-tui/dsh-tui":
							key = "client_version"
						}
						if key != "" {
							f := fact(key, value.Version, "runtime_package", at)
							if f.Value != "" {
								out = append(out, f)
							}
							break
						}
					}
				}
				next := filepath.Dir(dir)
				if next == dir {
					break
				}
				dir = next
			}
		}
	}
	return out
}
