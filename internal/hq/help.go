package hq

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"golang.org/x/term"
)

// HelpFlag is shared with the root command's machine-readable help.
type HelpFlag struct {
	Name, EN, ZH     string
	Values, Requires []string
	section          int
}

// HelpFlags returns a fresh catalog; advanced options remain discoverable in JSON.
func HelpFlags() []HelpFlag {
	return []HelpFlag{
		{Name: "--agent CMD", EN: "Choose the HQ agent, e.g. codex", ZH: "指定 HQ 使用的 agent，如 codex", section: 0},
		{Name: "--here", EN: "Start in the current pane", ZH: "在当前窗格启动", section: 0},
		{Name: "--pane %N", EN: "Start in an empty shell pane", ZH: "在指定的空闲 shell 窗格启动", section: 0},
		{Name: "--new-pane", EN: "Start in a new split pane", ZH: "拆分新窗格并启动", section: 0},
		{Name: "--board", EN: "Read the situation board", ZH: "查看态势板", section: 1},
		{Name: "--records", EN: "Check archive size and backup status", ZH: "查看档案大小与备份状态", section: 1},
		{Name: "--home", EN: "Show the HQ directory", ZH: "显示 HQ 目录", section: 1},
		{Name: "--export PATH", EN: "Export an encrypted backup", ZH: "导出加密备份", section: 2},
		{Name: "--import PATH", EN: "Restore a full backup; stop HQ first", ZH: "恢复完整备份，需先退出 HQ", section: 2},
		{Name: "migrate --help", EN: "Select content to move to another Mac", ZH: "查看换机迁移方法，按需选择内容", section: 2},
		{Name: "--rotate", EN: "Request rotation after the current turn", ZH: "申请在当前回合结束后轮换会话", section: 3},
		{Name: "--lang en|zh", EN: "Set the HQ playbook language (space syntax)", ZH: "设置 HQ 守则语言（参数使用空格分隔）", Values: []string{"en", "zh"}, section: 4},
		{Name: "--json", EN: "JSON output with --board or --records", ZH: "与 --board 或 --records 一起输出 JSON", section: 4},
		{Name: "--memory", EN: "Alias for --records", ZH: "--records 的旧名称", section: 4},
		{Name: "--plain", EN: "Export without encryption", ZH: "导出不加密的备份", section: 4},
		{Name: "--passphrase-stdin", EN: "Read the passphrase from stdin's first line", ZH: "从标准输入首行读取备份口令", section: 4},
		{Name: "--expect-archive DIGEST", EN: "Restore only if the archive digest matches", ZH: "仅在档案摘要匹配时恢复", Requires: []string{"--import"}, section: 4},
		{Name: "--maintenance-done distill|self-check", EN: "HQ records a completed maintenance pass", ZH: "供 HQ 记录已完成的维护", Values: []string{"distill", "self-check"}, section: 5},
		{Name: "--help-all", EN: "Show advanced options and environment variables", ZH: "查看完整参数与环境变量", section: 6},
	}
}

func helpWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		w, _ = strconv.Atoi(os.Getenv("COLUMNS"))
		if w <= 0 {
			w = 80
		}
	}
	return w
}

// HelpText groups options by user task and wraps by display columns, including CJK.
func HelpText(width int, all bool) string {
	if width < 40 {
		width = 40
	}
	if width > 80 {
		width = 80
	}
	zh := i18n.Lang() == "zh"
	choose := func(en, cn string) string {
		if zh {
			return cn
		}
		return en
	}
	var b strings.Builder
	paragraph := func(s string, indent int) {
		for _, line := range i18n.WrapDisp(s, width-indent) {
			b.WriteString(strings.Repeat(" ", indent) + line + "\n")
		}
	}
	b.WriteString(choose("gtmux hq [options]\n", "gtmux hq [参数]\n"))
	paragraph(choose("HQ follows your agents, reports progress and acts within your authorization.", "HQ 跟进 agent、汇报进展，并在你的授权范围内处理事务。"), 0)
	paragraph(choose("Run gtmux hq to open HQ, or start it if needed.", "运行 gtmux hq 打开 HQ；尚未启动时会自动创建。"), 0)
	sections := [][2]string{{"Start HQ", "启动位置与 agent"}, {"View information", "查看信息"}, {"Backup and migration", "备份与换机"}, {"Rotate the conversation", "会话轮换"}, {"Advanced options", "高级参数"}, {"HQ maintenance", "HQ 内部维护"}}
	flags := HelpFlags()
	for section, title := range sections {
		if section >= 4 && !all {
			break
		}
		b.WriteString("\n" + choose(title[0], title[1]) + "\n")
		labelWidth := 0
		for _, f := range flags {
			if f.section == section && i18n.DispWidth(f.Name) > labelWidth {
				labelWidth = i18n.DispWidth(f.Name)
			}
		}
		for _, f := range flags {
			if f.section != section {
				continue
			}
			if width < 60 || width-labelWidth-4 < 24 {
				paragraph(f.Name, 2)
				paragraph(choose(f.EN, f.ZH), 4)
			} else {
				for n, line := range i18n.WrapDisp(choose(f.EN, f.ZH), width-labelWidth-4) {
					if n == 0 {
						fmt.Fprintf(&b, "  %s  %s\n", i18n.PadRight(f.Name, labelWidth), line)
					} else {
						paragraph(line, labelWidth+4)
					}
				}
			}
		}
	}
	if all {
		b.WriteString("\n" + choose("Notes", "使用说明") + "\n")
		for _, note := range [][2]string{
			{"On first launch, choose an installed agent; HQ remembers your choice. GTMUX_HQ_AGENT overrides it.", "首次启动选择已安装的 agent，后续沿用；GTMUX_HQ_AGENT 可覆盖该选择。"},
			{"--here, --pane and --new-pane are mutually exclusive and require HQ to be stopped. Target panes must contain an idle, empty shell.", "--here、--pane、--new-pane 互斥，且需先退出 HQ。目标窗格必须是无草稿的空闲 shell。"},
			{"Before rotation, update the board and knowledge base and record the handoff. Success requires a verified new session ID.", "轮换前先更新态势板、知识库并记录交接；验证新会话 ID 后才算轮换成功。"},
			{"Full restore includes the board and retains the current archive as a backup. Migration selects long-term content and excludes the old board.", "完整恢复包含态势板，当前档案保留为备份。换机迁移按需选择长期内容，不迁移旧态势板。"},
			{"Passphrase: --passphrase-stdin, then GTMUX_HQ_PASSPHRASE, then a terminal prompt.", "备份口令依次从 --passphrase-stdin、GTMUX_HQ_PASSPHRASE、终端输入获取。"},
			{"Personal settings belong in LOCAL.md in the HQ directory; knowledge writes must run there too. GTMUX_HQ_BRIEF=off disables the startup briefing.", "个性化设置写入 HQ 目录的 LOCAL.md；知识库写操作也需在该目录执行。GTMUX_HQ_BRIEF=off 可关闭启动简报。"},
		} {
			lines := i18n.WrapDisp(choose(note[0], note[1]), width-4)
			for n, line := range lines {
				if n == 0 {
					b.WriteString("  • " + line + "\n")
				} else {
					b.WriteString("    " + line + "\n")
				}
			}
		}
	} else {
		b.WriteString("\n")
		paragraph(choose("More options: gtmux hq --help-all", "完整参数与使用说明：gtmux hq --help-all"), 0)
	}
	return b.String()
}
