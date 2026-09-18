package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Three renderings of the one table in helpdata.go: the screen, one command, and
// the same thing as data.

const helpNameW = 29 // the command column; the widest entry is `restore [--pick|--plan]`

// helpWidth is how wide help may draw: the terminal, capped at 80 so a wide window
// does not turn a paragraph into one long line, and floored so a narrow one still
// gets whole words.
func helpWidth() int {
	w := termWidth()
	if w > 80 {
		w = 80
	}
	if w < 40 {
		w = 40
	}
	return w
}

// usage prints the screen you get from `gtmux`, `gtmux --help` and any command
// gtmux does not recognise.
func usage() { fmt.Print(usageText()) }

func usageText() string {
	zh := i18n.Lang() == "zh"
	width := helpWidth()
	var b strings.Builder

	fmt.Fprintf(&b, "gtmux %s · %s\n\n", Version, tagline())

	if zh {
		fmt.Fprintf(&b, "  %s这一屏\n", i18n.PadRight("gtmux", helpNameW+2))
		fmt.Fprintf(&b, "  %s跑一个命令\n", i18n.PadRight("gtmux <命令> [参数]", helpNameW+2))
		fmt.Fprintf(&b, "  %s只看这一个命令，含全部参数\n", i18n.PadRight("gtmux <命令> --help", helpNameW+2))
		fmt.Fprintf(&b, "  %s同样的内容，输出成 JSON\n", i18n.PadRight("gtmux --help --json", helpNameW+2))
	} else {
		fmt.Fprintf(&b, "  %sthis screen\n", i18n.PadRight("gtmux", helpNameW+2))
		fmt.Fprintf(&b, "  %srun one command\n", i18n.PadRight("gtmux <command> [options]", helpNameW+2))
		fmt.Fprintf(&b, "  %sthat command alone, with every flag\n", i18n.PadRight("gtmux <command> --help", helpNameW+2))
		fmt.Fprintf(&b, "  %sthe same, as JSON\n", i18n.PadRight("gtmux --help --json", helpNameW+2))
	}

	for _, g := range helpGroups {
		head, mode := g.EN, g.ModeEN
		if zh {
			head, mode = g.ZH, g.ModeZH
		}
		fmt.Fprintf(&b, "\n%s %s\n", head, mode)
		if g.Compact {
			var names []string
			for _, c := range helpCommands {
				if c.Group == g.ID && !c.Internal {
					names = append(names, c.Name)
				}
			}
			for _, l := range i18n.WrapDisp(strings.Join(names, " · "), width-2) {
				b.WriteString("  " + l + "\n")
			}
			continue
		}
		for _, c := range helpCommands {
			if c.Group != g.ID || c.Internal {
				continue
			}
			line := c.Name
			if c.Args != "" {
				line += " " + c.Args
			}
			sum := c.EN
			if zh {
				sum = c.ZH
			}
			fmt.Fprintf(&b, "  %s%s\n", i18n.PadRight(line, helpNameW), sum)
		}
	}

	if zh {
		b.WriteString("\n第一次用：gtmux doctor 体检这台 Mac，gtmux doctor --fix 把缺的配上。\n")
		b.WriteString("语言跟随系统，也可以用 --lang=en|zh 或 GTMUX_LANG 指定。\n")
	} else {
		b.WriteString("\nNew here: gtmux doctor checks this Mac, gtmux doctor --fix sets up the rest.\n")
		b.WriteString("Language follows the system; --lang=en|zh or GTMUX_LANG overrides it.\n")
	}

	return b.String()
}

// commandHelp prints one command: what it does, how it is typed, and every flag
// with what that flag accepts. A command with no table entry falls back to the
// screen, which is what the old code did for all of them.
func commandHelp(name string) { fmt.Print(commandHelpText(name)) }

func commandHelpText(name string) string {
	c := findCommand(name)
	if c == nil {
		return usageText()
	}
	zh := i18n.Lang() == "zh"
	var b strings.Builder

	line := "gtmux " + c.Name
	if c.Args != "" {
		line += " " + c.Args
	}
	sum := c.EN
	if zh {
		sum = c.ZH
	}
	fmt.Fprintf(&b, "%s\n  %s\n", line, sum)

	// Say what running it does before the flags: an agent needs that first, and a
	// person reading it loses nothing. It is the group's own mode line, so the two
	// screens can never disagree about what a command touches.
	fmt.Fprintf(&b, "  %s\n", modeOf(c, zh))

	width := helpWidth()

	if len(c.Flags) > 0 {
		b.WriteString("\n")
		w := 0
		for _, f := range c.Flags {
			if n := i18n.DispWidth(f.Name); n > w {
				w = n
			}
		}
		if w > 28 {
			w = 28
		}
		gut := strings.Repeat(" ", w+4)
		for _, f := range c.Flags {
			d := f.EN
			if zh {
				d = f.ZH
			}
			for i, l := range i18n.WrapDisp(d, width-w-4) {
				if i == 0 {
					fmt.Fprintf(&b, "  %s  %s\n", i18n.PadRight(f.Name, w), l)
					continue
				}
				b.WriteString(gut + l + "\n")
			}
			for _, note := range flagNotes(f, zh) {
				for _, l := range i18n.WrapDisp(note, width-w-4) {
					b.WriteString(gut + l + "\n")
				}
			}
		}
	}

	detail := c.DetailEN
	if zh {
		detail = c.DetailZH
	}
	if detail != "" {
		b.WriteString("\n")
		for _, l := range i18n.WrapDisp(detail, width) {
			b.WriteString(l + "\n")
		}
	}

	if c.OwnHelp && len(c.Flags) == 0 {
		if zh {
			fmt.Fprintf(&b, "\n参数在它自己那儿：gtmux %s --help\n", c.Name)
		} else {
			fmt.Fprintf(&b, "\nIts flags are its own: gtmux %s --help\n", c.Name)
		}
	}

	return b.String()
}

// modeOf says what running this command does to the machine, in the group's own
// words. A group that prints as a name list has no single mode, so those fall back
// to the plain reads/writes split.
func modeOf(c *command, zh bool) string {
	for _, g := range helpGroups {
		if g.ID != c.Group || g.Compact {
			continue
		}
		if zh {
			return strings.TrimPrefix(g.ModeZH, "· ")
		}
		return strings.TrimPrefix(g.ModeEN, "· ")
	}
	if c.Writes {
		return i18n.Tr("changes something", "会改东西")
	}
	return i18n.Tr("reads only", "只读")
}

// flagNotes spells out the three things an error used to be the only source of:
// what a flag accepts, what it refuses above, and what must come with it.
func flagNotes(f cmdFlag, zh bool) []string {
	var out []string
	if len(f.Values) > 0 {
		if zh {
			out = append(out, "取值："+strings.Join(f.Values, " | "))
		} else {
			out = append(out, "takes: "+strings.Join(f.Values, " | "))
		}
	}
	if f.MaxBytes > 0 {
		if zh {
			out = append(out, fmt.Sprintf("上限 %d 字节，超了会被拒", f.MaxBytes))
		} else {
			out = append(out, fmt.Sprintf("refused above %d bytes", f.MaxBytes))
		}
	}
	if len(f.Requires) > 0 {
		if zh {
			out = append(out, "必须同时带上 "+strings.Join(f.Requires, " "))
		} else {
			out = append(out, "must come with "+strings.Join(f.Requires, " "))
		}
	}
	if f.Required {
		if zh {
			out = append(out, "必填")
		} else {
			out = append(out, "required")
		}
	}
	return out
}

// The `--help --json` shapes. Field names are what a reader would guess, and both
// language halves ship: an agent reading this is not always in the reader's locale.
type helpJSON struct {
	Version string        `json:"version"`
	Tagline string        `json:"tagline"`
	Groups  []groupJSON   `json:"groups"`
	Cmds    []commandJSON `json:"commands"`
}

type groupJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	TitleZH   string `json:"title_zh"`
	ReadsOnly bool   `json:"reads_only"`
}

type commandJSON struct {
	Name      string     `json:"name"`
	Group     string     `json:"group"`
	Usage     string     `json:"usage"`
	Summary   string     `json:"summary"`
	SummaryZH string     `json:"summary_zh"`
	Writes    bool       `json:"writes"`
	Internal  bool       `json:"internal,omitempty"`
	OwnHelp   bool       `json:"own_help,omitempty"`
	Flags     []flagJSON `json:"flags,omitempty"`
	Detail    string     `json:"detail,omitempty"`
	DetailZH  string     `json:"detail_zh,omitempty"`
}

type flagJSON struct {
	Name     string   `json:"name"`
	Summary  string   `json:"summary"`
	Values   []string `json:"values,omitempty"`
	MaxBytes int      `json:"max_bytes,omitempty"`
	Requires []string `json:"requires,omitempty"`
	Required bool     `json:"required,omitempty"`
}

func usageJSON() int {
	out := helpJSON{Version: Version, Tagline: tagline()}
	for _, g := range helpGroups {
		out.Groups = append(out.Groups, groupJSON{ID: g.ID, Title: g.EN, TitleZH: g.ZH, ReadsOnly: g.ReadsOnly})
	}
	for _, c := range helpCommands {
		u := "gtmux " + c.Name
		if c.Args != "" {
			u += " " + c.Args
		}
		cj := commandJSON{
			Name: c.Name, Group: c.Group, Usage: u,
			Summary: c.EN, SummaryZH: c.ZH,
			Writes: c.Writes, Internal: c.Internal, OwnHelp: c.OwnHelp,
			Detail: c.DetailEN, DetailZH: c.DetailZH,
		}
		for _, f := range c.Flags {
			cj.Flags = append(cj.Flags, flagJSON{
				Name: f.Name, Summary: f.EN,
				Values: f.Values, MaxBytes: f.MaxBytes,
				Requires: f.Requires, Required: f.Required,
			})
		}
		out.Cmds = append(out.Cmds, cj)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		i18n.Sae("gtmux: could not write the help JSON", "gtmux: 写不出 help 的 JSON")
		return 1
	}
	return 0
}
