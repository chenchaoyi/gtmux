package codexbridge

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const hqThreadTitle = "gtmux HQ"

func seedHQThreadName(ctx context.Context, epoch *connectionEpoch, a resume.CodexFrontend, thread string) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if !epoch.current(a.Sequence) || !stampedHQFrontend(ctx, a) {
		return ""
	}
	var name string
	_ = epoch.publish(ctx, a.Sequence, func() {
		target := codexrpc.Target{HomeID: a.HomeID, ThreadID: thread, BackendPID: a.BackendPID, BackendStart: a.BackendStart}
		var reason string
		name, reason = codexrpc.SeedThreadName(ctx, target, hqThreadTitle, func() bool { return stampedHQFrontend(ctx, a) })
		if reason != "" {
			diag.For("codex-bridge").Warn("codex.hq.title.unavailable", "could not confirm the HQ thread title",
				"pane", a.Pane, "reason", reason)
		}
	})
	return name
}

func stampedHQFrontend(ctx context.Context, a resume.CodexFrontend) bool {
	const fields = "#{pane_id}\t#{pane_pid}\t#{pane_tty}\t#{" + hqpane.HomeOption + "}"
	out, err := exec.CommandContext(ctx, tmux.Bin, "-u", "display-message", "-p", "-t", a.Pane, fields).Output()
	return err == nil && matchesHQTitlePane(a, strings.TrimSuffix(string(out), "\n"), state.HQHome())
}

// A directory match is enough for recovering wake routing, but cannot
// authorize a title write. Require the stamp on this admitted terminal.
func matchesHQTitlePane(a resume.CodexFrontend, snapshot, home string) bool {
	f := strings.SplitN(snapshot, "\t", 4)
	return len(f) == 4 && a.Pane != "" && a.PanePID > 1 && a.TTY != "" &&
		f[0] == a.Pane && f[1] == strconv.Itoa(a.PanePID) && strings.TrimPrefix(f[2], "/dev/") == a.TTY &&
		hqpane.SameDir(f[3], home)
}

// Relay the acknowledged title on the root response itself. Codex can send
// the client's initial input as soon as it handles this response, before it
// processes the corresponding thread/name/updated notification.
func withRootName(data []byte, name string) []byte {
	var envelope, result, thread map[string]json.RawMessage
	if json.Unmarshal(data, &envelope) != nil ||
		json.Unmarshal(envelope["result"], &result) != nil ||
		json.Unmarshal(result["thread"], &thread) != nil || thread == nil {
		return data
	}
	thread["name"], _ = json.Marshal(name)
	result["thread"], _ = json.Marshal(thread)
	envelope["result"], _ = json.Marshal(result)
	out, err := json.Marshal(envelope)
	if err != nil {
		return data
	}
	return out
}
