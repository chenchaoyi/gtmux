package app

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// renderOverview returns the full sessions/windows/panes summary as text.
func renderOverview() string {
	totalS := len(tmux.Lines("list-sessions"))
	totalW := len(tmux.Lines("list-windows", "-a"))
	totalP := len(tmux.Lines("list-panes", "-a"))

	current := ""
	if tmux.InTmux() {
		current = tmux.Display("", "#{session_name}")
	}

	var b strings.Builder
	title := i18n.Tr("tmux overview", "tmux 概览")
	fmt.Fprintf(&b, "%s%s%s — %s · %s · %s\n\n", i18n.Bold, title, i18n.Reset,
		i18n.Pl(totalS, "session"), i18n.Pl(totalW, "window"), i18n.Pl(totalP, "pane"))

	// Per-window pane-count format: pluralize in en, plain in zh.
	pfmt := "(#{?#{==:#{window_panes},1},1 pane,#{window_panes} panes})"
	if i18n.Lang() == "zh" {
		pfmt = "(#{window_panes} pane)"
	}
	winFmt := fmt.Sprintf("    #{window_index}: #{window_name}#{?window_active, *,}"+
		"#{?window_zoomed_flag, Z,}#{?window_activity_flag, •,}  %s%s%s", i18n.Dim, pfmt, i18n.Reset)

	for _, line := range tmux.Lines("list-sessions", "-F", "#{session_name}|#{session_attached}|#{session_windows}") {
		f := strings.SplitN(line, "|", 3)
		if len(f) < 3 {
			continue
		}
		name, attached, nwin := f[0], f[1], f[2]
		panes := len(tmux.Lines("list-panes", "-s", "-t", name))
		nw, _ := strconv.Atoi(nwin)

		var mark string
		switch {
		case current != "" && name == current:
			mark = i18n.Cyan + "▶" + i18n.Reset
		case attached != "0" && attached != "":
			mark = i18n.Green + "●" + i18n.Reset
		default:
			mark = i18n.Yellow + "○" + i18n.Reset
		}
		fmt.Fprintf(&b, "%s %s%s%s %s · %s\n", mark, i18n.Bold, i18n.PadRight(name, 20), i18n.Reset,
			i18n.Pl(nw, "window"), i18n.Pl(panes, "pane"))
		for _, wl := range tmux.Lines("list-windows", "-t", name, "-F", winFmt) {
			b.WriteString(wl + "\n")
		}
		b.WriteString("\n")
	}

	if i18n.Lang() == "zh" {
		fmt.Fprintf(&b, "%s▶ 当前  ● 已连接  ○ 待接回%s\n", i18n.Dim, i18n.Reset)
		fmt.Fprintf(&b, "%s* 活跃  Z 放大  • 新输出   (跳转: gtmux focus <名字>)%s\n", i18n.Dim, i18n.Reset)
	} else {
		fmt.Fprintf(&b, "%s▶ current  ● attached  ○ detached%s\n", i18n.Dim, i18n.Reset)
		fmt.Fprintf(&b, "%s* active  Z zoomed  • new output   (jump: gtmux focus <name>)%s\n", i18n.Dim, i18n.Reset)
	}
	return b.String()
}

// cmdOverview implements `gtmux overview [--popup|--hold]`.
func cmdOverview(args []string) int {
	if !tmux.ServerUp() {
		i18n.Say("No tmux server running", "没有运行中的 tmux server")
		return 1
	}
	mode := ""
	if len(args) > 0 {
		mode = args[0]
	}
	switch mode {
	case "--popup":
		if !tmux.InTmux() { // not inside tmux: just print
			fmt.Print(renderOverview())
			return 0
		}
		content := strings.Count(renderOverview(), "\n") + 3 // + popup border & hold hint
		maxh := 40
		if h, err := strconv.Atoi(tmux.Display("", "#{client_height}")); err == nil && h > 0 {
			maxh = h * 9 / 10
		}
		self := selfPath()
		var inner string
		if content <= maxh {
			inner = fmt.Sprintf("%s --lang=%s overview --hold", self, i18n.Lang())
			return tmux.RunInteractive("display-popup", "-E", "-w", "64", "-h", strconv.Itoa(content), inner)
		}
		inner = fmt.Sprintf("%s --lang=%s overview | less -R", self, i18n.Lang())
		return tmux.RunInteractive("display-popup", "-E", "-w", "64", "-h", strconv.Itoa(maxh), inner)
	case "--hold":
		fmt.Print(renderOverview())
		fmt.Print(i18n.Dim + i18n.Tr("any key to close", "按任意键关闭") + i18n.Reset)
		readKey()
	default:
		fmt.Print(renderOverview())
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
