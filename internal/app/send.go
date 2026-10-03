package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmdSend types into a tmux pane (a WRITE) — it backs the menu-bar / notification
// in-place reply (A1/A2) and any scripted input. `gtmux send <pane> <text…>`
// delivers the text then Enter and, by DEFAULT, VERIFIES it landed (the same
// layered deliver-verify as `gtmux spawn`); `--no-verify` skips verification;
// `--force` overrides the re-send interlock; `--no-enter` skips Enter (implies
// no-verify); `--key NAME` sends a single whitelisted control key. Every text path
// here — verified or not — pastes and submits the same way; only the confirmation
// differs. The server's POST /api/send stays UNVERIFIED (the API is on the phone's
// latency budget), and `--key` is a single keystroke with nothing to verify.
func cmdSend(args []string) int {
	enter := true
	verify := true
	force := false
	asJSON := false
	key := ""
	msgFile := ""
	var attach []string
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--message-file":
			if i+1 >= len(args) {
				return sendUsage()
			}
			i++
			msgFile = args[i]
		case strings.HasPrefix(a, "--message-file="):
			msgFile = strings.TrimPrefix(a, "--message-file=")
		case a == "--attach":
			if i+1 >= len(args) {
				return sendUsage()
			}
			i++
			attach = append(attach, args[i])
		case strings.HasPrefix(a, "--attach="):
			attach = append(attach, strings.TrimPrefix(a, "--attach="))
		case a == "--no-enter":
			enter = false
			verify = false // nothing was submitted to verify
		case a == "--enter":
			enter = true
		case a == "--no-verify":
			verify = false
		case a == "--force":
			force = true
		case a == "--json":
			asJSON = true
		case a == "--key":
			if i+1 >= len(args) {
				return sendUsage()
			}
			i++
			key = args[i]
		case strings.HasPrefix(a, "--key="):
			key = strings.TrimPrefix(a, "--key=")
		case a == "-h" || a == "--help":
			return sendUsage()
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		return sendUsage()
	}
	// --json reports the VERIFIED delivery result (delivered/state/judged_by/
	// evidence); the unverified and single-key paths have no verdict to report.
	if asJSON && (!verify || !enter || key != "") {
		return sendUsage()
	}
	pane := rest[0]
	text := strings.Join(rest[1:], " ")
	// The SHELL-FREE message channel. `gtmux send <pane> <text…>` makes the caller hand
	// a shell a safe rendering of arbitrary prose — a guarantee no caller can hold, since
	// a backtick inside double quotes is EXECUTED rather than sent. --message-file takes
	// the bytes straight off disk (or stdin), and they reach the pane's input box
	// verbatim via the paste buffer's pipe.
	if msgFile != "" {
		if text != "" {
			i18n.Sae("gtmux send: --message-file and positional text are mutually exclusive; pass one of them",
				"gtmux send: --message-file 与位置参数文本只能二选一")
			return 2
		}
		if key != "" {
			i18n.Sae("gtmux send: --message-file cannot be combined with --key",
				"gtmux send: --message-file 不能与 --key 同时使用")
			return 2
		}
		m, err := dispatch.ReadPayload(msgFile, os.Stdin)
		if err != nil {
			// An attachment can be the whole message: a screenshot sent without a note. Only
			// an EMPTY message is waived, and only then; a file that cannot be read stays an error.
			if !errors.Is(err, dispatch.ErrPayloadEmpty) || len(attach) == 0 {
				i18n.Sae("gtmux send: --message-file: "+err.Error(), "gtmux send: --message-file: "+err.Error())
				return 2
			}
			m = ""
		}
		text = m
	}

	if len(attach) > 0 && key != "" {
		i18n.Sae("gtmux send: --attach cannot be combined with --key", "gtmux send: --attach 不能与 --key 同时使用")
		return 2
	}

	if tmux.Bin == "" || tmux.Display(pane, "#{pane_id}") == "" {
		i18n.Sae("gtmux send: pane not found", "gtmux send: 找不到该 pane")
		return 1
	}
	// Files are copied only once the pane is known to exist, so a typo'd pane leaves
	// nothing behind in the uploads dir.
	var attached []string
	if len(attach) > 0 {
		var err error
		if attached, err = attachFiles(attach); err != nil {
			i18n.Sae("gtmux send: --attach: "+err.Error(), "gtmux send: --attach: "+err.Error())
			return 2
		}
		text = withAttachments(text, attached)
		// A message carrying a file is never typed into an agent that is waiting on the user:
		// its text and Enter could answer a permission prompt or a question. Asked here, and
		// again inside the delivery right before the paste and before every Enter (the
		// dispatch Hold below). A plain `gtmux send` is unchanged: answering a prompt is one
		// of its jobs.
		if why := attachHold(tmux.Display(pane, "#{pane_id}")); why != "" {
			return refusedWaiting(paneID(pane), text, why, attached, asJSON)
		}
	}
	if key != "" {
		if !allowedSendKeys[key] {
			i18n.Sae("gtmux send: key not allowed", "gtmux send: 不允许的按键")
			return 2
		}
		// A pane in copy/view-mode eats the key as a mode-nav command; drop out first.
		_ = tmux.ExitCopyMode(pane)
		err := tmux.SendKey(pane, key)
		diag.Did("act.send", paneID(pane), diag.Outcome(err), "pressed a key in a pane", "key", key, "error", err)
		if err != nil {
			i18n.Sae("gtmux send: "+err.Error(), "gtmux send: "+err.Error())
			return 1
		}
		return 0
	}
	// Verified text delivery (default). Returns as soon as landing is confirmed, so a
	// healthy send stays fast.
	if verify && enter {
		paneID := tmux.Display(pane, "#{pane_id}")
		// Resolve the agent from the pane's process subtree (Claude Code reports its
		// version as pane_current_command, so the foreground command misses "claude" and
		// hook-equipped would wrongly be false — see radar.AgentDriverKey).
		agentCmd, plain := resolvePaneAgent(paneID)
		if plain {
			// PLAIN TERMINAL: no agent input box to verify against — the box-confirm
			// pipeline false-fails on stale box borders in scrollback (see plainsend.go).
			// Type + Enter directly. No re-send interlock either: running the same shell
			// command twice in a row is normal shell usage, not a double-dispatch. No
			// MarkAwaited — there is no agent whose completion could be awaited.
			_ = tmux.ExitCopyMode(paneID)
			if err := typePlain(paneID, text); err != nil {
				events.AuditSend(paneID, "failed", text, time.Now().Unix())
				i18n.Sae("gtmux send: "+err.Error(), "gtmux send: "+err.Error())
				return 1
			}
			events.AuditSend(paneID, statePlainSent, text, time.Now().Unix())
			if asJSON {
				b, _ := json.Marshal(sendJSON{Delivered: true, State: statePlainSent, Attachments: attached})
				fmt.Println(string(b))
			}
			return 0
		}
		tune := dispatch.LoadTuning()
		dio := dispatchbridge.DispatchIO(paneID, agentCmd)
		if len(attached) > 0 {
			dio.Hold = func() string { return attachHold(paneID) }
		}
		res := dispatch.Deliver(dio, dispatchbridge.DeliverOpts(paneID, agentCmd, force, tune), text)
		if res.State == dispatch.StateRefusedWaiting {
			return refusedWaiting(paneID, text, res.Evidence, attached, asJSON)
		}
		// The sender's side of the story (hq-action-journal): the target pane's hook
		// event carries the prompt's head but cannot say who drove it, and the
		// interlock keeps only an overwritten hash. Refusals are journaled too — the
		// attempt is part of the audit.
		events.AuditSend(paneID, string(res.State), text, time.Now().Unix())
		if res.Delivered {
			// HQ (or whoever drives `gtmux send`) awaits this pane's completion
			// (done-wake-keyed-on-awaited): mark it so its next `done` wakes HQ even when
			// the pane is attended — the send-driven case a plain attended-defer dropped.
			dispatch.MarkAwaited(paneID)
			// Close the loop on a RESCUE. The documented recovery for a spawn that failed
			// its ready gate is to re-send the same goal into the pane it left behind; the
			// ledger entry still says the dispatch never landed, so without this the row
			// would read `undelivered` forever while the worker is busy doing the job.
			// No-op for a pane with no entry, or one that already landed.
			dispatch.MarkDelivered(paneID, text, time.Now().Unix())
		}
		if asJSON {
			b, _ := json.Marshal(sendJSON{
				Delivered: res.Delivered, State: string(res.State),
				JudgedBy: res.JudgedBy, Evidence: res.Evidence, Attachments: attached,
			})
			fmt.Println(string(b))
			if res.Delivered || res.State == dispatch.StateQueued {
				return 0
			}
			return 1
		}
		switch res.State {
		case dispatch.StateLanded:
			return 0
		case dispatch.StateQueued:
			i18n.Say("• queued; it will run after the current turn", "• 已排队，当前这轮结束后执行")
			return 0
		case dispatch.StateRefusedDup:
			i18n.Sae("gtmux send: refused (identical payload re-sent within the window; use --force)",
				"gtmux send: 已拒发（时间窗内重复相同内容，要重发用 --force）")
			return 1
		case dispatch.StateRefusedDraft:
			i18n.Sae("gtmux send: refused. That pane has unsent text in its input box; sending would\n"+
				"append to it and submit both. Clear it, or use --force.\n"+res.Evidence,
				"gtmux send: 已拒发，该 pane 的输入框里有未提交的内容，发送会拼在它后面一起提交。\n"+
					"请先清空，或用 --force。\n"+res.Evidence)
			return 1
		default:
			i18n.Sae("gtmux send: not delivered. Evidence:\n"+res.Evidence,
				"gtmux send: 未送达。证据：\n"+res.Evidence)
			return 1
		}
	}
	// Plain (unverified) path: --no-verify or --no-enter. It skips the post-submit
	// LANDED verification, not the delivery mechanics — text still rides a paste buffer
	// and Enter is still a separate key. For a text+Enter send it still confirms the
	// DRAFT before submitting (dispatch.PasteAndSubmit) so a multi-line message is not
	// raced into a truncated submit; --no-enter just stages the paste. Drop out of
	// copy/view-mode first — otherwise the pane swallows the text and Enter as mode-nav.
	_ = tmux.ExitCopyMode(pane)
	if text != "" && enter {
		id := paneID(pane)
		// A plain terminal takes the direct type path here too — the draft-confirm
		// inside PasteAndSubmit trips on the same stale-scrollback-box false negative
		// and silently withholds the Enter (see plainsend.go).
		agentCmd, plain := resolvePaneAgent(id)
		if plain {
			if err := typePlain(id, text); err != nil {
				events.AuditSend(id, "failed", text, time.Now().Unix())
				i18n.Sae("gtmux send: "+err.Error(), "gtmux send: "+err.Error())
				return 1
			}
			events.AuditSend(id, statePlainSent, text, time.Now().Unix())
			return 0
		}
		// The unverified path still protects a draft — skipping VERIFICATION was never a
		// request to overwrite someone's unsubmitted line. `--force` waives it.
		// Build the opts through the bridge so this path gets the SAME draft-guard gating
		// (HasComposer) as the verified one — a hand-rolled Opts here is how the two
		// would drift apart.
		popts := dispatchbridge.DeliverOpts(id, agentCmd, force, dispatch.LoadTuning())
		popts.PasteRetries = 2
		pio := dispatchbridge.DispatchIO(id)
		if len(attached) > 0 {
			pio.Hold = func() string { return attachHold(id) }
		}
		_, refused := dispatch.PasteAndSubmit(pio, popts, text)
		if refused == dispatch.StateRefusedWaiting {
			return refusedWaiting(id, text, "the agent started asking before the message was submitted", attached, asJSON)
		}
		if refused == dispatch.StateRefusedDraft {
			events.AuditSend(id, string(dispatch.StateRefusedDraft), text, time.Now().Unix())
			i18n.Sae("gtmux send: refused. That pane has unsent text in its input box (use --force)",
				"gtmux send: 已拒发，该 pane 的输入框里有未提交的内容（要覆盖请用 --force）")
			return 1
		}
		events.AuditSend(id, statePlainSent, text, time.Now().Unix())
		return 0
	}
	if text != "" {
		// Single-line, no submit → literal keystrokes (a numbered menu commits on the
		// digit keypress; a bracketed paste of "1" is inserted as text and selects
		// nothing). Multi-line still uses the paste buffer so newlines don't submit
		// line-by-line. Mirrors sendToPane (POST /api/send).
		var err error
		if keystrokeText(text) {
			err = tmux.SendText(pane, text, false)
		} else {
			err = tmux.Paste(pane, text)
		}
		if err != nil {
			i18n.Sae("gtmux send: "+err.Error(), "gtmux send: "+err.Error())
			return 1
		}
	}
	if enter {
		if err := tmux.SendKey(pane, "Enter"); err != nil {
			i18n.Sae("gtmux send: "+err.Error(), "gtmux send: "+err.Error())
			return 1
		}
	}
	if text != "" {
		events.AuditSend(paneID(pane), "staged", text, time.Now().Unix())
	}
	return 0
}

// paneID resolves a pane target to its stable %id (dispatchIO keys everything off it).
func paneID(pane string) string {
	if id := tmux.Display(pane, "#{pane_id}"); id != "" {
		return id
	}
	return pane
}

// statePlainSent is the `--json` state reported for a PLAIN-TERMINAL send: the keys
// were delivered to the pane and Enter was sent, but there is no agent input box or
// receipt to verify a landing against, so the verified states don't apply.
const statePlainSent = "sent"

// sendJSON is the `gtmux send --json` contract (verified sends, plus the
// plain-terminal "sent" state — see statePlainSent).
type sendJSON struct {
	Delivered bool   `json:"delivered"`
	State     string `json:"state"`
	JudgedBy  string `json:"judged_by,omitempty"` // driver | screen — the layer that judged it
	Evidence  string `json:"evidence,omitempty"`
	// Attachments are the uploads-dir paths `--attach` added to the message, in order.
	Attachments []string `json:"attachments,omitempty"`
}

func sendUsage() int {
	i18n.Sae("usage: gtmux send <pane> (--message-file <path|-> | <text…>) [--attach FILE]… [--no-enter] [--no-verify] [--force] [--json] [--key NAME]\n  --message-file reads the message from a file (or - for stdin). Use it for anything\n  longer than one short line: text passed as an argument must survive shell parsing first.\n  --attach copies a file (≤30 MB) into gtmux's uploads dir and adds its path on a line of its own.",
		"用法：gtmux send <pane> (--message-file <文件|-> | <text…>) [--attach 文件]… [--no-enter] [--no-verify] [--force] [--json] [--key 键名]\n  --message-file 从文件（或 - 即 stdin）读取消息；超过一行的内容都用它：\n  作为命令行参数传的文本必须先过 shell 解析。\n  --attach 把文件（≤30 MB）拷进 gtmux 的 uploads 目录，路径单独占一行附在消息后。")
	return 2
}

// cmdOptions prints a waiting pane's interactive choice block as JSON
// ([{n,label}…]) using the shared parser — the menu-bar / notifications call this
// to render the 1/2/3 reply buttons. Empty array when there's no parseable menu.
func cmdOptions(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		i18n.Sae("usage: gtmux options <pane>", "用法：gtmux options <pane>")
		return 2
	}
	opts := prompt.ParseOptions(tmux.CapturePane(args[0]))
	if opts == nil {
		opts = []prompt.Option{}
	}
	b, _ := json.Marshal(opts)
	fmt.Println(string(b))
	return 0
}

// refusedWaiting reports a message with a file that was not typed, or not submitted,
// because the agent is asking the user something. why says what was seen.
func refusedWaiting(pane, text, why string, attached []string, asJSON bool) int {
	st := string(dispatch.StateRefusedWaiting)
	events.AuditSend(pane, st, text, time.Now().Unix())
	if asJSON {
		b, _ := json.Marshal(sendJSON{State: st, Evidence: why, Attachments: attached})
		fmt.Println(string(b))
	}
	i18n.Sae("gtmux send: refused. The agent is waiting on a decision (a permission prompt or a question); answer it first.\n"+why,
		"gtmux send: 已拒发，该 agent 正在等你做决定（权限提示或提问），请先回应它。\n"+why)
	return 1
}
