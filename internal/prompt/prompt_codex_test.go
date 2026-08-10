package prompt

import (
	"strings"
	"testing"
)

// Every fixture in this file is a REAL capture from codex-cli 0.147.0 on
// 2026-08-10 (the live-verify probe, HANDOFF-2026-08-09 §8.3 item ②): the two
// startup trust gates were reproduced in an isolated CODEX_HOME (hooks present,
// trust hashes absent), the boot/idle/approval frames read off the live probe
// pane. Do not hand-edit a frame to make an assertion pass — recapture instead;
// guessed signatures are exactly how this repo was bitten before
// (agent-tui-border-format-drift, codex-receipt-dead).

// codexDirTrustGate is the first-run directory trust gate.
const codexDirTrustGate = `> You are in /private/tmp

  Do you trust the contents of this directory? Working with untrusted contents comes with higher
  risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec
  policies to load.

› 1. Yes, continue
  2. No, quit

  Press enter to continue`

// codexHooksReviewGate appears after hooks.json changes (or its trust hashes are
// missing) — the gate #703's hook install re-triggers.
const codexHooksReviewGate = `  Hooks need review
  5 hooks are new or changed.
  Hooks can run outside the sandbox after you trust them.

› 1. Review hooks
  2. Trust all and continue
  3. Continue without trusting (hooks won't run)

  Press enter to confirm or esc to go back`

// codexBootMCP is the mid-boot window: the composer and status footer are ALREADY
// drawn while MCP servers are still connecting.
const codexBootMCP = `╭───────────────────────────────────────────╮
│ >_ OpenAI Codex (v0.147.0)                │
│                                           │
│ model:     gpt-5.6-sol   /model to change │
│ directory: /private/tmp                   │
╰───────────────────────────────────────────╯

  Tip: Try the Desktop app. Run 'codex app' or visit https://chatgpt.com/codex?app-landing-page=true

• You have 1 usage limit reset available. Run /usage to use one.

• Starting MCP servers (1/4): cloudflare-api, codex_apps, xcodebuildmcp (0s • esc to interrupt)

› Improve documentation in @filename

  gpt-5.6-sol default · /private/tmp`

// codexIdleStandingWarn is the settled idle screen on a machine whose MCP server
// permanently fails auth: the ⚠ lines are STANDING notices (they never resolve),
// and the composer holds only its dim placeholder.
const codexIdleStandingWarn = `╭───────────────────────────────────────────╮
│ >_ OpenAI Codex (v0.147.0)                │
│                                           │
│ model:     gpt-5.6-sol   /model to change │
│ directory: /private/tmp                   │
╰───────────────────────────────────────────╯

  Tip: Try the Desktop app. Run 'codex app' or visit https://chatgpt.com/codex?app-landing-page=true

• You have 1 usage limit reset available. Run /usage to use one.

⚠ MCP client for ` + "`cloudflare-api`" + ` failed to start: MCP startup failed: failed to refresh OAuth
  tokens for server cloudflare-api: OAuth refresh token was rejected: Server returned error
  response: invalid_grant: Grant not found

⚠ MCP startup incomplete (failed: cloudflare-api)

› Improve documentation in @filename

  gpt-5.6-sol default · /private/tmp`

// codexApprovalMenu is a live command-approval request (network access, mid-turn).
const codexApprovalMenu = `• Running curl -sI https://example.com | head -1


  Would you like to run the following command?

  Environment: local

  Reason: 是否允许访问 example.com，以便运行你给出的 curl 命令并返回响应首行？

  $ curl -sI https://example.com | head -1

› 1. Yes, proceed (y)
  2. Yes, and don't ask again for commands that start with ` + "`head -1`" + ` (p)
  3. No, and tell Codex what to do differently (esc)

  Press enter to confirm or esc to cancel`

func TestCodexStartupGates_RealFrames(t *testing.T) {
	for name, frame := range map[string]string{
		"directory trust": codexDirTrustGate,
		"hooks review":    codexHooksReviewGate,
	} {
		if !IsStartupGate(frame, "codex") {
			t.Errorf("%s: IsStartupGate(codex) must be true", name)
		}
		// The default (Claude) set does NOT cover codex's wording — that gap is
		// why the codex entries exist. If this starts passing with agent "",
		// the codex signatures may be redundant; re-check before deleting them.
		if IsStartupGate(frame, "") {
			t.Errorf("%s: the default gate set unexpectedly matches a codex frame", name)
		}
		if IsComposerReady(frame, "codex") {
			t.Errorf("%s: a trust gate must not read composer-ready", name)
		}
		if r := NotReadyReason(frame, "codex"); !strings.Contains(r, "startup gate") {
			t.Errorf("%s: NotReadyReason should name the startup gate, got %q", name, r)
		}
		// Belt and braces: both gates render as a ›-selected numbered menu, so the
		// glyph-based menu detector refuses them independently of the signature.
		if WaitingOptions(frame) == nil {
			t.Errorf("%s: the gate menu should also be caught by WaitingOptions", name)
		}
	}
}

func TestCodexBootBanner_MCPStartingBlocks_StandingWarnDoesNot(t *testing.T) {
	// Mid-boot: composer already drawn, MCP still connecting → NOT ready, and the
	// diagnostic names the line. The default set can't catch this (its one-word
	// "Starting" must prefix the line; codex's starts with "• ").
	if IsComposerReady(codexBootMCP, "codex") {
		t.Error("mid-boot (Starting MCP servers) must not read composer-ready")
	}
	if line := BootBannerLine(codexBootMCP, "codex"); !strings.Contains(line, "Starting MCP servers") {
		t.Errorf("BootBannerLine should surface the MCP boot line, got %q", line)
	}
	if !IsComposerReady(codexBootMCP, "") {
		t.Error("fixture self-check: the default set was expected to MISS codex's boot line; " +
			"if it now catches it, the codex entry may be redundant")
	}

	// Settled idle with PERMANENT ⚠ MCP-failure notices: those are standing, not
	// boot chrome — the gate must be satisfiable or every spawn on this machine
	// times out into a false NOT delivered (the spawn-readiness-persistent-banner
	// lesson, replayed for codex).
	if !IsComposerReady(codexIdleStandingWarn, "codex") {
		t.Errorf("idle codex with standing MCP warnings must be composer-ready; reason: %q",
			NotReadyReason(codexIdleStandingWarn, "codex"))
	}
}

func TestCodexApprovalMenu_RealFrame(t *testing.T) {
	opts := WaitingOptions(codexApprovalMenu)
	if len(opts) != 3 {
		t.Fatalf("the approval menu should parse as 3 options, got %#v", opts)
	}
	if IsComposerReady(codexApprovalMenu, "codex") {
		t.Error("an approval menu must not read composer-ready")
	}
	if IsStartupGate(codexApprovalMenu, "codex") {
		t.Error("a mid-turn approval is not a STARTUP gate")
	}
	// The labels are clean single-column text — the one-tap reply card can drive it.
	if !OptionsReplyable(opts) {
		t.Errorf("codex's approval menu should be tap-replyable, got %#v", opts)
	}
}
