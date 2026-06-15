package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// renderOverview returns the full sessions/windows/panes summary as text.
func renderOverview() string {
	totalS := len(tmuxLines("list-sessions"))
	totalW := len(tmuxLines("list-windows", "-a"))
	totalP := len(tmuxLines("list-panes", "-a"))

	current := ""
	if inTmux() {
		current = display("", "#{session_name}")
	}

	var b strings.Builder
	title := tr("tmux overview", "tmux 概览")
	fmt.Fprintf(&b, "%s%s%s — %s · %s · %s\n\n", cBold, title, cReset,
		pl(totalS, "session"), pl(totalW, "window"), pl(totalP, "pane"))

	// Per-window pane-count format: pluralize in en, plain in zh.
	pfmt := "(#{?#{==:#{window_panes},1},1 pane,#{window_panes} panes})"
	if lang == "zh" {
		pfmt = "(#{window_panes} pane)"
	}
	winFmt := fmt.Sprintf("    #{window_index}: #{window_name}#{?window_active, *,}"+
		"#{?window_zoomed_flag, Z,}#{?window_activity_flag, •,}  %s%s%s", cDim, pfmt, cReset)

	for _, line := range tmuxLines("list-sessions", "-F", "#{session_name}|#{session_attached}|#{session_windows}") {
		f := strings.SplitN(line, "|", 3)
		if len(f) < 3 {
			continue
		}
		name, attached, nwin := f[0], f[1], f[2]
		panes := len(tmuxLines("list-panes", "-s", "-t", name))
		nw, _ := strconv.Atoi(nwin)

		var mark string
		switch {
		case current != "" && name == current:
			mark = cCyan + "▶" + cReset
		case attached != "0" && attached != "":
			mark = cGreen + "●" + cReset
		default:
			mark = cYellow + "○" + cReset
		}
		fmt.Fprintf(&b, "%s %s%s%s %s · %s\n", mark, cBold, padRight(name, 20), cReset,
			pl(nw, "window"), pl(panes, "pane"))
		for _, wl := range tmuxLines("list-windows", "-t", name, "-F", winFmt) {
			b.WriteString(wl + "\n")
		}
		b.WriteString("\n")
	}

	if lang == "zh" {
		fmt.Fprintf(&b, "%s▶ 当前  ● 已连接  ○ 待接回%s\n", cDim, cReset)
		fmt.Fprintf(&b, "%s* 活跃  Z 放大  • 新输出   (跳转: gtmux focus <名字>)%s\n", cDim, cReset)
	} else {
		fmt.Fprintf(&b, "%s▶ current  ● attached  ○ detached%s\n", cDim, cReset)
		fmt.Fprintf(&b, "%s* active  Z zoomed  • new output   (jump: gtmux focus <name>)%s\n", cDim, cReset)
	}
	return b.String()
}

// cmdOverview implements `gtmux overview [--popup|--hold]`.
func cmdOverview(args []string) int {
	if !tmuxServerUp() {
		say("No tmux server running", "没有运行中的 tmux server")
		return 1
	}
	mode := ""
	if len(args) > 0 {
		mode = args[0]
	}
	switch mode {
	case "--popup":
		if !inTmux() { // not inside tmux: just print
			fmt.Print(renderOverview())
			return 0
		}
		content := strings.Count(renderOverview(), "\n") + 3 // + popup border & hold hint
		maxh := 40
		if h, err := strconv.Atoi(display("", "#{client_height}")); err == nil && h > 0 {
			maxh = h * 9 / 10
		}
		self := selfPath()
		var inner string
		if content <= maxh {
			inner = fmt.Sprintf("%s --lang=%s overview --hold", self, lang)
			return runTmux("display-popup", "-E", "-w", "64", "-h", strconv.Itoa(content), inner)
		}
		inner = fmt.Sprintf("%s --lang=%s overview | less -R", self, lang)
		return runTmux("display-popup", "-E", "-w", "64", "-h", strconv.Itoa(maxh), inner)
	case "--hold":
		fmt.Print(renderOverview())
		fmt.Print(cDim + tr("any key to close", "按任意键关闭") + cReset)
		readKey()
	default:
		fmt.Print(renderOverview())
	}
	return 0
}

// runTmux runs a tmux subcommand inheriting stdio and returns its exit code.
func runTmux(args ...string) int {
	c := exec.Command(tmuxBin, args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// readKey waits for a single keypress (raw, no echo) via /dev/tty.
func readKey() {
	exec.Command("stty", "-f", "/dev/tty", "cbreak", "-echo").Run()
	defer exec.Command("stty", "-f", "/dev/tty", "sane").Run()
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return
	}
	defer tty.Close()
	var b [1]byte
	tty.Read(b[:])
}
