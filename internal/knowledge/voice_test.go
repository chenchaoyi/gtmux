package knowledge

import (
	"strings"
	"testing"
)

// The check's whole design is the STRENGTH RANKING, so that is what these pin: what fires
// alone, what needs company, and what it must not read at all.
func TestVoiceStrongTellsFireOnOneSighting(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"a house coinage names its plain word", "这条船在等你", "会话"},
		{"a decorative arrow", "读到告警 ⇒ 先查事件流", "⇒"},
		{"chat residue", "希望这对你有帮助", "chat residue"},
	} {
		got := voiceCheck(tc.text)
		if got == "" {
			t.Errorf("%s: read as fine, want a finding", tc.name)
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: detail %q does not name %q", tc.name, got, tc.want)
		}
	}
}

func TestVoiceWeakTellsNeedCompany(t *testing.T) {
	// Each of these has an honest use, so one alone proves nothing.
	dashes := "第一段 —— 说明\n第二段 —— 补充"
	if got := voiceCheck(dashes); got != "" {
		t.Errorf("dashes alone = %q, want quiet", got)
	}
	bold := "**结论**在前面,**证据**在后面"
	if got := voiceCheck(bold); got != "" {
		t.Errorf("bold alone = %q, want quiet", got)
	}
	notX := "这不是配置问题,是权限问题"
	if got := voiceCheck(notX); got != "" {
		t.Errorf("a contrast alone = %q, want quiet — whether it is earned is a reading, not a match", got)
	}
	// Two kinds together are the shape this check is for.
	if got := voiceCheck(dashes + "\n" + bold); got == "" {
		t.Error("dashes and bold together read as fine, want a finding")
	}
	if got := voiceCheck(notX + "\n" + dashes); got == "" {
		t.Error("a contrast with dashes read as fine, want a finding")
	}
}

func TestVoiceReadsProseOnly(t *testing.T) {
	// A command, a quoted warning, a table and an indented block are none of its business.
	// Every one of these carried the tells that made the first draft of this check fire on
	// a fifth of a real base.
	for _, tc := range []struct{ name, text string }{
		{"a fenced block", "看这个:\n```sh\ngtmux send -- --force\ngtmux reap -- --snooze\n```"},
		{"inline code", "用 `--force` 和 `--body-file` 两个参数"},
		{"a quoted warning", "它自述:「实际没落地 —— 我搜过了 —— 一个字都没有」"},
		{"a table", "| 名字 | 说明 |\n| a —— b | c —— d |"},
		{"an indented block", "    第一行 —— 说明\n    第二行 —— 补充"},
	} {
		if got := voiceCheck(tc.text); got != "" {
			t.Errorf("%s was read: %q", tc.name, got)
		}
	}
}

func TestVoiceCountsADoubleDashOnce(t *testing.T) {
	// `——` is one dash a writer typed, not two. Counting it twice made a single quoted
	// aside look like a habit.
	if got := voiceCheck("一句话 —— 补充说明"); got != "" {
		t.Errorf("one double dash = %q, want quiet", got)
	}
}

func TestVoiceLeavesAPlainEntryAlone(t *testing.T) {
	plain := "hook 通道检查把长期没有事件判成通道故障。\n\n" +
		"2026-09-14 实测 14 条闲置会话全被点名,同期真在动的三条 hook 一条没漏。\n" +
		"判据:先按 pane 统计一遍事件流,再决定信不信。"
	if got := voiceCheck(plain); got != "" {
		t.Errorf("a plain entry was flagged: %q", got)
	}
}
