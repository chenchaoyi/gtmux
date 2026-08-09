package prompt

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseOptions_ClaudeBox(t *testing.T) {
	text := `╭───────────────────────────────────────────╮
│ Do you want to make this edit to serve.go?  │
│                                             │
│ ❯ 1. Yes                                    │
│   2. Yes, and don't ask again this session  │
│   3. No, and tell Claude what to do (esc)   │
╰───────────────────────────────────────────╯`
	got := ParseOptions(text)
	want := []Option{
		{1, "Yes"},
		{2, "Yes, and don't ask again this session"},
		{3, "No, and tell Claude what to do (esc)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

// A rich picker (Claude Code's AskUserQuestion) renders each option BESIDE a preview
// panel on the same line, split by a box rule, so the parsed label swallows the preview.
// It is not a tap-to-reply menu — a bare number-send can't drive it and the labels are
// garbage — so OptionsReplyable is FALSE and the card layer offers nothing. But it still
// PARSES (non-nil) so the readiness detector treats it as "a menu, not a goal-ready
// composer". Pins the reported "选取后无法输入 / 选项解析错了" bug.
func TestOptionsReplyable_RejectsSideBySidePreviewPicker(t *testing.T) {
	picker := "❯ 1. Collapsible cards          │  ‹ All panes    19 panes · 10 sessions\n" +
		"  2. Clean tree                 │  ▾ Aurora        3 · 2 agents  ■1 ✓1\n"
	opts := ParseOptions(picker)
	if len(opts) == 0 {
		t.Fatal("the picker must still PARSE (non-nil) so readiness sees a menu, not a composer")
	}
	if OptionsReplyable(opts) {
		t.Error("a side-by-side preview picker is NOT tap-to-reply (interior box rule); OptionsReplyable must be false")
	}
	if WaitingOptions(picker) == nil {
		t.Error("WaitingOptions must still DETECT the picker (for hasPromptLine)")
	}
	// A genuinely long single-column label also reads as not-a-simple-menu.
	if OptionsReplyable([]Option{{1, strings.Repeat("x", 120)}, {2, "no"}}) {
		t.Error("an implausibly long option label is not replyable")
	}
	// A MULTI-select picker marks each option with a checkbox — the one-tap card can't
	// express "check 1 AND 3, submit", so it parses but is NOT replyable.
	multi := ParseOptions("❯ 1. [ ] point one\n  2. [ ] point two\n  3. [ ] point three\n")
	if len(multi) == 0 {
		t.Fatal("the multi-select picker must still PARSE so readiness sees a menu")
	}
	if OptionsReplyable(multi) {
		t.Error("a multi-select checkbox picker is NOT tap-to-reply; OptionsReplyable must be false")
	}
	// Regression: a CLEAN permission menu is both parsed AND replyable.
	clean := ParseOptions("❯ 1. Yes\n  2. No, and tell Claude what to do (esc)\n")
	if len(clean) != 2 || !OptionsReplyable(clean) {
		t.Errorf("a clean menu must parse AND be replyable; got %#v replyable=%v", clean, OptionsReplyable(clean))
	}
}

func TestParseOptions_PlainAndSelectorVariants(t *testing.T) {
	text := "Continue?\n  1. Yes\n> 2. No"
	got := ParseOptions(text)
	want := []Option{{1, "Yes"}, {2, "No"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

// NOTE on the ❯ in these fixtures: an option run must now carry a selector glyph to be
// read as a MENU. Without one it is a numbered LIST in prose, and presenting that as an
// approval card offers choices the agent never made. These fixtures gained the glyph
// because they were always meant to describe real menus.
func TestParseOptions_LatestMenuWins(t *testing.T) {
	// An older menu scrolled up, a fresh one below — the fresh run (restart at 1)
	// must win, not a concatenation.
	text := "❯ 1. old-a\n2. old-b\n... lots of output ...\n❯ 1. new-a\n2. new-b\n3. new-c"
	got := ParseOptions(text)
	want := []Option{{1, "new-a"}, {2, "new-b"}, {3, "new-c"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_StripsANSIColors(t *testing.T) {
	// A colored capture (capture-pane -e) leaks SGR codes into the menu; the ESC
	// byte is non-printing so users saw the bare "[38;5;153m…[0m" in the option
	// labels (reported 2026-06-29). The parser must strip them.
	esc := "\x1b"
	text := "❯ 1. " + esc + "[38;5;153mgtmux update" + esc + "[39m 之后" + esc + "[1m 自动" + esc + "[0m\n" +
		"  2. " + esc + "[31mNo" + esc + "[0m, keep current"
	got := ParseOptions(text)
	want := []Option{
		{1, "gtmux update 之后 自动"},
		{2, "No, keep current"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_StripsOSCHyperlinks(t *testing.T) {
	// Claude Code wraps file paths in OSC 8 hyperlinks; strip them too.
	esc := "\x1b"
	bel := "\x07"
	text := "❯ 1. open " + esc + "]8;;file:///tmp/a.go" + bel + "a.go" + esc + "]8;;" + bel + " now"
	got := ParseOptions(text)
	want := []Option{{1, "open a.go now"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_None(t *testing.T) {
	if got := ParseOptions("just some logs\nno menu here\nworking…"); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestParseOptions_GapBreaksRun(t *testing.T) {
	// "3." without a preceding 2. must not attach to option 1.
	text := "❯ 1. only\nsome noise\n3. orphan"
	got := ParseOptions(text)
	want := []Option{{1, "only"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

// WaitingOptions must fire on a live approval menu at the bottom (selector + ≥2
// options) but NOT on a numbered list in prose output, nor a menu far from the
// bottom — that's what keeps screen-based waiting detection from false alarms.
func TestWaitingOptions(t *testing.T) {
	codexMenu := "› Allow Codex to run this command?\n\n  › 1. Yes\n    2. Yes, don't ask again\n    3. No, tell Codex what to do\n"
	if got := WaitingOptions(codexMenu); len(got) != 3 || got[0].Label != "Yes" {
		t.Errorf("codex approval menu → %#v, want 3 options", got)
	}

	// a numbered list in prose (no selector cursor) must NOT read as waiting
	proseList := "Here's the plan:\n1. First refactor the parser\n2. Then add tests\n3. Finally ship it\nRunning now…\n"
	if got := WaitingOptions(proseList); got != nil {
		t.Errorf("prose numbered list → %#v, want nil", got)
	}

	// a menu buried far above the bottom (agent moved on) must NOT read as waiting
	buried := "› 1. Yes\n  2. No\n" + strings.Repeat("output line\n", 20)
	if got := WaitingOptions(buried); got != nil {
		t.Errorf("menu far from bottom → %#v, want nil", got)
	}

	// a single "1." with a selector isn't enough (need ≥2 real choices)
	if got := WaitingOptions("› 1. Only one\n"); got != nil {
		t.Errorf("single option → %#v, want nil", got)
	}

	// Claude's session-startup RESUME picker is a numbered menu with a selector,
	// but it's pre-task chrome — an old session reopened to it must NOT read as
	// "needs you" (the "2h-old session stuck waiting" bug).
	resume := "  This session is 22h 38m old and 166.8k tokens.\n" +
		"  Resuming the full session will consume a substantial portion of your usage limits.\n" +
		"  ❯ 1. Resume from summary (recommended)\n" +
		"    2. Resume full session as-is\n" +
		"    3. Don't ask me again\n" +
		"  Enter to confirm · Esc to cancel\n"
	if got := WaitingOptions(resume); got != nil {
		t.Errorf("resume picker → %#v, want nil (startup chooser, not a task-wait)", got)
	}

	// the trust-folder gate is likewise a startup chooser, not a task approval
	trust := "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n"
	if got := WaitingOptions(trust); got != nil {
		t.Errorf("trust-folder gate → %#v, want nil", got)
	}
}

// IsStartupGate detects an agent's PRE-TURN blocking gate (trust-folder confirmation) so
// the radar reads a stuck-before-running dispatch as waiting — but NOT the resume/theme
// picker (a reopened session must never read waiting forever). Per-agent, default set.
func TestIsStartupGate(t *testing.T) {
	trust := "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n"
	if !IsStartupGate(trust, "") {
		t.Error("trust-folder gate should be a startup gate")
	}
	resume := "  ❯ 1. Resume from summary (recommended)\n    2. Resume full session as-is\n"
	if IsStartupGate(resume, "") {
		t.Error("resume picker is NOT a gate — a reopened session must not read waiting")
	}
	if IsStartupGate("normal idle prompt\n❯ ", "") {
		t.Error("a normal idle screen is not a gate")
	}
	// A named agent still checks the default gate set.
	if !IsStartupGate(trust, "Claude Code") {
		t.Error("a named agent should still match the default gate set")
	}
}

// IsComposerReady is the SCREEN half of the spawn readiness handshake: a pane is ready
// to take a pasted goal only when its input prompt row is present, no startup/trust
// gate is up, and no boot banner is still on screen.
func TestIsComposerReady(t *testing.T) {
	// A clean idle Claude composer is ready.
	if !IsComposerReady("some earlier output\n\n❯ ", "") {
		t.Error("a clean idle composer should be ready")
	}
	// A boot banner (still CONNECTING/loading) is NOT ready — the composer hasn't taken
	// over input yet, so a long paste would truncate and its Enter would be swallowed.
	banners := []string{
		"❯ \n\nConnecting to MCP servers…",
		"Connecting to MCP servers…\n❯ ",
		"Starting Claude Code…",
		"Loading…\n❯ ",
	}
	for _, b := range banners {
		if IsComposerReady(b, "") {
			t.Errorf("boot banner should NOT be ready: %q", b)
		}
	}
	// A trust/permission gate is NOT ready (pasting through it would eat the keypress).
	trust := "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n"
	if IsComposerReady(trust, "") {
		t.Error("trust gate should NOT be ready")
	}
	// A live approval menu is a CHOICE wait, not a goal-ready composer.
	menu := "  ❯ 1. Yes\n    2. No\n    3. Always\n"
	if IsComposerReady(menu, "") {
		t.Error("a live approval menu should NOT be ready (waiting for a choice, not a goal)")
	}
	// A blank / still-booting screen with no prompt row is NOT ready.
	if IsComposerReady("\n\n\n", "") {
		t.Error("a screen with no prompt row should NOT be ready")
	}
	// A named agent still checks the default banner set.
	if IsComposerReady("Connecting to MCP servers…\n❯ ", "Claude Code") {
		t.Error("a named agent should still match the default boot-banner set")
	}
	// REGRESSION (2026-08-01 send-wedge): a READY composer must NOT be vetoed just
	// because the transcript/scrollback MENTIONS a boot word. hasBootBanner used to
	// substring-scan the WHOLE capture for bare words, so a dev pane whose own output
	// said "still connecting / Loading" refused every send forever (CLI + --message-file
	// + mobile POST /api/send, the phone bar wedged with no recovery).
	if !IsComposerReady("user asked why the still connecting / Loading text is so dim\n\n❯ ", "") {
		t.Error("a boot word MID-PROSE must not veto a ready composer (the send-wedge)")
	}
	// The same word far up in scrollback (outside the bottom region) must not veto either.
	scrolledUp := "Loading assets\n" + strings.Repeat("output line\n", 20) + "❯ "
	if !IsComposerReady(scrolledUp, "") {
		t.Error("a boot word up in scrollback must not veto a ready composer")
	}
}

// A STANDING NOTICE is not boot noise. `⚠ N MCP servers need authentication · run /mcp`
// names an action only the user can take and never clears on its own, so treating it as
// startup chrome made the ready gate UNSATISFIABLE on any machine carrying one: every
// `gtmux spawn` ran out its 20s timeout into a false `NOT delivered` while the session
// sat there with an empty composer (three dispatches on 2026-08-09, each rescued by
// hand). The control assertions below are the boundary — the fix must not buy this by
// disarming the gate for a pane that IS still booting.
func TestIsComposerReady_StandingAuthNoticeIsNotBootNoise(t *testing.T) {
	// The real shape from the incident capture: the notice sits just above an empty,
	// settled composer row.
	live := " ⚠ 10 MCP servers need authentication · run /mcp\n❯ "
	if !IsComposerReady(live, "") {
		t.Error("a settled composer carrying a standing auth notice must be READY")
	}
	if !IsComposerReady(live, "claude") {
		t.Error("same for a named agent (the default set is what carried the phrase)")
	}
	// Below the composer box is the other layout the notice appears in.
	below := "╭──────────────╮\n│ >            │\n╰──────────────╯\n  ⚠ 3 MCP servers need authentication · run /mcp"
	if !IsComposerReady(below, "") {
		t.Error("the notice below the composer box must not block either")
	}

	// CONTROL 1 — the transient sibling still holds the gate. This is the hazard the
	// banner list was written for: Claude paints the composer while MCP servers are
	// still coming up, and a paste into that repainting window truncates.
	if IsComposerReady("Connecting to MCP servers…\n❯ ", "") {
		t.Error("a CONNECTING banner must still hold the gate — it resolves by waiting")
	}
	// CONTROL 2 — a genuinely booting pane (no composer row yet) is still not ready.
	if IsComposerReady("Starting Claude Code…\n 10 MCP servers need authentication", "") {
		t.Error("a booting pane with no composer row must still be NOT ready")
	}
	// CONTROL 3 — the notice does not waive the startup gate either.
	gate := " ⚠ 10 MCP servers need authentication · run /mcp\n Do you trust the files in this folder?\n ❯ 1. Yes\n   2. No\n"
	if IsComposerReady(gate, "") {
		t.Error("a trust gate must still block, notice or not")
	}
}

// BootBannerLine is the diagnostic twin: the ready-gate timeout has to be able to say
// WHICH line said no. "composer not ready within the ready timeout" alone is what got
// this footgun read as "the agent is slow" for months.
func TestBootBannerLine(t *testing.T) {
	got := BootBannerLine("prior output\n│ Connecting to MCP servers… │\n❯ ", "")
	if got != "Connecting to MCP servers…" {
		t.Errorf("BootBannerLine = %q, want the matched line stripped of box chrome", got)
	}
	if BootBannerLine("all quiet\n❯ ", "") != "" {
		t.Error("a ready screen has no banner line")
	}
	if l := BootBannerLine(" ⚠ 10 MCP servers need authentication · run /mcp\n❯ ", ""); l != "" {
		t.Errorf("a standing notice is not a banner line, got %q", l)
	}
}

// NotReadyReason explains a not-ready capture in one line — each branch naming what the
// reader has to DO about it.
func TestNotReadyReason(t *testing.T) {
	if r := NotReadyReason("earlier output\n\n❯ ", ""); r != "" {
		t.Errorf("a ready capture has no reason, got %q", r)
	}
	if r := NotReadyReason("Connecting to MCP servers…\n❯ ", ""); !strings.Contains(r, "Connecting to MCP servers…") {
		t.Errorf("reason must quote the blocking line, got %q", r)
	}
	trust := "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n"
	if r := NotReadyReason(trust, ""); !strings.Contains(r, "startup gate") {
		t.Errorf("a trust gate must be named as such, got %q", r)
	}
	if r := NotReadyReason("  ❯ 1. Yes\n    2. No\n    3. Always\n", ""); !strings.Contains(r, "choice menu") {
		t.Errorf("a live menu must be named as such, got %q", r)
	}
	if r := NotReadyReason("\n\n\n", ""); !strings.Contains(r, "no composer prompt row") {
		t.Errorf("a blank screen must report the missing composer row, got %q", r)
	}
}

// hasBootBanner matches the default set and a named agent's own phrases.
func TestHasBootBanner(t *testing.T) {
	if !hasBootBanner("all good\nConnecting to MCP servers…\n❯ ", "") {
		t.Error("a connect spinner is a boot banner")
	}
	if hasBootBanner("normal idle\n❯ ", "") {
		t.Error("a clean idle screen has no boot banner")
	}
	// A boot word appearing MID-LINE as prose (not the start of a bottom line) is not a
	// banner — anchoring the one-word signatures is what unwedged everyday `gtmux send`.
	if hasBootBanner("we were Loading and Connecting earlier, all done now\n❯ ", "") {
		t.Error("mid-prose boot words must not read as a boot banner")
	}
	// But a genuine bottom spinner line still counts.
	if !hasBootBanner("Connecting to MCP server…\n", "") {
		t.Error("a real bottom 'Connecting…' line is still a boot banner")
	}
	// A boot word up in scrollback (outside the bottom region) is not a banner.
	if hasBootBanner("Loading models\n"+strings.Repeat("x\n", 20)+"❯ ", "") {
		t.Error("a boot word up in scrollback must not read as a boot banner")
	}
}

// The reported bug: a pane genuinely WAITING on a free-form question, whose recent output
// happened to contain a numbered list, showed that list as an approval menu — "choices"
// the agent never offered, on a card that invites a single keypress to answer with.
//
// The discriminator was already named in prompt.go (selectorGlyphs) and used for
// waiting-detection; the parser itself matched any "1. …" anywhere on screen.
func TestParseOptions_ProseListIsNotAMenu(t *testing.T) {
	prose := "Here's what's left:\n" +
		"1. 雷达不该做 per-pane exec。\n" +
		"2. attach 的 8s 超时 + 误导性错误。\n" +
		"3. Ghostty 窗口顺序（机器验不了）\n" +
		"\nWhich should I do next?"
	if got := ParseOptions(prose); got != nil {
		t.Errorf("a prose list became a menu: %#v", got)
	}
}

// ...while a real menu on the same screen still parses. The guard must not cost us the
// actual approval card.
func TestParseOptions_RealMenuStillParsesAfterProse(t *testing.T) {
	text := "Here's what's left:\n1. a thing\n2. another\n\n" +
		"Do you want to proceed?\n❯ 1. Yes\n  2. Yes, and don't ask again\n  3. No"
	got := ParseOptions(text)
	want := []Option{{1, "Yes"}, {2, "Yes, and don't ask again"}, {3, "No"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v; want the real menu", got)
	}
}

// The selector may sit on a row other than the first — only ONE row is highlighted.
func TestParseOptions_SelectorOnALaterRow(t *testing.T) {
	got := ParseOptions("Continue?\n  1. Yes\n❯ 2. No")
	if len(got) != 2 {
		t.Fatalf("got %#v; want both options — the run carries a glyph on row 2", got)
	}
}
