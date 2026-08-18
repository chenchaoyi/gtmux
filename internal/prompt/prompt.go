// Package prompt parses an interactive coding-agent menu (Claude Code's
// "❯ 1. Yes / 2. … / 3. …" choice block) out of captured pane text. This is the
// ONE canonical parser (HANDOFF: shared by the menu-bar in-place reply, macOS
// notifications, and the mobile approval card) — surfaces consume it via
// `gtmux options <pane>` (CLI) or the serve API, never re-implement it.
package prompt

import (
	"regexp"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/ansi"
)

// Option is one numbered choice: N is the key the user presses (1/2/3…), Label is
// the agent's own wording for it.
type Option struct {
	N     int    `json:"n"`
	Label string `json:"label"`
}

// numbered matches a cleaned line like "1. Yes, proceed" → (1, "Yes, proceed").
var numbered = regexp.MustCompile(`^(\d+)\.\s+(.*\S)`)

// ParseOptions extracts the LAST contiguous run of options starting at 1 from the
// captured pane text (the most recent menu on screen). Returns nil when there's
// no parseable choice block — callers fall back to "jump to reply".
//
// It tolerates the box-drawing chrome Claude Code draws around the menu (leading
// │ ╭ ╰ ─ and the ❯/> selector, trailing │ and padding) and resets whenever it
// sees a fresh "1." so a stale earlier menu never wins.
func ParseOptions(text string) []Option {
	var opts []Option
	want := 1
	selected := false // did this run carry a selector glyph — i.e. is it a LIVE menu?
	for _, raw := range strings.Split(text, "\n") {
		line := clean(raw)
		m := numbered.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// Note the cursor BEFORE clean() discards it. An interactive menu marks its
		// highlighted row (❯ 1. Yes); prose never does. The glyph must LEAD the row —
		// see selectorLeads for the false approval card that "anywhere on the line"
		// produced out of an agent's own findings list.
		glyph := selectorLeads(raw, promptGlyphs)
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		label := strings.TrimSpace(m[2])
		if n == 1 {
			// a new menu started — restart the run.
			opts = []Option{{N: 1, Label: label}}
			want = 2
			selected = glyph
			continue
		}
		if n == want {
			opts = append(opts, Option{N: n, Label: label})
			want++
			selected = selected || glyph
		}
	}
	// A run with NO selector glyph anywhere is a numbered LIST, not a menu.
	//
	// This file already named the discriminator — see selectorGlyphs — and used it for
	// waiting-detection, but the parser itself matched any "1. …" line anywhere on
	// screen. So while a pane was genuinely waiting on a free-form question, three
	// bullet points from the agent's own prose were scraped out of the scrollback and
	// presented as an approval menu: choices that were never offered, on a card that
	// invites one keypress to "answer" with. Presenting a wrong choice is worse than
	// presenting none — with no card the user simply types their reply.
	if len(opts) == 0 || !selected {
		return nil
	}
	return opts
}

// OptionsReplyable reports whether a parsed option run is a CLEAN, SINGLE-select,
// tap-to-reply menu — the kind a bare number-send actually drives (a permission dialog).
// It is FALSE for a rich picker the one-tap card can't express:
//   - a side-by-side preview picker (Claude Code's AskUserQuestion renders each option
//     BESIDE a preview panel on the SAME line, split by a box rule, so the parsed label
//     swallows the preview: "整洁树 │ ‹ All panes 19 panes …"). clean() strips the box's
//     EDGE rule but not an INTERIOR one; an implausibly long label is the same smell.
//   - a MULTI-select picker (each option marked with a "[ ] …" / "[x] …" checkbox): the
//     card can only single-tap-and-submit, so it can't express "check 1 AND 3, submit".
//
// Callers that offer a one-tap reply card (the mobile ApprovalCard, the digest Ask) gate
// on this; the readiness detector (hasPromptLine) deliberately does NOT — a picker is
// still "a menu, not a goal-ready composer".
func OptionsReplyable(opts []Option) bool {
	for _, o := range opts {
		label := strings.TrimSpace(o.Label)
		if strings.ContainsRune(label, '│') || strings.ContainsRune(label, '┃') || len([]rune(label)) > 100 {
			return false
		}
		if strings.HasPrefix(label, "[ ]") || strings.HasPrefix(label, "[x]") || strings.HasPrefix(label, "[X]") {
			return false
		}
	}
	return len(opts) > 0
}

// selectorGlyphs are the cursor marks interactive TUI choice menus put on the
// highlighted row (Claude ❯, Codex ›, others ▶ ▸ →). A numbered list in prose
// output has none — so requiring one tells an ACTIVE approval menu apart from a
// list, which is what lets us detect "waiting" from the screen for agents that
// (unlike Claude) fire no waiting hook.
const selectorGlyphs = "❯›▶▸→"

// boxChrome is the box-drawing a TUI draws around a menu, skipped when looking for
// the selector cursor's position.
const boxChrome = "│╭╰╮╯─ \t"

// selectorLeads reports whether a selector cursor sits BEFORE the number on this line —
// `❯ 1. Yes`, which is where a TUI actually draws it — as opposed to merely appearing
// SOMEWHERE on the line.
//
// That distinction is the whole discriminator between a live menu and prose, and testing
// it as "contains a glyph anywhere" was not one. `→` is in the set and is an everyday
// character in technical writing: on 2026-08-10 an agent's own numbered findings list,
// one bullet of which read `(config.toml enabled = false)→ gtmux 收不到`, was served to
// the phone as a five-choice approval card — tapping any row would have typed a digit
// into the pane. Measured: with that one arrow, ParseOptions returned 5 options and
// OptionsReplyable said yes; delete the arrow and it returns none. `>` (in promptGlyphs)
// is worse still — it opens every quoted shell line and every markdown blockquote.
//
// A cursor prefix is: box chrome and spaces, at most a selector glyph, then the digit.
// Anything else before the number means the glyph is prose, not a cursor.
func selectorLeads(raw, glyphs string) bool {
	seen := false
	for _, r := range ansi.Strip(raw) {
		switch {
		case r >= '0' && r <= '9':
			return seen
		case strings.ContainsRune(glyphs, r):
			seen = true
		case strings.ContainsRune(boxChrome, r):
			// skip
		default:
			return false
		}
	}
	return false
}

// startupGates are an agent's PRE-TURN BLOCKING gate — it needs a keypress to proceed
// before any task can run (Claude's trust-folder confirmation, and equivalents). Unlike
// the resume/theme pickers below, a gate means the worker is STUCK before running a
// single step, so the radar DOES read it as waiting (needs-you). Keyed by agent name
// ("" = the default/Claude set); an agent can add its own gate phrases. Extensible for
// codex/gemini/… whose startup gates differ — NOT hardcoded to one agent.
var startupGates = map[string][]string{
	"": {"Do you trust the files"}, // Claude trust-folder gate
	// Codex 0.147.0, live-captured (2026-08-10): both gates render as a numbered
	// menu with a › selector, so WaitingOptions already refuses them a "ready"
	// verdict — these signatures make the protection EXPLICIT (a renderer change
	// can't silently open the gate) and let NotReadyReason name the gate instead
	// of "a choice menu".
	"codex": {
		"Do you trust the contents of this directory", // directory trust gate ("› 1. Yes, continue / 2. No, quit")
		"Hooks need review",                           // hooks trust gate after a hooks.json change ("› 1. Review hooks / 2. Trust all …")
	},
}

// startupPickers are REOPENED-SESSION chrome — the resume picker / theme picker. They
// present as a numbered menu with a selector (so they'd pass WaitingOptions' shape
// test), but they are NOT a block: a 2h-old reopened session at its resume picker must
// never read "waiting" (the original "stuck waiting" false-positive). So they are
// excluded from waiting detection but are NOT treated as a startup GATE.
//
// Keyed by agent exactly like startupGates ("" = the default/Claude set; a named agent
// also gets its own), and resolved through the same registry lookup, so an agent whose
// reopened-session chrome reads differently can be taught it without anyone editing a
// hardcoded list. Codex has no entry yet — not because it has no such menu, but because
// nobody has captured one; inventing signatures for chrome we have not seen is how a
// table starts lying.
var startupPickers = map[string][]string{
	"": {
		"Resume from summary",       // Claude resume picker
		"Resume full session",       // Claude resume picker
		"Resuming the full session", // Claude resume picker body
	},
}

// pickerSignatures returns the picker phrases to match for this agent — the default set
// plus its own, resolved by key OR display label (see gateSignatures for why both).
func pickerSignatures(agent string) []string {
	sigs := startupPickers[""]
	if agent == "" {
		return sigs
	}
	own, ok := startupPickers[agent]
	if !ok {
		own = startupPickers[agents.KeyForLabel(agent)]
	}
	if len(own) == 0 {
		return sigs
	}
	return append(append([]string(nil), sigs...), own...)
}

// IsStartupPicker reports whether the capture shows the agent's REOPENED-SESSION menu —
// the resume picker and friends. It is the twin of IsStartupGate and carries both of that
// function's hard-won narrowings for the same reasons: only the BOTTOM region counts (a
// pane whose scrollback merely MENTIONS "Resume from summary" is not sitting at one), and
// faint text is dropped (an agent's dimmed ghost suggestion is not chrome it is drawing).
//
// The caller that needs this most is the stuck-dispatch classifier. A resume menu is a
// numbered list the agent PAINTED; the draft detector, asked whether the input box holds
// unsubmitted text, reads that painted menu as text somebody typed. On 2026-08-18 that
// produced a `stuck before running — draft` alarm about a session nobody had dispatched
// anything to — the third time in two weeks that agent chrome has been misread as user
// input (dark placeholder text, 2026-08-06; a gate phrase in scrollback, 2026-08-10).
func IsStartupPicker(capture, agent string) bool {
	sigs := pickerSignatures(agent)
	for _, raw := range bottomLines(ansi.StripDroppingFaint(capture), gateRegionLines) {
		line := strings.TrimLeft(raw, "│╭╰╮╯─ \t")
		for _, sig := range sigs {
			if strings.Contains(line, sig) {
				return true
			}
		}
	}
	return false
}

// gateRegionLines is how far up from the bottom a gate signature is looked for. It is
// WIDER than bootBannerLine's 14 because the two are different shapes: a boot banner is
// one line of chrome drawn where the composer will be, while a gate is a whole DIALOG
// whose question sits near its TOP with the option rows, the box border and a
// "Press enter to confirm" hint below it (Claude's trust dialog runs ~15 lines). Too
// narrow a region here would stop seeing a real gate, which is the expensive direction.
const gateRegionLines = 24

// gateSignatures returns the gate phrases to match for this agent: the default (Claude)
// set plus the agent's own, if it has any.
//
// The agent argument is accepted as EITHER the registry key ("codex", what the dispatch
// ready gate passes) or the display label ("Codex", what the radar and the HQ slow tick
// carry on a pane row). The map is keyed by key, so the label callers silently looked up
// nothing and codex's two gates — added in #751 precisely so a codex worker stuck at its
// trust prompt is seen — were dead on exactly the paths that watch a running fleet.
func gateSignatures(agent string) []string {
	sigs := startupGates[""]
	if agent == "" {
		return sigs
	}
	own, ok := startupGates[agent]
	if !ok {
		own = startupGates[agents.KeyForLabel(agent)]
	}
	if len(own) == 0 {
		return sigs
	}
	return append(append([]string(nil), sigs...), own...)
}

// IsStartupGate reports whether the capture shows the agent's PRE-TURN BLOCKING gate
// (needs a keypress to proceed) — the trust-folder confirmation and equivalents —
// looked up per-agent (agent "" uses the default set only; a named agent also checks
// its own). It deliberately does NOT match the resume/theme pickers (those are handled
// by startupPickers / looksLikeStartupChooser).
//
// Two narrowings, both paid for in production (2026-08-09/10):
//
//   - REGION. It used to scan the whole capture, which is 200 lines of SCROLLBACK. A
//     worker editing this very file rendered its own diff — the source line
//     `"": {"Do you trust the files"}` — into its pane, and gtmux read that as a live
//     trust gate: 36 false `waiting·startup` knocks at HQ over 13.6 hours, starting five
//     minutes after the edit. This is the same defect #652 fixed for boot banners ("a
//     pane whose scrollback merely MENTIONED those words"), left standing in the sibling
//     function. A gate is chrome the agent is DRAWING, so only the bottom region counts.
//   - FAINT. Text emitted while SGR faint is active is an agent's ghost/placeholder
//     suggestion, not something it is asking. On a COLOR capture that text is dropped
//     (StripDroppingFaint is identity on a plain one, so the plain-capture callers are
//     unaffected). The assumption is that no agent renders a blocking gate — the most
//     prominent thing it can put on screen — dimmed; TestStartupGate_ColorCapture pins
//     the other side of it, that a normal-brightness gate in a color capture still reads.
func IsStartupGate(capture, agent string) bool {
	sigs := gateSignatures(agent)
	for _, raw := range bottomLines(ansi.StripDroppingFaint(capture), gateRegionLines) {
		line := strings.TrimLeft(raw, "│╭╰╮╯─ \t")
		for _, sig := range sigs {
			if strings.Contains(line, sig) {
				return true
			}
		}
	}
	return false
}

// bootBanners are an agent's still-BOOTING chrome — the startup banner it paints
// while connecting before its composer stably takes over input (the MCP-connect
// spinner, the generic starting/loading lines). Pasting a task while any of these is on
// screen risks a truncated goal and a swallowed Enter, so they make a pane NOT-ready.
// Keyed by agent name ("" = the default/Claude set); an agent can add its own boot
// phrases. Extensible for codex/gemini/… — NOT hardcoded to one agent.
//
// THE MEMBERSHIP RULE — a boot banner is chrome that RESOLVES BY WAITING. That is the
// whole justification for blocking on it: wait, and the pane becomes deliverable.
//
// A STANDING NOTICE does not belong here even though it looks identical — a
// bottom-region line naming an action only the USER can take. `⚠ N MCP servers need
// authentication · run /mcp` is not boot noise, it is the boot RESULT: some servers
// failed auth, and the line stays on screen forever until someone runs /mcp. It sat in
// this list until 2026-08-09, and on a machine that permanently carries one that made
// the ready gate UNSATISFIABLE — every `gtmux spawn` timed out into a false
// `NOT delivered` while the session sat there with an empty composer and the goal in
// nobody's hands (three dispatches on 2026-08-09 alone, each rescued by hand with
// `gtmux send --message-file`). #652 narrowed the MATCHING (bottom region + line
// anchoring) but left the phrase in, so only the "transcript mentions it" variant was
// fixed and the real standing banner kept blocking; see the
// spawn-readiness-persistent-banner change for why gating the phrase on the composer's
// absence — the obvious-looking alternative — would have deleted the check outright
// instead of narrowing it.
//
// The transient sibling stays: while MCP servers are still coming up, Claude paints
// `Connecting…`, which DOES resolve — so the hazard this list was written for is
// unchanged. The residual window (the auth line already up while the TUI still
// repaints) is covered by the caller's two-frame settle check, which is the mechanism
// built for exactly that.
var bootBanners = map[string][]string{
	"": { // Claude Code boot noise
		"Connecting",
		"Connecting…",
		"Starting",
		"Loading",
		"Initializing",
	},
	// Codex 0.147.0, live-captured (2026-08-10): while its MCP servers connect it
	// paints "• Starting MCP servers (1/4): … (0s • esc to interrupt)" WITH the
	// composer already drawn below — so without this signature the ready gate says
	// yes mid-boot. The default one-word "Starting" can't catch it: that must
	// PREFIX the line, and Codex's line starts with "• ". Multi-word, so it
	// matches anywhere on a bottom line, and it RESOLVES BY WAITING (the line is
	// replaced by the connect result). Its standing siblings ("⚠ MCP startup
	// incomplete", "⚠ MCP client … failed to start") stay OUT of this list — they
	// never resolve, and listing one made the gate unsatisfiable once before
	// (the spawn-readiness-persistent-banner change).
	"codex": {
		"Starting MCP servers",
	},
}

// hasBootBanner reports whether the BOTTOM of the capture shows a still-booting banner
// for the agent (default set + the named agent's own). A booting pane is not yet ready
// to take a goal.
//
// Scope + anchoring both matter. An earlier version did strings.Contains over the WHOLE
// capture for bare words like "Loading"/"Connecting", so ANY pane whose scrollback or
// agent output merely MENTIONED those words permanently failed IsComposerReady — which
// gates `gtmux spawn` and HQ's briefing dispatch (their READY handshake before pasting a
// goal). NB: it does NOT gate `gtmux send` / `POST /api/send`; those never call
// IsComposerReady — a phone "input box didn't confirm" is the pre-submit paste guard, a
// different scrape. Root-caused 2026-08-01 on a dev pane whose transcript was literally
// discussing "still connecting / loading". A boot banner is bottom chrome shown WHERE the
// composer will be, so we scan only the bottom region (like hasPromptLine), and a one-word
// spinner signature must ANCHOR the line — the line IS "Connecting…", not prose that
// happens to contain "connecting". Multi-word signatures ("MCP servers need
// authentication") are specific enough to match anywhere on a bottom line.
func hasBootBanner(capture, agent string) bool { return bootBannerLine(capture, agent) != "" }

// BootBannerLine is hasBootBanner's diagnostic twin: it returns the bottom-region LINE
// that matched a boot-banner signature (trimmed of box chrome), or "" when none did.
// The predicate answers "is this pane ready?"; this answers "which line said no?" — the
// question a ready-gate timeout has to be able to answer, because "composer not ready
// within the ready timeout" alone reads as "the agent is slow" and got this exact
// footgun misdiagnosed for months.
func BootBannerLine(capture, agent string) string { return bootBannerLine(capture, agent) }

func bootBannerLine(capture, agent string) string {
	sigs := bootBanners[""]
	if agent != "" {
		sigs = append(append([]string(nil), sigs...), bootBanners[agent]...)
	}
	for _, raw := range bottomLines(capture, 14) {
		s := strings.TrimLeft(ansi.Strip(raw), "│╭╰╮╯─ \t")
		for _, sig := range sigs {
			if strings.ContainsRune(sig, ' ') {
				if strings.Contains(s, sig) { // specific multi-word banner
					return strings.TrimRight(s, "│ \t")
				}
			} else if strings.HasPrefix(s, sig) { // generic one-word spinner: must start the line
				return strings.TrimRight(s, "│ \t")
			}
		}
	}
	return ""
}

// promptGlyphs are the input-prompt cursor marks an agent draws on its (empty)
// composer row when it is idle and ready to take input — the selector glyphs plus the
// plain ">" some agents use. A composer row carries one; a boot banner / blank screen
// does not.
const promptGlyphs = selectorGlyphs + ">"

// hasPromptLine reports whether the bottom region of the capture shows the agent's
// input-prompt (composer) row — the row a ready, idle agent draws to take a goal. It
// deliberately returns false when a live approval/gate MENU is up (WaitingOptions
// non-nil): a menu is waiting for a CHOICE, not a goal, and is not a goal-ready
// composer.
func hasPromptLine(capture, agent string) bool {
	if WaitingOptions(capture) != nil {
		return false
	}
	for _, raw := range bottomLines(capture, 14) {
		s := ansi.Strip(raw)
		s = strings.TrimLeft(s, "│╭╰╮╯─ \t")
		if strings.ContainsAny(s, promptGlyphs) {
			return true
		}
	}
	return false
}

// bottomLines returns up to the last n non-whitespace-blank lines of the capture — the
// bottom region where an agent's active prompt/menu lives. It trims on raw whitespace
// (NOT clean()) because clean() strips the very selector glyph the composer row is
// detected by, which would swallow an empty `❯ ` prompt row as if it were chrome.
func bottomLines(text string, n int) []string {
	lines := strings.Split(text, "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	lo := end - n
	if lo < 0 {
		lo = 0
	}
	return lines[lo:end]
}

// IsComposerReady reports whether the capture shows an agent whose composer is
// input-ready and settled enough to receive a pasted goal: the input prompt row is
// present, no PRE-TURN blocking gate (trust/permission) is up, and no still-booting
// banner is on screen. It is the SCREEN half of the spawn readiness handshake
// (launched → ready → content-verified → submitted); the caller adds the
// two-stable-samples check. Agent "" uses the default (Claude) signatures; a named
// agent also checks its own. Pure string predicate — no tmux.
func IsComposerReady(capture, agent string) bool {
	return hasPromptLine(capture, agent) &&
		!IsStartupGate(capture, agent) &&
		!hasBootBanner(capture, agent)
}

// NotReadyReason names, in one diagnostic line, WHY IsComposerReady is false for this
// capture — "" when it is ready. It is the screen half of a ready-gate failure report:
// the caller adds what only it can see (the agent never launched; the composer never
// settled). Ordered most-actionable first, and each branch says what to DO about it,
// because the reader of this string is someone whose dispatch just failed.
//
// English-only by the same convention `dispatch`'s evidence strings follow: this is
// diagnostic DATA embedded in `--json` output, not a UI string (the ✗ line that
// introduces it is localized).
func NotReadyReason(capture, agent string) string {
	if IsStartupGate(capture, agent) {
		return "the agent is at a startup gate and needs a keypress before it can take a task"
	}
	if line := BootBannerLine(capture, agent); line != "" {
		return "a startup banner is on screen: " + line
	}
	if WaitingOptions(capture) != nil {
		return "the pane is at a choice menu, not a goal-ready composer"
	}
	if !hasPromptLine(capture, agent) {
		return "no composer prompt row on screen (the agent has not drawn its input box)"
	}
	return ""
}

// looksLikeStartupChooser reports whether the bottom-of-screen menu is agent
// session-startup chrome (a GATE or a PICKER) rather than a task-level approval — so
// WaitingOptions doesn't flag either as an active task wait.
func looksLikeStartupChooser(window string) bool {
	return IsStartupGate(window, "") || IsStartupPicker(window, "")
}

// WaitingOptions returns the on-screen choice block ONLY when it looks like an
// ACTIVE approval menu the agent is blocked on: a run of ≥2 numbered options in
// the bottom of the capture, with a selector cursor present. It's deliberately
// stricter than ParseOptions (which callers use once a pane is already known to
// be waiting) so it can be used to DETECT waiting without false-positiving on a
// numbered list. Returns nil otherwise.
func WaitingOptions(text string) []Option {
	lines := strings.Split(text, "\n")
	end := len(lines)
	for end > 0 && clean(lines[end-1]) == "" {
		end-- // ignore trailing blank / chrome-only lines
	}
	if end == 0 {
		return nil
	}
	lo := end - 14 // only the bottom of the screen — the active prompt lives there
	if lo < 0 {
		lo = 0
	}
	window := lines[lo:end]
	joined := strings.Join(window, "\n")
	if looksLikeStartupChooser(joined) {
		return nil // a session-startup menu, not an agent task-wait
	}
	opts := ParseOptions(joined)
	if len(opts) < 2 {
		return nil // a real menu has ≥2 choices; a lone "1." is likely a list item
	}
	for _, l := range window {
		// The selector cursor must LEAD a numbered choice ("❯ 1. Yes") — that's a live
		// menu. A bare "❯ " input prompt (Claude idle) also carries the glyph, so
		// requiring the number too avoids flagging an idle pane whose recent OUTPUT
		// happens to contain a numbered list above the prompt; requiring the glyph to
		// come FIRST (selectorLeads) stops a prose bullet that merely contains a `→`
		// from passing as the highlighted row.
		if selectorLeads(l, selectorGlyphs) && numbered.MatchString(clean(l)) {
			return opts
		}
	}
	return nil
}

// clean strips the menu's box-drawing/selector chrome so numbered() can match the
// content: ANSI color/hyperlink escapes anywhere in the line, then the leading
// │ ╭ ╰ ╮ ╯ ─ space and the ❯ / > selector, and a trailing │ + padding spaces.
func clean(s string) string {
	s = ansi.Strip(s) // CSI + OSC escapes → plain, so labels/box-detection are text
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "│╭╰╮╯─ \t")
	s = strings.TrimLeft(s, selectorGlyphs+"> \t") // ❯ (Claude) › ▶ ▸ → (others) > and padding
	s = strings.TrimRight(s, "│ \t")
	return s
}
