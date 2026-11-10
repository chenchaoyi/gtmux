package app

import (
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/screenshot"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

var screenshotCurrentTarget = currentScreenshotTarget
var screenshotPhysical = screenshotPhysicalIdentity
var screenshotDispatch = dispatch.Deliver
var screenshotSaveAttachment = saveAttachment
var screenshotHold = attachHold
var screenshotIO = func(pane, agent string) dispatch.IO { return dispatchbridge.DispatchIO(pane, agent) }

func screenshotPhysicalIdentity(pane string) (int, string, int, string, error) {
	if !strings.HasPrefix(pane, "%") || len(pane) < 2 || strings.Trim(pane[1:], "0123456789") != "" {
		return 0, "", 0, "", screenshot.Fail("target-invalid", "Choose an exact %N pane")
	}
	f := strings.Split(tmux.Display(pane, "#{pane_id}\t#{pane_pid}\t#{session_name}:#{window_index}.#{pane_index}\t#{pid}"), "\t")
	if len(f) != 4 || f[0] != pane {
		return 0, "", 0, "", screenshot.Fail("target-unavailable", "The selected pane is unavailable")
	}
	root, _ := strconv.Atoi(f[1])
	server, _ := strconv.Atoi(f[3])
	p, ok := launchhost.ReadAgent(server)
	if root <= 1 || server <= 1 || !ok || p.Start == "" {
		return 0, "", 0, "", screenshot.Fail("target-unverified", "Could not verify the tmux server incarnation")
	}
	return root, f[2], server, p.Start, nil
}

func currentScreenshotTarget(pane string) (screenshot.Target, error) {
	var t screenshot.Target
	root, loc, server, start, err := screenshotPhysical(pane)
	if err != nil {
		return t, err
	}
	t = screenshot.Target{Version: 1, Pane: pane, Loc: loc, PanePID: root, ServerPID: server, ServerStart: start}
	if f, ok := resume.CodexFrontendForPane(pane); ok && f.PanePID == root && f.Loc == loc {
		t.Agent, t.SessionID, t.HomeID, t.Instance, t.ClientPID = "codex", f.SessionID, f.HomeID, f.Instance, f.PID
		return t, nil
	}
	if a, ok := agentclient.ForPane(pane); ok && a.InputReady {
		t.Agent, t.SessionID, t.HomeID, t.Instance, t.ClientPID = a.Agent, a.SessionID, a.HomeID, a.Instance, a.PID
		return t, nil
	}
	return t, screenshot.Fail("target-unverified", "An exact managed Codex or DSH native binding is required; other clients retain gtmux send --attach")
}

func screenshotResolveTarget(pane, session string) (any, error) {
	t, err := screenshotCurrentTarget(pane)
	if err != nil {
		return nil, err
	}
	if session != "" && session != t.SessionID {
		return nil, screenshot.Fail("target-changed", "The selected pane belongs to another conversation")
	}
	now := time.Now()
	t.ExpiresAt = now.Add(10 * time.Minute).Unix()
	token, err := screenshot.MintTarget(t, now)
	if err != nil {
		return nil, err
	}
	return map[string]any{"target_ref": token, "target": t}, nil
}

func screenshotSend(ctx context.Context, path, token, id, notePath string) (screenshot.Receipt, error) {
	var empty screenshot.Receipt
	t, err := screenshot.ParseTarget(token, time.Now())
	if err != nil {
		return empty, err
	}
	current, err := screenshotCurrentTarget(t.Pane)
	if err != nil || !t.SameBinding(current) {
		return empty, screenshot.Fail("target-changed", "The target changed; resolve it again before a new submission")
	}
	f, err := screenshot.OpenImage(path, "")
	if err != nil {
		return empty, err
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > screenshot.MaxPixels || config.Height > screenshot.MaxPixels || int64(config.Width)*int64(config.Height) > screenshot.MaxPixels {
		return empty, screenshot.Fail("invalid-image", "Send a bounded PNG/JPEG image; import other formats first")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return empty, err
	}
	data, err := io.ReadAll(io.LimitReader(f, screenshot.MaxImage+1))
	if err != nil || len(data) > screenshot.MaxImage {
		return empty, screenshot.Fail("invalid-image", "Image exceeded its byte budget")
	}
	note := ""
	if notePath != "" {
		note, err = dispatch.ReadPayload(notePath, os.Stdin)
		if err != nil && !errors.Is(err, dispatch.ErrPayloadEmpty) {
			return empty, screenshot.Fail("note-unavailable", "The message file could not be read")
		}
	}
	r, claimed, err := screenshot.Claim(id, token, screenshot.Digest(data), note)
	if err != nil {
		return r, err
	}
	if !claimed {
		if r.State == "unknown" {
			return r, screenshot.Fail("delivery-unconfirmed", "This request may already have been accepted; check its status instead of resubmitting")
		}
		if r.State == "blocked" || r.State == "failed" {
			return r, screenshot.Fail("send-blocked", "This request was blocked; its stored receipt was preserved")
		}
		return r, nil
	}
	finish := func(status, reason string, confirmed bool) (screenshot.Receipt, error) {
		out, err := screenshot.Finish(r, status, reason, confirmed)
		if err != nil {
			return r, err
		}
		if status == "unknown" {
			return out, screenshot.Fail("delivery-unconfirmed", "Input may have been accepted; do not resubmit this request")
		}
		if status == "blocked" || status == "failed" {
			return out, screenshot.Fail("send-blocked", "Submission was blocked; inspect the receipt and target before creating another request")
		}
		return out, nil
	}
	if ctx.Err() != nil {
		return finish("blocked", "cancelled-before-submission", false)
	}
	attached, err := screenshotSaveAttachment("screenshot.png", data)
	if err != nil {
		return finish("failed", "attachment-unavailable", false)
	}
	text := withAttachments(note, []string{attached})
	dio := screenshotIO(t.Pane, t.Agent)
	if t.Agent == "codex" && dio.SubmitNative == nil {
		return finish("blocked", "native-binding-changed", false)
	}
	guard := func() string {
		if ctx.Err() != nil {
			return "screenshot request deadline expired"
		}
		root, loc, server, start, err := screenshotPhysical(t.Pane)
		if err != nil || root != t.PanePID || loc != t.Loc || server != t.ServerPID || start != t.ServerStart {
			return "screenshot target incarnation changed"
		}
		return screenshotHold(t.Pane)
	}
	dio.Hold = guard
	protect := func(original func() error) func() error {
		return func() error {
			if why := guard(); why != "" {
				return errors.New(why)
			}
			return original()
		}
	}
	if dio.Enter != nil {
		dio.Enter = protect(dio.Enter)
	}
	if dio.ExitMode != nil {
		dio.ExitMode = protect(dio.ExitMode)
	}
	if dio.ClearDraft != nil {
		dio.ClearDraft = protect(dio.ClearDraft)
	}
	if original := dio.Paste; original != nil {
		dio.Paste = func(value string) error {
			if why := guard(); why != "" {
				return errors.New(why)
			}
			return original(value)
		}
	}
	opts := dispatchbridge.DeliverOpts(t.Pane, t.Agent, false, dispatch.LoadTuning())
	opts.DeliveryID, opts.ActiveImageInput, opts.Context = id, true, ctx
	opts.ExpectedSessionID, opts.ExpectedHomeID, opts.ExpectedInstance, opts.ExpectedClientPID = t.SessionID, t.HomeID, t.Instance, t.ClientPID
	if deadline, ok := ctx.Deadline(); ok {
		remaining := int64(time.Until(deadline) / time.Second)
		if remaining <= 0 {
			return finish("blocked", "deadline-before-submission", false)
		}
		if remaining < opts.DeliverTimeout {
			opts.DeliverTimeout = remaining
		}
	}
	if guard() != "" {
		return finish("blocked", "target-waiting-or-changed", false)
	}
	result := screenshotDispatch(dio, opts, text)
	events.AuditSend(t.Pane, string(result.State), text, time.Now().Unix())
	outcome := diag.Failed
	if result.Delivered || result.State == dispatch.StateQueued {
		outcome = diag.OK
	}
	diag.Did("act.screenshot.agent.send", t.Pane, outcome, "recorded screenshot input receipt", "state", string(result.State))
	if result.Delivered && result.State == dispatch.StateLanded {
		return finish("landed", "input-receipt", true)
	}
	if !result.Delivered && result.State == dispatch.StateQueued {
		if strings.HasPrefix(result.Evidence, "codex-active-") {
			return finish("active_turn_accepted", "current-turn-accepted", true)
		}
		return finish("queued", "accepted-for-queue-not-confirmed-in-conversation", false)
	}
	if strings.HasPrefix(string(result.State), "refused-") {
		return finish("blocked", string(result.State), false)
	}
	return finish("unknown", "no-confirmed-input-receipt", false)
}
