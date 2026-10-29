package dispatchbridge

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

var codexInputTarget = resume.CodexFrontendForPane
var codexQueueInputs = codexrpc.QueueInputs

func codexNativeSubmit(pane string) func(dispatch.Opts, string, func() string) dispatch.Result {
	return func(opts dispatch.Opts, text string, hold func() string) dispatch.Result {
		f, ok := codexInputTarget(pane)
		if !ok {
			return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: "codex-frontend-unavailable"}
		}
		a := agentclient.Attachment{Version: agentclient.ProtocolVersion, Agent: "codex", Client: "codex", Pane: pane,
			PID: f.PID, Instance: f.Instance, HomeID: f.HomeID, SessionID: f.SessionID, Cwd: f.Cwd, ServerEpoch: f.RootStart,
			BindingEpoch: f.StartedAt, StartedAt: f.StartedAt, UpdatedAt: time.Now().UnixMilli(), InputReady: true, NativeSession: true}
		r, err := agentclient.Prepare(a, text, opts.DeliveryID, opts.Force && opts.DeliveryID == "", time.Now())
		if err != nil {
			return dispatch.Result{State: dispatch.StateFailed, Evidence: "codex-input-retry-held: " + err.Error()}
		}
		if receipt, known := agentclient.ReadReceipt(r); known && receipt.State == "accepted" {
			return dispatch.Result{State: dispatch.StateQueued, JudgedBy: dispatch.JudgedByDriver, Evidence: "codex-queue-already-accepted"}
		}
		claimed := false
		check := func() bool {
			current, valid := codexInputTarget(pane)
			if !valid || current.Instance != f.Instance || current.HomeID != f.HomeID || current.SessionID != f.SessionID || (hold != nil && hold() != "") {
				return false
			}
			if agentclient.ClaimNative(r) != nil {
				return false
			}
			claimed = true
			return true
		}
		outcome := codexQueueInputs(context.Background(), codexrpc.Target{HomeID: f.HomeID, ThreadID: f.SessionID, BackendPID: f.BackendPID, BackendStart: f.BackendStart}, r.ID, codexInputs(text), check)
		if outcome.Accepted {
			if err := agentclient.FinishNative(r, "accepted", ""); err != nil {
				return dispatch.Result{State: dispatch.StateFailed, Evidence: "codex-input-receipt-unconfirmed"}
			}
			return dispatch.Result{State: dispatch.StateQueued, JudgedBy: dispatch.JudgedByDriver, Evidence: "codex-queue-accepted:" + outcome.QueueID, Attempts: 1}
		}
		if claimed {
			verdict := "uncertain"
			if !outcome.Attempted {
				verdict = "refused"
			}
			_ = agentclient.FinishNative(r, verdict, outcome.Reason)
			return dispatch.Result{State: dispatch.StateFailed, Evidence: "codex-input-" + verdict + ":" + outcome.Reason}
		}
		return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: outcome.Reason}
	}
}

// Uploads already authorized as attachments retain their image semantics. Other
// files stay text paths. Never infer an image from arbitrary prose or follow a
// file symlink outside the private uploads directory.
func codexInputs(text string) []codexrpc.Input {
	inputs := []codexrpc.Input{{Type: "text", Text: text}}
	root := filepath.Join(state.Dir(), "uploads")
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		path := strings.TrimSpace(line)
		rel, err := filepath.Rel(root, path)
		if err != nil || !filepath.IsAbs(path) || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || seen[path] || len(inputs) >= 17 {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		file.Close()
		kind := http.DetectContentType(buf[:n])
		if kind != "image/png" && kind != "image/jpeg" && kind != "image/gif" && kind != "image/webp" {
			continue
		}
		seen[path] = true
		inputs = append(inputs, codexrpc.Input{Type: "localImage", Path: path})
	}
	return inputs
}
