// `gtmux pair` — the PAIR track of the pair-share model: enroll YOUR OWN surfaces
// (phone / browser / another computer's terminal) as full-control devices. One
// short-lived code, three media: a QR for the phone app, a URL for a browser, and
// a one-line `gtmux attach` command for a terminal — any ONE of which redeems once
// into the same revocable roster. Collaborators never come through here — that's
// `gtmux share` (least-privilege guest links).
package app

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// cmdPair implements `gtmux pair [list|revoke <id>]`. `gtmux devices` stays as an
// alias of the roster surface (list/revoke) for muscle memory.
func cmdPair(args []string) int {
	port := defaultServePort
	var rest []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			return pairUsage()
		case a == "--port" || a == "-p":
			if i+1 < len(args) {
				port = atoiOr(args[i+1], port)
				i++
			}
		case strings.HasPrefix(a, "--port="):
			port = atoiOr(strings.TrimPrefix(a, "--port="), port)
		default:
			rest = append(rest, a)
		}
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	token := resolveServeToken("")
	sub := "new"
	if len(rest) > 0 {
		sub = rest[0]
		rest = rest[1:]
	}
	switch sub {
	case "new":
		return pairNew(port, token)
	case "list", "ls":
		return pairList(base, token)
	case "revoke":
		if len(rest) == 0 {
			i18n.Sae("gtmux pair revoke: missing <id>", "gtmux pair revoke: 缺少 <id>")
			return 2
		}
		return revokeDevice(base, token, rest[0])
	default:
		i18n.Sae("gtmux pair: unknown subcommand '"+sub+"'", "gtmux pair: 未知子命令 '"+sub+"'")
		return pairUsage()
	}
}

// pairBase resolves the address a pairing link should carry: the public tunnel URL
// when one is up (reachable from anywhere), else the first LAN address.
func pairBase(port int) (base string, lanOnly bool) {
	if url := readTunnelURL(); url != "" {
		return strings.TrimRight(url, "/"), false
	}
	hosts := reachableHosts("")
	if len(hosts) == 0 {
		return "", true
	}
	return "http://" + net.JoinHostPort(hosts[0], strconv.Itoa(port)), true
}

// pairMedia renders the three pairing media for one code (pure — unit-tested).
func pairMedia(base, code string) (browserURL, attachCmd string) {
	link := base + "/#c=" + code
	return link, "gtmux attach '" + link + "'"
}

// pairNew mints ONE short-lived code and prints it three ways. Any single medium
// redeems the code once; run `gtmux pair` again for the next device.
func pairNew(port int, token string) int {
	code := mintEnrollCode(port, token)
	if code == "" {
		i18n.Sae("gtmux pair: can't reach the local serve — start it with `gtmux serve` (or `gtmux tunnel`).",
			"gtmux pair: 连不上本地 serve —— 先用 `gtmux serve`（或 `gtmux tunnel`）启动。")
		return 1
	}
	base, lanOnly := pairBase(port)
	if base == "" {
		i18n.Sae("gtmux pair: no reachable address (no LAN interface, no tunnel)",
			"gtmux pair: 没有可达地址（无局域网接口,也无隧道）")
		return 1
	}
	browserURL, attachCmd := pairMedia(base, code)

	fmt.Println()
	i18n.Say(i18n.Bold+"Pair one of YOUR OWN devices (full control) — one code, three doors:"+i18n.Reset,
		i18n.Bold+"配对你自己的设备（全权）—— 一个码,三种用法："+i18n.Reset)
	i18n.Say("  the code is one-time and expires in 5 minutes; use exactly ONE of:",
		"  配对码一次性、5 分钟内有效;三选一使用：")
	fmt.Println()
	i18n.Say("  1) Phone — scan in the gtmux app (Pair → Scan):",
		"  1) 手机 —— 在 gtmux App 里扫码（配对 → 扫一扫）：")
	printBrandQR(os.Stdout, string(pairingPayload(base, "", code, "")))
	i18n.Say("  2) Browser — open:", "  2) 浏览器 —— 打开：")
	fmt.Printf("       %s\n", browserURL)
	i18n.Say("  3) Another computer's terminal — run:", "  3) 另一台电脑的终端 —— 运行：")
	fmt.Printf("       %s\n", attachCmd)
	fmt.Println()
	if lanOnly {
		i18n.Say("  (LAN address — for pairing from outside your network, run `gtmux tunnel` first)",
			"  （局域网地址 —— 想在外网配对,先跑 `gtmux tunnel`）")
	}
	i18n.Say("Manage paired devices: gtmux pair list · revoke: gtmux pair revoke <id>. Collaborators go through `gtmux share` instead.",
		"管理已配对设备：gtmux pair list · 吊销：gtmux pair revoke <id>。给协作者用 `gtmux share`。")
	return 0
}

// pairList shows the OWNER devices (guests live under `gtmux share status`).
func pairList(base, token string) int {
	devices, ok := fetchDevices(base, token)
	if !ok {
		return 1
	}
	var owners []deviceListEntry
	for _, d := range devices {
		if d.Scope != "guest" {
			owners = append(owners, d)
		}
	}
	if len(owners) == 0 {
		i18n.Say("No paired devices yet. Pair one: gtmux pair", "还没有配对设备。配对：gtmux pair")
		return 0
	}
	i18n.Say(fmt.Sprintf("%d paired device(s) — your own surfaces, full control:", len(owners)),
		fmt.Sprintf("%d 台已配对设备 —— 你自己的设备,全权：", len(owners)))
	for _, d := range owners {
		last := i18n.Tr("never seen", "从未连接")
		if d.LastSeen > 0 {
			last = fmtAgo(d.LastSeen)
		}
		fmt.Printf("  %s  %-24s  %s\n", d.ID, deviceDisplayName(d.Name), last)
	}
	i18n.Say("Revoke one: gtmux pair revoke <id>   ·   guests: gtmux share status",
		"吊销某台：gtmux pair revoke <id>   ·   分享链接看：gtmux share status")
	return 0
}

func pairUsage() int {
	i18n.Sae(
		"usage: gtmux pair [list | revoke <id>] [--port N]\n"+
			"  PAIR = your own devices, full control (the owner track of pair/share).\n"+
			"  Bare `gtmux pair` mints a one-time code and prints it three ways:\n"+
			"    · phone: a QR for the gtmux app\n"+
			"    · browser: an https://…/#c=<code> link\n"+
			"    · terminal: a one-line `gtmux attach` command (token persists — later\n"+
			"      just `gtmux attach <host>`)\n"+
			"  Collaborators get scoped access via `gtmux share`, never through pair.",
		"用法：gtmux pair [list | revoke <id>] [--port N]\n"+
			"  PAIR = 你自己的设备,全权(pair/share 双轨中的本人轨)。\n"+
			"  裸 `gtmux pair` 生成一次性配对码,三种用法一次给全：\n"+
			"    · 手机：gtmux App 扫码\n"+
			"    · 浏览器：打开 https://…/#c=<code> 链接\n"+
			"    · 终端：一行 `gtmux attach` 命令(token 会保存 —— 之后直接\n"+
			"      `gtmux attach <host>`)\n"+
			"  协作者走 `gtmux share` 的受限链接,不走 pair。")
	return 0
}
