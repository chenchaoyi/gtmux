package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Claude Code logs live at ~/.claude/projects/<cwd-slug>/<sessionId>.jsonl, one
// JSON event per line. We glob by sessionId so we never reconstruct the cwd-slug
// encoding. Each line has a top-level `type`; the conversation is reconstructed
// linearly (a full parentUuid DAG is overkill for a recent-history glance):
//   user prompt → assistant text/tool_use* … → final assistant text.
//
// Schema notes (from the 2026-06 prior-art survey, see docs/design/RESEARCH-…):
//   - message.content is `string | block[]`; user prompts are often a bare string.
//   - tool results come back as `user` events whose content is tool_result blocks
//     — those are NOT new prompts, so they don't open a turn.
//   - the FINAL assistant message has stop_reason=="end_turn" (tool_use is mid-turn);
//     we approximate by letting the latest assistant text win.
//   - isMeta (slash-command echo) and isSidechain (sub-agent) entries are skipped
//     so they don't pollute the main timeline.
//   - unknown `type` values (ai-title/queue-operation/attachment/…) are ignored.

type claudeLine struct {
	Type        string `json:"type"`
	IsMeta      bool   `json:"isMeta"`
	IsSidechain bool   `json:"isSidechain"`
	// isApiErrorMessage marks an assistant entry that IS an API/tool error (Claude
	// Code writes one per failed attempt, e.g. "API Error: Unable to connect to
	// API"). Only meaningful as the LAST message — mid-turn ones usually recovered.
	IsAPIError bool           `json:"isApiErrorMessage"`
	Timestamp  string         `json:"timestamp"`
	Message    *claudeMessage `json:"message"`
}

type claudeMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	StopReason string          `json:"stop_reason"`
}

type claudeBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"` // tool_result payload (string | block[])
}

func claudeLogPath(sessionID string) string {
	matches, _ := filepath.Glob(filepath.Join(claudeProjectsDir(), "*", sessionID+".jsonl"))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func claudeProjectsDir() string {
	return filepath.Join(os.Getenv("HOME"), ".claude", "projects")
}

// claudeStep folds one Claude log line into the parse state: a real user prompt
// opens a turn; tool_result-only user events are skipped; assistant text/tool_use
// extend the open turn (latest text wins → the final reply).
func claudeStep(line string, st *parseState) {
	var e claudeLine
	if json.Unmarshal([]byte(line), &e) != nil || e.Message == nil || e.IsSidechain {
		return
	}
	switch e.Type {
	case "user":
		if e.IsMeta {
			return
		}
		prompt, isPrompt := claudePrompt(e.Message.Content)
		if !isPrompt {
			return // a tool_result-only user event — not a new turn
		}
		st.open(prompt, e.Timestamp)
	case "assistant":
		st.ensure() // tail may have started mid-turn (no preceding prompt)
		text, steps := claudeAssistant(e.Message.Content)
		if text != "" {
			st.addText(text) // each assistant text starts a new bubble
		}
		st.addSteps(steps) // its tool calls attach to that bubble (intermediate process)
	}
}

// claudePrompt extracts a user-typed prompt from message.content. Returns ok=false
// when the content is a tool result (not a real prompt) or empty/meta wrapper.
func claudePrompt(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	// content as a bare string (the common typed-prompt case).
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return CleanUserPrompt(s)
	}
	// content as blocks: collect typed text, plus any user feedback embedded in a
	// tool_result (a tool REJECTED with "No, and tell Claude what to do" — the typed
	// message lives inside the rejection result, not a plain prompt; without this it
	// silently vanishes from the chat). Plain tool_result OUTPUT still yields nothing.
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return "", false
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			parts = append(parts, strings.TrimSpace(b.Text))
		} else if b.Type == "tool_result" {
			if fb := toolResultFeedback(b.Content); fb != "" {
				parts = append(parts, fb)
			}
		}
	}
	if len(parts) == 0 {
		return "", false // tool_result OUTPUT / image only
	}
	return CleanUserPrompt(strings.Join(parts, "\n"))
}

// rejectFeedbackMarker precedes the user's typed message inside a tool_result when
// they reject a tool use with "No, and tell Claude what to do (esc)" — Claude Code
// embeds the feedback in the rejection result rather than logging a plain prompt.
const rejectFeedbackMarker = "To tell you how to proceed, the user said:"

// toolResultFeedback returns the user's typed feedback from a tool_result block's
// content, or "" when there is none. A real tool OUTPUT result has array content
// (or a string without the marker) → "". A plain rejection ("…wait for the user to
// tell you how to proceed.") lacks the marker → "".
func toolResultFeedback(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(content, &s) != nil {
		return "" // array/object content = real tool output, not feedback
	}
	i := strings.Index(s, rejectFeedbackMarker)
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(s[i+len(rejectFeedbackMarker):])
}

// harnessBlockRe strips the synthetic XML-ish blocks the Claude Code harness
// injects INTO user content — context reminders and background-task notices. They
// are not typed by the user, so they must never surface as chat prompts (a turn
// that is ONLY such a block collapses to empty and is dropped; one appended to a
// real prompt is just trimmed off). `\b[^>]*>` tolerates attributes on the open tag.
var harnessBlockRe = regexp.MustCompile(`(?s)<(system-reminder|task-notification)\b[^>]*>.*?</(system-reminder|task-notification)>`)

// harnessOpenRe strips a DANGLING open block — a `<task-notification>`/`<system-reminder>`
// whose close tag is absent (truncated/streamed content). Without this a fragment like
// `<task-notification> <task-id>b50xphl27</` survives and is read back as a goal.
var harnessOpenRe = regexp.MustCompile(`(?s)<(system-reminder|task-notification)\b[^>]*>.*`)

// CleanUserPrompt strips system-injected content (harness reminder/task-notification
// blocks — closed OR truncated, `[SYSTEM NOTIFICATION …]` notices, and gtmux's own
// wake lines echoed back into a pane) from a raw prompt, and reports whether a real
// user-TYPED prompt remains. Shared by the transcript parser AND the hook's
// UserPromptSubmit path, so a system injection never surfaces as a goal or a nudge.
//
// It answers "does this render as a chat turn". A caller deciding whether the USER
// ACTED wants ClassifyUserPrompt instead — a slash command is not typed prose, but it
// is very much the user doing something.
func CleanUserPrompt(s string) (string, bool) {
	if text, kind := ClassifyUserPrompt(s); kind == PromptUser {
		return text, true
	}
	return "", false // a slash command's NAME is not a prompt — see ClassifyUserPrompt
}

// Prompt kinds — what a raw UserPromptSubmit payload actually IS.
const (
	PromptUser  = "user"  // prose the user typed; text is the cleaned prompt
	PromptSlash = "slash" // a slash-command invocation; text is the command name
	PromptDrop  = "drop"  // not authored by the user (harness, gtmux's own echo) — silent
)

// ClassifyUserPrompt strips system-injected content and classifies what remains.
// The distinction PromptSlash makes exists because a slash command (`/compact`,
// `/model`, a custom `/deploy`) carries no prose and so used to be dropped as "not a
// user prompt" — reaching HQ as silence, even though it changes what a session is
// doing. It is a user ACT with a machine-readable name, not a message.
func ClassifyUserPrompt(s string) (text, kind string) {
	s = stripInjected(s)
	switch {
	case s == "":
		return "", PromptDrop
	case isSlashWrapper(s):
		return slashCommandName(s), PromptSlash
	case isClaudeMetaPrompt(s):
		return "", PromptDrop
	default:
		return s, PromptUser
	}
}

// gtmuxEchoPrefixes are gtmux's own injected lines. Our wake line landing back in a
// pane's prompt (an echo, a copy-paste) must never read back as a user goal. The `»`
// form is the current wake format; `[gtmux]` is the retired one, kept for old panes.
var gtmuxEchoPrefixes = []string{"[gtmux]", Sigil + " gtmux·"}

// Sigil is the rune every gtmux wake line opens with (hqwake.Sigil — duplicated here
// rather than imported, so the transcript parser stays free of the wake channel).
const Sigil = "»"

// stripInjected removes non-user-typed content: harness XML blocks (closed then any
// dangling open one), then any LINE that is a `[SYSTEM NOTIFICATION …]` notice or one
// of gtmux's own injected lines.
func stripInjected(s string) string {
	s = harnessBlockRe.ReplaceAllString(s, "")
	s = harnessOpenRe.ReplaceAllString(s, "")
	var kept []string
	for _, ln := range strings.Split(s, "\n") {
		if isInjectedLine(strings.TrimSpace(ln)) {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// isInjectedLine reports whether a line was injected rather than typed.
func isInjectedLine(t string) bool {
	if strings.HasPrefix(t, "[SYSTEM NOTIFICATION") {
		return true
	}
	for _, p := range gtmuxEchoPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// claudeWrapperTags are the exact synthetic tags Claude injects as "user" content.
// Matching the EXACT tags (not the loose `<command-` prefix this once used) keeps a
// real prompt that happens to open with an angle bracket out of the filter.
var claudeWrapperTags = []string{
	"<command-name>", "<command-message>", "<command-args>",
	"<local-command-stdout>", "<local-command-stderr>",
}

// localCommandCaveat opens the wrapper Claude puts around local-command output.
const localCommandCaveat = "Caveat: The messages below were generated by the user while running local commands"

// isClaudeMetaPrompt filters the synthetic wrappers Claude injects as "user" content
// (slash-command echoes, local-command stdout, the local-command caveat).
func isClaudeMetaPrompt(s string) bool {
	if strings.HasPrefix(s, localCommandCaveat) {
		return true
	}
	for _, tag := range claudeWrapperTags {
		if strings.HasPrefix(s, tag) {
			return true
		}
	}
	return false
}

// isSlashWrapper reports whether the payload is a slash-command invocation — the
// `<command-name>` wrapper. Command OUTPUT (`<local-command-stdout>`) is not: the
// user's act was the command, and it already waked on its own submission.
func isSlashWrapper(s string) bool { return strings.HasPrefix(s, "<command-name>") }

// slashCommandName extracts the invoked command from a `<command-name>` wrapper
// ("" when it cannot be read — the caller still reports the act, unnamed).
func slashCommandName(s string) string {
	_, rest, found := strings.Cut(s, "<command-name>")
	if !found {
		return ""
	}
	name, _, found := strings.Cut(rest, "</command-name>")
	if !found {
		return ""
	}
	return strings.TrimSpace(name)
}

// claudeAssistant returns the concatenated text of an assistant message and its
// tool_use blocks as Steps.
func claudeAssistant(raw json.RawMessage) (string, []Step) {
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		// some assistant messages carry a bare string
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return strings.TrimSpace(s), nil
		}
		return "", nil
	}
	var text []string
	var steps []Step
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if strings.TrimSpace(b.Text) != "" {
				text = append(text, strings.TrimSpace(b.Text))
			}
		case "tool_use":
			steps = append(steps, Step{Kind: "tool", Title: b.Name, Detail: toolDetail(b.Input)})
		}
	}
	return strings.TrimSpace(strings.Join(text, "\n")), steps
}

// toolDetail picks a short, human-meaningful summary from a tool_use input object
// (file path / command / pattern / url …).
func toolDetail(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(input, &m) != nil {
		return ""
	}
	for _, k := range []string{"file_path", "path", "command", "pattern", "url", "query", "description", "prompt"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return clip(s, 80)
			}
		}
	}
	return ""
}
