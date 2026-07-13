package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmdSend types into a tmux pane (a WRITE) — it backs the menu-bar / notification
// in-place reply (A1/A2) and any scripted input. `gtmux send <pane> <text…>`
// delivers the text then Enter and, by DEFAULT, VERIFIES it landed (the same
// layered deliver-verify as `gtmux spawn`); `--no-verify` skips verification;
// `--force` overrides the re-send interlock; `--no-enter` skips Enter (implies
// no-verify); `--key NAME` sends a single whitelisted control key. `--key` and the
// server's POST /api/send path are UNCHANGED (the API stays fast — mobile latency).
func cmdSend(args []string) int {
	enter := true
	verify := true
	force := false
	key := ""
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--no-enter":
			enter = false
			verify = false // nothing was submitted to verify
		case a == "--enter":
			enter = true
		case a == "--no-verify":
			verify = false
		case a == "--force":
			force = true
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
	pane := rest[0]
	text := strings.Join(rest[1:], " ")

	if tmux.Bin == "" || tmux.Display(pane, "#{pane_id}") == "" {
		i18n.Sae("gtmux send: pane not found", "gtmux send: 找不到该 pane")
		return 1
	}
	if key != "" {
		if !allowedSendKeys[key] {
			i18n.Sae("gtmux send: key not allowed", "gtmux send: 不允许的按键")
			return 2
		}
		if err := tmux.SendKey(pane, key); err != nil {
			i18n.Sae("gtmux send: "+err.Error(), "gtmux send: "+err.Error())
			return 1
		}
		return 0
	}
	// Verified text delivery (default). Returns as soon as landing is confirmed, so a
	// healthy send stays fast.
	if verify && enter {
		paneID := tmux.Display(pane, "#{pane_id}")
		agentCmd := tmux.Display(paneID, "#{pane_current_command}")
		tune := dispatch.LoadTuning()
		res := dispatch.Deliver(dispatchIO(paneID), deliverOpts(paneID, agentCmd, force, tune), text)
		switch res.State {
		case dispatch.StateLanded:
			return 0
		case dispatch.StateQueued:
			i18n.Say("• queued — it will run after the current turn", "• 已排队 —— 当前这轮结束后执行")
			return 0
		case dispatch.StateRefusedDup:
			i18n.Sae("gtmux send: refused (identical payload re-sent within the window; use --force)",
				"gtmux send: 已拒发（时间窗内重复相同内容，要重发用 --force）")
			return 1
		default:
			i18n.Sae("gtmux send: NOT delivered — evidence:\n"+res.Evidence,
				"gtmux send: 未送达 —— 证据：\n"+res.Evidence)
			return 1
		}
	}
	// Plain (unverified) path: --no-verify or --no-enter.
	if err := tmux.SendText(pane, text, enter); err != nil {
		i18n.Sae("gtmux send: "+err.Error(), "gtmux send: "+err.Error())
		return 1
	}
	return 0
}

func sendUsage() int {
	i18n.Sae("usage: gtmux send <pane> <text…> [--no-enter] [--no-verify] [--force] [--key NAME]",
		"用法：gtmux send <pane> <text…> [--no-enter] [--no-verify] [--force] [--key 键名]")
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
