package connect

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"golang.org/x/term"
)

// Run implements `gtmux attach` — attach to a remote gtmux pane in the local terminal.
//
//	gtmux attach <host> --token <tok> [%pane]     # owner (full)
//	gtmux attach https://host/#g=<token> [%pane]  # guest (scope-restricted; legacy #t= accepted)
//	  --read-only   watch only, never send input
func Run(args []string) int {
	var target, token, pane string
	readOnly := false
	predict := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			return usage()
		case a == "--read-only" || a == "-r":
			readOnly = true
		case a == "--predict":
			predict = true
		case a == "--token":
			if i+1 < len(args) {
				token = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
		case strings.HasPrefix(a, "%"):
			pane = a
		case strings.HasPrefix(a, "-"):
			i18n.Sae("gtmux attach: unknown flag "+a, "gtmux attach: 未知参数 "+a)
			return 2
		default:
			if target == "" {
				target = a
			} else if pane == "" {
				pane = a
			}
		}
	}

	tgt, err := ParseTarget(target, token)
	if err != nil {
		i18n.Sae("gtmux attach: "+err.Error(), "gtmux attach: "+err.Error())
		return 2
	}

	ctx := context.Background()

	// A PAIR link (pair-share-model S2): redeem the one-time code for this
	// terminal's OWN device token, persist it (remotes.json, 0600), and proceed as
	// owner — a later bare `gtmux attach <host>` reuses it. Revocation on the host
	// (`gtmux pair revoke`) kills the persisted token instantly.
	if tgt.EnrollCode != "" {
		host, _ := os.Hostname()
		tok, err := RedeemEnrollCode(ctx, tgt.URL, tgt.EnrollCode, host)
		if err != nil {
			i18n.Sae("gtmux attach: pairing failed ("+err.Error()+")",
				"gtmux attach: 配对失败（"+err.Error()+"）")
			return 1
		}
		tgt.Token = tok
		if err := SaveRemoteToken(tgt.URL, tok); err == nil {
			i18n.Sae("paired this terminal with "+tgt.URL+" — next time just: gtmux attach "+tgt.URL,
				"本终端已与 "+tgt.URL+" 配对 —— 下次直接：gtmux attach "+tgt.URL)
		}
	}

	c := NewClient(tgt.URL, tgt.Token)
	if !c.Health(ctx) {
		i18n.Sae("gtmux attach: can't reach "+tgt.URL, "gtmux attach: 连不上 "+tgt.URL)
		return 1
	}
	cap, err := c.Share(ctx)
	if err != nil {
		i18n.Sae("gtmux attach: auth failed ("+err.Error()+")", "gtmux attach: 鉴权失败（"+err.Error()+"）")
		return 1
	}
	isGuest := !cap.All

	// Resolve the pane: use the given one, else auto-pick when exactly one is
	// attachable, else list the choices.
	if pane == "" {
		p, code := pickPane(ctx, c, isGuest)
		if code != 0 {
			return code
		}
		pane = p
	}

	// A guest may type only into an input-allowed pane; force read-only otherwise (the
	// server enforces this too, but the client shouldn't pretend it can type).
	if isGuest && !contains(cap.Panes, pane) {
		readOnly = true
	}

	who := i18n.Tr("owner", "本人")
	if isGuest {
		who = i18n.Tr("guest", "访客")
	}
	mode := ""
	if readOnly {
		mode = i18n.Tr(" · read-only", " · 只读")
	}
	fmt.Fprintf(os.Stderr, "%s %s %s %s%s\n",
		i18n.Tr("attaching", "正在附着"), pane,
		i18n.Tr("as", "以"), who, mode)
	i18n.Sae("(detach: tmux prefix + d, or Ctrl-])", "（退出：tmux 前缀键 + d，或 Ctrl-]）")

	if err := RunAttach(tgt.URL, tgt.Token, pane, readOnly, predict); err != nil {
		i18n.Sae("gtmux attach: "+err.Error(), "gtmux attach: "+err.Error())
		return 1
	}
	return 0
}

// pickPane auto-selects the single attachable tmux pane, or lists the choices.
func pickPane(ctx context.Context, c *Client, isGuest bool) (string, int) {
	agents, err := c.Agents(ctx)
	if err != nil {
		i18n.Sae("gtmux attach: "+err.Error(), "gtmux attach: "+err.Error())
		return "", 1
	}
	var panes []Agent
	for _, a := range agents {
		if a.Source != "native" && a.PaneID != "" {
			panes = append(panes, a)
		}
	}
	if len(panes) == 0 {
		i18n.Sae("gtmux attach: no attachable sessions", "gtmux attach: 没有可附着的会话")
		return "", 1
	}
	if len(panes) == 1 {
		return panes[0].PaneID, 0
	}
	// Many panes, no %pane given. On a TTY, offer a numbered menu and attach to the
	// chosen row in place — no re-run. Off a TTY (pipe / script / CI), keep the old
	// list-and-exit so automation never blocks on a prompt.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		i18n.Sae("gtmux attach: name a pane —", "gtmux attach: 指定一个 pane —")
		for _, a := range panes {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", a.PaneID, formatPaneChoice(a))
		}
		return "", 2
	}
	return promptPane(panes, os.Stdin)
}

// promptPane renders a numbered menu of panes and reads a choice from r (cooked mode,
// so the number + Enter echo normally). Enter picks row 1; `q`/EOF cancels. Invalid
// input re-prompts. Returns the chosen pane id, or ("", code) on cancel.
func promptPane(panes []Agent, r *os.File) (string, int) {
	i18n.Sae("gtmux attach: which pane?", "gtmux attach: 附着哪个 pane？")
	for i, a := range panes {
		mark := " "
		if i == 0 {
			mark = "›" // the default (Enter) row
		}
		fmt.Fprintf(os.Stderr, "  %s %2d) %s  %s\n", mark, i+1, a.PaneID, formatPaneChoice(a))
	}
	br := bufio.NewReader(r)
	for {
		fmt.Fprintf(os.Stderr,
			i18n.Tr("attach to which? [1-%d, Enter=1, q=cancel] ",
				"附着哪个？[1-%d，回车=1，q=取消] "), len(panes))
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			i18n.Sae("cancelled.", "已取消。") // EOF (Ctrl-D)
			return "", 1
		}
		idx, cancel, ok := parsePaneChoice(line, len(panes))
		if cancel {
			i18n.Sae("cancelled.", "已取消。")
			return "", 1
		}
		if !ok {
			i18n.Sae("  not a valid choice — enter a number, or q to cancel.",
				"  无效选择 —— 输入数字，或 q 取消。")
			continue
		}
		return panes[idx].PaneID, 0
	}
}

// parsePaneChoice interprets one menu line for n panes: empty ⇒ the default row 0;
// `q`/`quit`/a leading ESC ⇒ cancel; a number in [1,n] ⇒ that row (0-based). Anything
// else is invalid (ok=false) so the caller re-prompts.
func parsePaneChoice(input string, n int) (idx int, cancel, ok bool) {
	s := strings.TrimSpace(input)
	switch {
	case s == "":
		return 0, false, true
	case s == "q" || s == "Q" || s == "quit" || strings.HasPrefix(input, "\x1b"):
		return 0, true, true
	}
	if v, err := strconv.Atoi(s); err == nil && v >= 1 && v <= n {
		return v - 1, false, true
	}
	return 0, false, false
}

// formatPaneChoice is the one-line label for a pane in the picker / list: its identity
// (session · agent · status) plus a truncated task. Falls back to the pane id if the
// row carries no descriptive fields.
func formatPaneChoice(a Agent) string {
	var id []string
	if a.Session != "" {
		id = append(id, a.Session)
	}
	if a.Agent != "" {
		id = append(id, a.Agent)
	}
	if a.Status != "" {
		id = append(id, a.Status)
	}
	label := strings.Join(id, " · ")
	task := a.Task
	if r := []rune(task); len(r) > 60 {
		task = string(r[:57]) + "…"
	}
	if task != "" {
		if label != "" {
			label += "  "
		}
		label += task
	}
	if label == "" {
		label = a.PaneID
	}
	return label
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func usage() int {
	i18n.Sae(
		"usage: gtmux attach <host|pair-link|share-link> [%pane] [--token <tok>] [--read-only] [--predict]\n"+
			"  Attach to a remote gtmux pane in your local terminal (raw, interactive).\n"+
			"  A pair link (…/#c=<code>, from `gtmux pair`) enrolls THIS terminal as one of\n"+
			"  your own devices (full control, token persisted — later just `gtmux attach <host>`).\n"+
			"  A share link (…/#g=<token>) connects as a scope-restricted guest; a host +\n"+
			"  --token also works. Detach with tmux `prefix d` or Ctrl-].",
		"用法：gtmux attach <host|配对链接|分享链接> [%pane] [--token <tok>] [--read-only] [--predict]\n"+
			"  在本地终端里附着到远程 gtmux 的 pane（原生、可交互）。\n"+
			"  配对链接（…/#c=<code>，来自 `gtmux pair`）把本终端登记为你自己的设备\n"+
			"  （全权,token 会保存 —— 之后直接 `gtmux attach <host>`）。\n"+
			"  分享链接（…/#g=<token>）以受限访客接入；host + --token 亦可。\n"+
			"  --predict（实验）用本地预测回显掩盖往返延迟：你敲的字立刻显示、加下划线表示未确认，\n"+
			"  服务器确认后转正；快链路自动不预测，全屏 TUI 内不预测。\n"+
			"  退出：tmux 前缀键 + d，或 Ctrl-]。")
	return 0
}
