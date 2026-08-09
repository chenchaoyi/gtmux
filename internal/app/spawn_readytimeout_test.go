package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/radar"
)

// A ready-gate timeout is the one spawn failure a reader has to be able to ACT on, and
// it used to be the least readable: `agent composer not ready within the ready timeout`
// followed by the FULL capture — measured at 224 lines / 11.8 KB of live agent TUI on
// the machine that hit this. It named no blocker, so a pane held by a permanent banner
// read as "the agent is slow"; and the `✗ NOT delivered` line that introduces it was the
// head of a wall that looks like an ordinary startup log, so a real failure (stderr,
// exit 1) was reported as "spawn didn't error".
//
// Both halves are pinned here: the blocker leads, and the capture is bounded.
func TestReadyTimeoutEvidence(t *testing.T) {
	capture := strings.Repeat("scrollback line\n", 200) +
		" ⚠ 10 MCP servers need authentication · run /mcp\n❯ "
	got := readyTimeoutEvidenceOf("a startup banner is on screen: Connecting to MCP servers…", capture)

	first := strings.SplitN(got, "\n", 2)[0]
	if !strings.Contains(first, "Connecting to MCP servers…") {
		t.Errorf("the FIRST line must name the blocker, got %q", first)
	}
	if !strings.Contains(first, "not ready within the ready timeout") {
		t.Errorf("the first line must still say what failed, got %q", first)
	}
	if n := strings.Count(got, "\n") + 1; n > 20 {
		t.Errorf("evidence is %d lines — the verdict drowns in its own proof", n)
	}
	// The tail is the bottom of the screen, where the blocker lives.
	if !strings.Contains(got, "run /mcp") {
		t.Error("the evidence tail must carry the pane's bottom region")
	}

	// A pane that vanished has no capture — the verdict still stands alone, with no
	// trailing blank line pretending to be evidence.
	bare := readyTimeoutEvidenceOf("the pane is gone", "")
	if strings.Contains(bare, "\n") {
		t.Errorf("with no capture the evidence is one line, got %q", bare)
	}
}

// The digest must not file a never-delivered dispatch under "completed" — by PANE status
// alone it does, because a failed dispatch leaves a live, empty, idle agent. That is the
// same false signal `gtmux tasks` used to give, on the surface HQ actually reads.
func TestDigestBucket_UndeliveredNeedsYou(t *testing.T) {
	row := radar.DigestRow{Status: "idle", Task: "implement the thing",
		TaskStatus: radar.TaskStatusUndelivered}
	if got := digestBucket(row); got != "needs_input" {
		t.Errorf("digestBucket = %q, want needs_input", got)
	}
	if got := digestBadge(row); got != radar.TaskStatusUndelivered {
		t.Errorf("digestBadge = %q, want %q", got, radar.TaskStatusUndelivered)
	}
	// A delivered dispatch on the same idle pane is genuinely completed.
	done := radar.DigestRow{Status: "idle", Task: "implement the thing", TaskStatus: "done"}
	if got := digestBucket(done); got != "completed" {
		t.Errorf("a delivered+idle dispatch = %q, want completed", got)
	}
}
