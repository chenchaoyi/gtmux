package connect

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Run implements `gtmux attach` — attach to a remote gtmux pane in the local terminal.
//
//	gtmux attach <host> --token <tok> [%pane]     # owner (full)
//	gtmux attach https://host/#t=<token> [%pane]  # guest (scope-restricted)
//	  --read-only   watch only, never send input
func Run(args []string) int {
	var target, token, pane string
	readOnly := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			return usage()
		case a == "--read-only" || a == "-r":
			readOnly = true
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

	if err := RunAttach(tgt.URL, tgt.Token, pane, readOnly); err != nil {
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
	i18n.Sae("gtmux attach: name a pane —", "gtmux attach: 指定一个 pane —")
	for _, a := range panes {
		name := a.Task
		if name == "" {
			name = a.Session
		}
		fmt.Fprintf(os.Stderr, "  %s  %s\n", a.PaneID, name)
	}
	return "", 2
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
		"usage: gtmux attach <host|pair-link|share-link> [%pane] [--token <tok>] [--read-only]\n"+
			"  Attach to a remote gtmux pane in your local terminal (raw, interactive).\n"+
			"  A pair link (…/#c=<code>, from `gtmux pair`) enrolls THIS terminal as one of\n"+
			"  your own devices (full control, token persisted — later just `gtmux attach <host>`).\n"+
			"  A share link (…/#t=<token>) connects as a scope-restricted guest; a host +\n"+
			"  --token also works. Detach with tmux `prefix d` or Ctrl-].",
		"用法：gtmux attach <host|配对链接|分享链接> [%pane] [--token <tok>] [--read-only]\n"+
			"  在本地终端里附着到远程 gtmux 的 pane（原生、可交互）。\n"+
			"  配对链接（…/#c=<code>，来自 `gtmux pair`）把本终端登记为你自己的设备\n"+
			"  （全权,token 会保存 —— 之后直接 `gtmux attach <host>`）。\n"+
			"  分享链接（…/#t=<token>）以受限访客接入；host + --token 亦可。\n"+
			"  退出：tmux 前缀键 + d，或 Ctrl-]。")
	return 0
}
