// `gtmux awake` — keep this Mac working with the lid closed so `serve`,
// the tunnel and the phone keep answering (openspec change server-mode).
//
// This phase ships SENSING only: `status` reports the truth about the machine and
// about who owns that state. `on`/`off` need the privileged half (one admin
// authorization plus a de-escalation-only guard) and say so rather than pretending.
//
// The reporting here deliberately shows BOTH sleep readings — the live kernel one
// and the persisted one — because they answer different questions ("will it sleep
// now" vs "will it still be disabled after a reboot"), and conflating them is what
// made an earlier draft report a Mac as healthy while it could not sleep.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/servermode"
)

// cmdServerMode implements `gtmux awake [status|on|off] [--json]`.
//
// `server-mode` was the name it shipped under in v0.44.0 and still works — it is the
// only day-to-day command that ever carried a hyphen, which is why it was shortened.
func cmdServerMode(invokedAs string, args []string) int {
	if invokedAs == "server-mode" {
		i18n.Say("note: `gtmux server-mode` is now `gtmux awake` (the old name still works).",
			"提示：`gtmux server-mode` 已改名为 `gtmux awake`（旧名仍可用）。")
	}
	jsonOut, yes := false, false
	sub := "status"
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--yes", "-y":
			yes = true
		case "-h", "--help":
			return serverModeUsage()
		case "status", "on", "off":
			sub = a
		default:
			i18n.Sae("gtmux awake: unknown option '"+a+"'",
				"gtmux awake: 未知选项 '"+a+"'")
			return serverModeUsage()
		}
	}
	switch sub {
	case "on":
		return serverModeOn(yes)
	case "off":
		return serverModeOff()
	}
	return serverModeStatus(jsonOut)
}

func serverModeStatus(jsonOut bool) int {
	st := servermode.Current()
	if jsonOut {
		b, _ := json.MarshalIndent(st, "", "  ")
		fmt.Println(string(b))
		return 0
	}

	// Headline: state · tier · how long · power.
	// The headline names the COMMAND, not the feature: someone who typed `gtmux awake`
	// should read `awake = …` back. The longer "server mode" framing lives in help and
	// docs, where there is room to explain it.
	head := i18n.Tr("awake", "保持唤醒") + " = " + serverModeStateLabel(st.State)
	if st.Tier != "" {
		head += " (" + st.Tier + ")"
	}
	if st.Since > 0 {
		head += "  ·  " + i18n.Tr("up ", "已运行 ") + fmtDurShort(time.Now().Unix()-st.Since)
	}
	head += "  ·  " + i18n.Tr("power ", "电源 ") + st.Power
	if st.BatteryPct > 0 {
		head += fmt.Sprintf(" %d%%", st.BatteryPct)
	}
	fmt.Println(head)

	switch st.State {
	case servermode.StateOn:
		i18n.Say("  The lid may close — this Mac will not sleep.",
			"  合上盖子也不会睡 —— 这台 Mac 现在不休眠。")
		if !st.OwnedByGtmux {
			i18n.Say("  ⚠ gtmux did not set this. Reporting only — it will not be changed for you.",
				"  ⚠ 这不是 gtmux 开的。只报告、不会替你改动。")
			i18n.Say("    To undo it yourself: sudo pmset -a disablesleep 0",
				"    想自己关掉：sudo pmset -a disablesleep 0")
		}
	case servermode.StateLapsed:
		i18n.Say("  ⚠ gtmux's record says server mode is on, but the kernel says sleep is enabled.",
			"  ⚠ gtmux 的记录说服务器模式开着，但内核说睡眠是允许的。")
		i18n.Say("    Something turned it off underneath us — treat the closed-lid session as over.",
			"    有别的东西在底下把它关掉了 —— 请认为合盖会话已经结束。")
	default:
		i18n.Say("  This Mac sleeps normally (closing the lid sleeps it).",
			"  这台 Mac 会正常睡眠（合盖即睡）。")
	}

	// The two readings disagree only transiently (the plist lags a write), so say
	// which one is authoritative rather than leaving a reader to guess.
	if st.PersistedDisableSleep != st.SystemDisableSleep {
		i18n.Say(fmt.Sprintf("  note: live=%v, persisted=%v — the live reading wins; the stored one lags a write.",
			st.SystemDisableSleep, st.PersistedDisableSleep),
			fmt.Sprintf("  注意：活状态=%v、落盘=%v —— 以活状态为准，落盘值会滞后。",
				st.SystemDisableSleep, st.PersistedDisableSleep))
	} else if st.PersistedDisableSleep {
		i18n.Say("  This survives a reboot — it is stored in the system's power settings.",
			"  这项设置会跨重启保留 —— 它写在系统电源设置里。")
	}

	fmt.Println("  " + i18n.Tr("guard: ", "守护：") + serverModeGuardLabel(st.Guard))
	if line := serverModePlatformLine(st.Platform); line != "" {
		fmt.Println("  " + line)
	}
	if st.LastExit != nil {
		fmt.Printf("  %s%s (%s)\n", i18n.Tr("last exit: ", "上次结束："),
			st.LastExit.Reason, time.Unix(st.LastExit.At, 0).Format("2006-01-02 15:04"))
	}
	return 0
}

func serverModeStateLabel(state string) string {
	switch state {
	case servermode.StateOn:
		return i18n.Tr("on", "开启")
	case servermode.StateLapsed:
		return i18n.Tr("lapsed", "已失效")
	default:
		return i18n.Tr("off", "关闭")
	}
}

// serverModeGuardLabel describes the de-escalation-only guard. Absent is not a
// neutral fact while server mode is on: nothing would give sleep back if gtmux died.
func serverModeGuardLabel(g servermode.Guard) string {
	switch {
	case g.Installed && g.Healthy:
		return i18n.Tr("installed · healthy", "已安装 · 健康")
	case g.Installed:
		return i18n.Tr("INCOMPLETE — it cannot restore sleep in this state",
			"不完整 —— 这种状态下它无法恢复睡眠")
	default:
		return i18n.Tr("not installed", "未安装")
	}
}

// serverModePlatformLine explains the preflight verdict. It says nothing on a
// verified machine (no noise), refuses loudly when the mechanism is absent, and on
// an unverified OS says exactly that — unverified, not unsupported — because the
// project has measured one configuration and a hard version allowlist would break
// this on every future macOS.
func serverModePlatformLine(p servermode.Support) string {
	switch p.Reason {
	case servermode.ReasonNotMacOS:
		return i18n.Tr("✕ not supported: server mode is macOS-only.",
			"✕ 不支持：服务器模式仅适用于 macOS。")
	case servermode.ReasonNoSetting:
		return i18n.Tr("✕ not supported: this macOS no longer accepts the sleep setting gtmux relies on ("+p.OSVersion+").",
			"✕ 不支持：这个 macOS 已不接受 gtmux 依赖的睡眠设置（"+p.OSVersion+"）。")
	case servermode.ReasonNoReadback:
		return i18n.Tr("✕ not supported: the kernel power state is unreadable here — gtmux will not manage a state it cannot see.",
			"✕ 不支持：这里读不到内核电源状态 —— gtmux 不会去管理一个它看不见的状态。")
	case servermode.ReasonUnverifiedOS:
		return i18n.Tr("⚠ unverified on macOS "+p.OSVersion+" (tested on 26.x). It relies on an undocumented setting — verify once: enable, close the lid 2 min, check it kept serving.",
			"⚠ macOS "+p.OSVersion+" 未经验证（我们只测过 26.x）。它依赖一项未公开文档的设置 —— 请验证一次：开启后合盖 2 分钟，再看是否一直在服务。")
	}
	return ""
}

func serverModeUsage() int {
	i18n.Say("usage: gtmux awake [status|on|off] [--json] [--yes]",
		"用法：gtmux awake [status|on|off] [--json] [--yes]")
	i18n.Say("  Keep this Mac working with the lid closed, so serve/tunnel/the phone keep answering.",
		"  让这台 Mac 合盖也继续工作，serve/隧道/手机端保持可用。")
	i18n.Say("  status  the real state: sleep setting, who owns it, power, guard health, platform.",
		"  status  真实状态：睡眠设置、归属、电源、守护健康度、平台支持情况。")
	i18n.Say("  on      asks for your administrator password once, then verifies it took effect.",
		"  on      需要输入一次管理员密码，之后会确认确实生效。")
	i18n.Say("  off     restores sleep. Works even if you dismiss the prompt — the guard finishes it.",
		"  off     恢复睡眠。即使你取消密码框也没关系 —— 守护会把它做完。")
	i18n.Say("  It stays on until you turn it off. On battery it runs down to 20%, warning at 30%.",
		"  开启后一直生效，直到你关闭。用电池时会跑到 20% 才恢复睡眠，30% 时提醒。")
	return 0
}

// serverModeOn enables the clamshell tier. The order here is deliberate: every
// refusal that can be decided WITHOUT privilege happens before the password prompt,
// so a user is never asked to authorize something that was going to be refused.
func serverModeOn(yes bool) int {
	st := servermode.Current()
	if st.SystemDisableSleep {
		if st.OwnedByGtmux {
			i18n.Say("awake is already on — the lid may close.", "已经开着了 —— 合盖不会睡。")
			return 0
		}
		i18n.Sae("sleep is already disabled on this Mac, but not by gtmux — refusing to take it over.",
			"这台 Mac 的睡眠已经被禁用，但不是 gtmux 干的 —— 不接管它。")
		i18n.Say("  Undo it yourself first if you want gtmux to manage it: sudo pmset -a disablesleep 0",
			"  想让 gtmux 管理它，请先自己关掉：sudo pmset -a disablesleep 0")
		return 1
	}
	if line := serverModePlatformLine(st.Platform); line != "" && !st.Platform.OK {
		fmt.Println(line)
		return 1
	}

	// What the user is about to accept, in the terms that actually matter to them.
	i18n.Say("This keeps the Mac running with the lid closed (server mode).",
		"这会让 Mac 合上盖子也继续运行（服务器模式）。")
	i18n.Say("  · It stays on until you turn it off — it does not expire.",
		"  · 开启后会一直生效，直到你自己关闭 —— 不会自动到期。")
	fmt.Printf("  · %s\n", i18n.Tr(
		fmt.Sprintf("On battery it keeps going and restores sleep at %d%% (you are warned at %d%%).",
			20, servermode.EnableThresholdPct),
		fmt.Sprintf("用电池时也继续跑，掉到 %d%% 才恢复睡眠（%d%% 时会先提醒你）。",
			20, servermode.EnableThresholdPct)))
	i18n.Say("  · A closed lid dissipates heat worse — hard surface, not in a bag. A fanless Air suffers most.",
		"  · 合盖散热更差 —— 放硬质平面、别塞进包里；无风扇的 Air 影响最大。")
	i18n.Say("  · This Mac stays remotely reachable, unattended, for as long as it is on.",
		"  · 开启期间这台 Mac 会长时间无人值守地保持可远程访问。")
	if !serveIsRunning() {
		i18n.Say("  ⚠ Neither `gtmux serve` nor a tunnel is running — server mode would keep this Mac awake for nothing.",
			"  ⚠ `gtmux serve` 和隧道都没在跑 —— 现在开启只会让 Mac 白白醒着。")
	}
	if !st.Platform.Verified {
		fmt.Println("  " + serverModePlatformLine(st.Platform))
	}
	if !yes && !confirm(i18n.Tr("Turn it on?", "开启？")) {
		return 1
	}

	switch err := servermode.Enable(); {
	case err == nil:
		i18n.Say("awake is on — verified against the kernel, not just requested.",
			"已开启 —— 已向内核确认生效，不只是发了个命令。")
		if !st.Platform.Verified {
			i18n.Say("  Please verify once: close the lid for 2 minutes, then check it kept serving.",
				"  请验证一次：合盖 2 分钟，再看它是否一直在服务。")
		}
		return 0
	case errors.Is(err, servermode.ErrNoAuth):
		i18n.Sae("authorization declined — nothing was changed.", "未获授权 —— 什么都没有改动。")
		i18n.Say("  Enabling needs an administrator password typed at this machine (it cannot be done remotely).",
			"  开启需要在这台机器上输入管理员密码（无法远程完成）。")
		return 1
	case errors.Is(err, servermode.ErrLowBattery):
		i18n.Sae("refused: "+err.Error(), "已拒绝："+err.Error())
		return 1
	case errors.Is(err, servermode.ErrNotVerified):
		i18n.Sae("the setting was applied but the kernel did not take it — server mode is NOT on.",
			"设置发出去了，但内核没有采纳 —— 服务器模式并未开启。")
		i18n.Say("  This is the honest answer for a macOS where the mechanism does not work. Anything applied was undone.",
			"  这就是「这个 macOS 上机制不生效」的诚实答案。已做的改动都撤回了。")
		return 1
	case errors.Is(err, servermode.ErrGuardMissing):
		i18n.Sae("the safety guard could not be installed, so the change was undone.",
			"恢复睡眠的守护装不上，改动已撤回。")
		i18n.Say("  gtmux will not leave a Mac unable to sleep with nothing able to restore it.",
			"  gtmux 不会让一台 Mac 处于「不能睡、又没人能恢复」的状态。")
		return 1
	default:
		i18n.Sae("gtmux awake on: "+err.Error(), "gtmux awake on: "+err.Error())
		return 1
	}
}

// serverModeOff turns it off. The stand-down marker goes down first and needs no
// privilege, so even a declined password prompt cannot leave the Mac awake.
func serverModeOff() int {
	if !servermode.Current().SystemDisableSleep && !servermode.GuardInstalled() {
		i18n.Say("awake is already off — this Mac sleeps normally.", "已经是关闭的 —— 这台 Mac 正常睡眠。")
		return 0
	}
	if err := servermode.Disable(); err != nil {
		i18n.Sae("gtmux awake off: "+err.Error(), "gtmux awake off: "+err.Error())
		return 1
	}
	if servermode.SleepDisabled() {
		i18n.Say("stand-down requested — the guard will restore sleep shortly.",
			"已请求关闭 —— 守护会在很短时间内恢复睡眠。")
		return 0
	}
	i18n.Say("awake is off — this Mac sleeps normally again.",
		"已关闭 —— 这台 Mac 恢复正常睡眠。")
	return 0
}

// serveIsRunning reports whether anything is actually being served. Enabling server
// mode with nothing listening keeps a Mac awake for no reason.
func serveIsRunning() bool {
	out, _ := exec.Command("pgrep", "-f", "gtmux serve").Output()
	return len(strings.TrimSpace(string(out))) > 0
}
