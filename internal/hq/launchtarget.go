package hq

import (
	"errors"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/panefocus"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// launchTarget is where `gtmux hq` was told to put the supervisor. Without one, the
// command keeps its old behaviour: focus a live HQ, revive a stamped-but-dead one in
// place, else spawn its own tmux session.
//
// The old behaviour is what the commander hit on 2026-09-16: a window that had hosted
// HQ once (%21) kept its stamp and its start path after the agent was gone and the
// shell had cd'd elsewhere, so every `gtmux hq` from any other window revived HQ THERE
// — and nothing let him say "no, here". These three flags say it.
type launchTarget struct {
	pane    string // --pane %N: an existing pane, which must be a bare shell
	here    bool   // --here: the pane the command runs in
	newPane bool   // --new-pane: split the current window and use the new pane
}

func (t launchTarget) any() bool { return t.pane != "" || t.here || t.newPane }

// resolveLaunchTarget turns the flags into one instruction: kind "pane" with the pane
// to type into, or "split" with the pane whose window is split. `own` is $TMUX_PANE.
func resolveLaunchTarget(t launchTarget, own string) (kind, pane string, err error) {
	n := 0
	if t.pane != "" {
		n++
	}
	if t.here {
		n++
	}
	if t.newPane {
		n++
	}
	switch {
	case n == 0:
		return "", "", nil
	case n > 1:
		return "", "", errors.New(i18n.Tr("pick one of --pane, --here, --new-pane", "--pane、--here、--new-pane 只能选一个"))
	case t.pane != "":
		if !strings.HasPrefix(t.pane, "%") {
			return "", "", errors.New(i18n.Tr("--pane takes a tmux pane id like %21", "--pane 要的是 tmux 的 pane id，形如 %21"))
		}
		return "pane", t.pane, nil
	case t.here:
		if own == "" {
			return "", "", errors.New(i18n.Tr("--here only works from inside a tmux pane", "--here 只能在 tmux 的 pane 里用"))
		}
		return "pane", own, nil
	default:
		if own == "" {
			return "", "", errors.New(i18n.Tr("--new-pane only works from inside a tmux pane (it splits the window you are in)", "--new-pane 只能在 tmux 的 pane 里用（它拆的是你所在的窗口）"))
		}
		return "split", own, nil
	}
}

// shq single-quotes s for a POSIX shell.
func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// launchHQAt starts the supervisor where the user said, and moves the identity there.
//
// One supervisor, ever: a live HQ anywhere refuses the request rather than starting a
// second. The target must be a bare shell — typing an agent command over a running
// program is not something gtmux does. The old stamped window, if any, is unstamped
// FIRST, so the next resolve (and every wake) finds the new pane and only it.
func launchHQAt(kind, target, agentCmd string) int {
	if live := hqpane.Find(); live != "" && hqAgentAlive(live) {
		where := hqWhere(live)
		i18n.Sae("gtmux hq: HQ is already running at "+where+" — there is only ever one. Quit it there first, or run plain `gtmux hq` to go to it.",
			"gtmux hq: HQ 已经在跑了（"+where+"）—— HQ 只有一个。先在那边退出它，或者直接跑 `gtmux hq` 切过去。")
		return 1
	}
	pane := target
	if kind == "split" {
		id, err := tmux.Run("split-window", "-P", "-F", "#{pane_id}", "-t", target, "-c", state.HQHome())
		id = strings.TrimSpace(id)
		if err != nil || id == "" {
			i18n.Sae("gtmux hq: could not split the window ("+target+")", "gtmux hq: 拆分窗口失败（"+target+"）")
			return 1
		}
		pane = id
	} else {
		if tmux.Display(pane, "#{pane_id}") != pane {
			i18n.Sae("gtmux hq: no tmux pane "+pane, "gtmux hq: 没有 "+pane+" 这个 pane")
			return 1
		}
		if hqAgentAlive(pane) {
			i18n.Sae("gtmux hq: something is already running in "+hqWhere(pane)+" — pick an empty shell",
				"gtmux hq: "+hqWhere(pane)+" 里已经有程序在跑 —— 换一个空着的 shell")
			return 1
		}
		// The one question every pane writer asks first (composer-writers-need-one-guard):
		// a paste appends, so a half-typed line in that shell would be submitted joined
		// to the agent command. Withhold instead; the user clears the line and re-runs.
		if !dispatch.BoxEmpty(dispatchbridge.DispatchIO(pane)) {
			i18n.Sae("gtmux hq: "+hqWhere(pane)+" has unsent text on its line — clear it, then run this again",
				"gtmux hq: "+hqWhere(pane)+" 的命令行上有没提交的内容 —— 清掉再跑一次")
			return 1
		}
	}
	rawCmd := resolveHQLaunchAgent(agentCmd)
	cleared := hqpane.ClearStamps()
	hqpane.Stamp(pane)
	cmd := agentenv.Wrap(rawCmd)
	if kind != "split" {
		// The pane may sit anywhere; the supervisor must run in its home.
		cmd = "cd " + shq(state.HQHome()) + " && " + cmd
	}
	_ = tmux.SendText(pane, cmd, true)
	where := hqWhere(pane)
	i18n.Say("HQ starting at "+where+".", "HQ 正在 "+where+" 启动。")
	for _, old := range cleared {
		if old != pane {
			i18n.Say("  "+old+" no longer counts as HQ's window.", "  "+old+" 不再算 HQ 的窗口。")
		}
	}
	_ = panefocus.FocusPaneByID(pane)
	noteAtPane(pane, i18n.Tr("gtmux: HQ starts here", "gtmux：HQ 从这里启动"))
	deliverHQBriefing(pane, rawCmd)
	return 0
}

func ownPane() string { return os.Getenv("TMUX_PANE") }
