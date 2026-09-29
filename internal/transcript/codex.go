package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Codex logs live at $CODEX_HOME/sessions/YYYY/MM/DD/rollout-<ts>-<sessionId>.jsonl
// (default CODEX_HOME=~/.codex), with archived_sessions/ as a fallback. Each line
// is {timestamp, type, payload}.
//
// Observed schema contract (sanitized examples live in testdata/codex-current.jsonl):
//   - user prompts appear as response_item message/role=user/input_text in Codex 0.157+;
//     older rollouts may use event_msg/user_message. Some versions emit both.
//   - tool calls live in `response_item` as function_call (name + arguments as a
//     JSON *string*); these interleave with event_msg in file order, so a single
//     linear pass keeps steps in the right place.
//   - injected AGENTS.md and environment context is filtered; assistant text and final
//     answers arrive in event_msg/agent_message and event_msg/task_complete.
//   - response_item/function_call carries tool calls; unknown records, including
//     token_count, do not affect turn parsing.

type codexLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Message string `json:"message"` // event_msg user_message/agent_message
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"` // response_item message (Codex 0.157+ user input)
	LastAgentMessage string `json:"last_agent_message"` // event_msg task_complete
	Name             string `json:"name"`               // response_item function_call
	Arguments        string `json:"arguments"`          // response_item function_call (JSON string)
	SessionID        string `json:"session_id"`         // session_meta (first line)
	ID               string `json:"id"`                 // current session_meta
	Originator       string `json:"originator"`         // codex-tui | codex_work_desktop
	Cwd              string `json:"cwd"`                // session_meta (first line)
}

// CodexHome is where Codex keeps its sessions, for the packages that need to
// read them without re-deriving the layout (limits reads rate limits out of the
// same rollouts this package reads transcripts from).
func CodexHome() string { return codexHome() }

func codexHome() string {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return h
	}
	return filepath.Join(state.Home(), ".codex")
}

func codexLogPath(sessionID string) string {
	paths := codexLogPaths(sessionID)
	if len(paths) == 0 {
		return ""
	}
	// Raw-log consumers (notably usage's byte-offset counter) must keep their
	// existing file identity when Codex starts a continuation. Readers that
	// need the whole conversation use codexLogPaths instead.
	return paths[0]
}

// A desktop Codex conversation can continue in a second rollout named
// rollout-<timestamp>-<sessionID>_<instanceID>.jsonl. The suffix is not a new
// conversation: session_meta.id still names the original conversation. Match
// that ID before accepting a suffixed file, so a similarly named session cannot
// lend its completion or messages to this one.
func codexLogPaths(sessionID string) []string {
	if sessionID == "" || strings.ContainsAny(sessionID, `*/?[]\\`) {
		return nil
	}
	home := codexHome()
	var paths []string
	for _, dir := range []string{filepath.Join(home, "sessions", "*", "*", "*"), filepath.Join(home, "archived_sessions")} {
		for _, suffix := range []string{".jsonl", "_*.jsonl"} {
			matches, _ := filepath.Glob(filepath.Join(dir, "rollout-*-"+sessionID+suffix))
			for _, path := range matches {
				meta := readCodexSessionMeta(path)
				id := meta.ID
				if id == "" {
					id = meta.SessionID // legacy Codex session_meta
				}
				if id != "" && id != sessionID || id == "" && suffix != ".jsonl" {
					continue
				}
				paths = append(paths, path)
			}
		}
	}
	// The rollout name starts with its creation timestamp; sorting by basename
	// keeps resumed files in conversation order across day directories/archives.
	sort.SliceStable(paths, func(i, j int) bool {
		return filepath.Base(paths[i]) < filepath.Base(paths[j])
	})
	// An archived copy can briefly coexist with the live file. Keep the live
	// one (collected first) so Chat never duplicates its turns.
	unique := paths[:0]
	for _, path := range paths {
		if len(unique) == 0 || filepath.Base(unique[len(unique)-1]) != filepath.Base(path) {
			unique = append(unique, path)
		}
	}
	return unique
}

// CodexClient identifies the client that created a conversation from its own
// session_meta. The rollout's `source` can be "vscode" for both the desktop app
// and TUI, so only the explicit originator is used. Unknown stays unknown.
func CodexClient(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	path := codexLogPath(sessionID)
	if path == "" {
		return ""
	}
	meta := readCodexSessionMeta(path)
	if meta.ID != sessionID && meta.SessionID != sessionID {
		return ""
	}
	switch meta.Originator {
	case "codex_work_desktop":
		return "chatgpt_desktop"
	case "codex-tui":
		return "terminal"
	default:
		return ""
	}
}

// CodexSessionForCwd finds a unique Codex session started in
// `cwd` and returns its session id. Codex's `notify` payload (unlike Claude's
// hooks) carries NO conversation id, so the resume/transcript machinery would
// otherwise have nothing to key on — we derive it from the on-disk rollout whose
// session_meta.cwd matches the pane's dir. If more than one session matches,
// recency alone is not identity and the function returns ok=false.
func CodexSessionForCwd(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	var files []string
	for _, pat := range []string{
		filepath.Join(codexHome(), "sessions", "*", "*", "*", "rollout-*.jsonl"),
		filepath.Join(codexHome(), "archived_sessions", "rollout-*.jsonl"),
	} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	var found string
	for _, f := range files {
		if sid, fcwd := codexSessionMeta(f); sid != "" && fcwd == cwd {
			if found != "" && found != sid {
				return "", false
			}
			found = sid
		}
	}
	return found, found != ""
}

// codexSessionMeta reads a rollout's first line (the session_meta record) and
// returns its session id + cwd. Cheap: only the first line is read.
func codexSessionMeta(path string) (sessionID, cwd string) {
	p := readCodexSessionMeta(path)
	// Current Codex writes session_meta.id; session_id is the legacy spelling.
	// Using only session_id left new TUI sessions unbound to their panes, so an
	// earlier agent's resume record could keep owning the location indefinitely.
	if p.ID != "" {
		return p.ID, p.Cwd
	}
	return p.SessionID, p.Cwd
}

func readCodexSessionMeta(path string) codexPayload {
	f, err := os.Open(path)
	if err != nil {
		return codexPayload{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20) // session_meta can be large (base_instructions)
	if !sc.Scan() {
		return codexPayload{}
	}
	var e codexLine
	if json.Unmarshal(sc.Bytes(), &e) != nil || e.Type != "session_meta" {
		return codexPayload{}
	}
	var p codexPayload
	if json.Unmarshal(e.Payload, &p) != nil {
		return codexPayload{}
	}
	return p
}

// CodexLastTurnBoundary returns the latest task start, completion, or abort
// from every matching rollout. An end event proves the turn ended even when a
// Stop hook was delivered without a usable pane identity. Reading only the tail
// keeps this bounded for long-lived sessions; an oversized final record simply
// leaves the boundary unknown.
func CodexLastTurnBoundary(sessionID string) (string, time.Time) {
	var latestKind string
	var latestAt time.Time
	for _, path := range codexLogPaths(sessionID) {
		kind, at := codexLastTurnBoundaryIn(path)
		if at.After(latestAt) {
			latestKind, latestAt = kind, at
		}
	}
	return latestKind, latestAt
}

func codexLastTurnBoundaryIn(path string) (string, time.Time) {
	f, err := os.Open(path)
	if err != nil {
		return "", time.Time{}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", time.Time{}
	}
	const tailBytes int64 = 256 << 10
	start := max(int64(0), info.Size()-tailBytes)
	buf := make([]byte, info.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return "", time.Time{}
	}
	if start > 0 {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			return "", time.Time{}
		}
		buf = buf[i+1:]
	}
	lines := bytes.Split(buf, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		var line codexLine
		if json.Unmarshal(lines[i], &line) != nil || line.Type != "event_msg" {
			continue
		}
		var payload struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(line.Payload, &payload) != nil ||
			(payload.Type != "task_started" && payload.Type != "task_complete" && payload.Type != "turn_aborted") {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, line.Timestamp)
		if err == nil {
			return payload.Type, at
		}
	}
	return "", time.Time{}
}

// codexStep folds one Codex log line into the parse state: event_msg user_message
// opens a turn; agent_message / task_complete set the (latest/authoritative)
// reply; response_item function_call adds a tool step.
func codexStep(line string, st *parseState) {
	var e codexLine
	if json.Unmarshal([]byte(line), &e) != nil || len(e.Payload) == 0 {
		return
	}
	var p codexPayload
	if json.Unmarshal(e.Payload, &p) != nil {
		return
	}
	switch e.Type {
	case "event_msg":
		switch p.Type {
		case "user_message":
			if prompt, ok := codexUserPrompt(p.Message); ok {
				codexOpenPrompt(st, prompt, e.Timestamp)
			}
		case "agent_message":
			if msg := strings.TrimSpace(p.Message); msg != "" {
				st.addText(msg) // each agent message starts a new bubble
			}
		case "task_complete":
			// authoritative final reply — append it unless an agent_message already
			// carried it (avoid duplicating the closing paragraph).
			if fin := strings.TrimSpace(p.LastAgentMessage); fin != "" && !lastSegmentText(st, fin) {
				st.addText(fin)
			}
		}
	case "response_item":
		if p.Type == "message" && p.Role == "user" {
			var parts []string
			for _, block := range p.Content {
				if block.Type == "input_text" && strings.TrimSpace(block.Text) != "" {
					parts = append(parts, block.Text)
				}
			}
			if prompt, ok := codexUserPrompt(strings.Join(parts, "\n")); ok {
				codexOpenPrompt(st, prompt, e.Timestamp)
			}
		} else if p.Type == "function_call" && p.Name != "" {
			st.addSteps([]Step{{Kind: "tool", Title: codexToolName(p.Name), Detail: codexToolDetail(p.Arguments)}})
		}
	}
}

func codexUserPrompt(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	// Codex writes the initial repository instructions and environment snapshot as
	// role=user response_items. They were supplied by the harness, not typed in chat.
	if strings.HasPrefix(s, "# AGENTS.md instructions") || strings.HasPrefix(s, "<environment_context>") {
		return "", false
	}
	return CleanUserPrompt(s)
}

// Some rollouts carry both the response_item and event_msg form of the same
// user input. They are adjacent before any reply; keep one prompt bubble.
func codexOpenPrompt(st *parseState, prompt, timestamp string) {
	if st.cur != nil && st.cur.Prompt == prompt && len(st.cur.Segments) == 0 {
		return
	}
	st.open(prompt, timestamp)
}

// codexToolName tidies a raw function name (exec_command → "exec") for display.
func codexToolName(name string) string {
	switch name {
	case "exec_command", "shell", "local_shell":
		return "exec"
	case "apply_patch":
		return "patch"
	}
	return name
}

// codexToolDetail pulls a short summary out of a function_call's JSON-string args
// (cmd / command / path / workdir).
func codexToolDetail(args string) string {
	if strings.TrimSpace(args) == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(args), &m) != nil {
		return clip(args, 80)
	}
	for _, k := range []string{"cmd", "command", "file_path", "path", "query", "workdir"} {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return clip(t, 80)
				}
			case []any:
				var parts []string
				for _, x := range t {
					if s, ok := x.(string); ok {
						parts = append(parts, s)
					}
				}
				if len(parts) > 0 {
					return clip(strings.Join(parts, " "), 80)
				}
			}
		}
	}
	return ""
}
