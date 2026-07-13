package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// cmdShare implements `gtmux share` — the host's control over SHARED web input: a
// collaborator on the shared web page may type into the terminal ONLY with consent
// AND only into allowlisted panes. It talks to the LOCAL running serve (which owns the
// policy in memory) over its master-token API, like `gtmux devices`.
//
//	gtmux share                       show consent, allowlist, and guest links
//	gtmux share on | off              turn shared input on/off (consent)
//	gtmux share add <pane…>           allow a pane (e.g. %3) for guests
//	gtmux share remove <pane…>        disallow a pane
//	gtmux share new [--label <name>]  mint a guest share link (URL + QR)
//	gtmux share revoke <id>           kill one guest link
func cmdShare(args []string) int {
	port := defaultServePort
	var rest []string
	label := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			return shareUsage()
		case a == "--port" || a == "-p":
			if i+1 < len(args) {
				port = atoiOr(args[i+1], port)
				i++
			}
		case strings.HasPrefix(a, "--port="):
			port = atoiOr(strings.TrimPrefix(a, "--port="), port)
		case a == "--label":
			if i+1 < len(args) {
				label = args[i+1]
				i++
			}
		default:
			rest = append(rest, a)
		}
	}

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	token := resolveServeToken("")
	sub := "status"
	if len(rest) > 0 {
		sub = rest[0]
		rest = rest[1:]
	}
	switch sub {
	case "status":
		return shareStatus(base, token)
	case "on":
		return shareSetEnabled(base, token, true)
	case "off":
		return shareSetEnabled(base, token, false)
	case "add":
		return shareEditPanes(base, token, rest, true)
	case "remove", "rm":
		return shareEditPanes(base, token, rest, false)
	case "new":
		return shareNew(base, token, label, port)
	case "revoke":
		if len(rest) == 0 {
			i18n.Sae("gtmux share revoke: missing <id>", "gtmux share revoke: 缺少 <id>")
			return 2
		}
		return revokeDevice(base, token, rest[0]) // guests live in the same roster
	default:
		i18n.Sae("gtmux share: unknown subcommand '"+sub+"'", "gtmux share: 未知子命令 '"+sub+"'")
		return shareUsage()
	}
}

type shareStateJSON struct {
	Enabled bool     `json:"enabled"`
	Panes   []string `json:"panes"`
}

func shareStatus(base, token string) int {
	st, ok := getShareState(base, token)
	if !ok {
		return 1
	}
	if st.Enabled {
		i18n.Say("shared web input: ON", "分享输入：已开启")
	} else {
		i18n.Say("shared web input: OFF (guests are view-only)", "分享输入：已关闭（访客只读）")
	}
	if len(st.Panes) == 0 {
		i18n.Say("  allowed panes: (none)", "  允许输入的 pane：（无）")
	} else {
		fmt.Printf("  %s %s\n", i18n.Tr("allowed panes:", "允许输入的 pane："), strings.Join(st.Panes, " "))
	}
	// Guest links = roster entries with scope "guest".
	if guests := listGuests(base, token); len(guests) > 0 {
		i18n.Say(fmt.Sprintf("%d guest link(s):", len(guests)), fmt.Sprintf("%d 个分享链接：", len(guests)))
		for _, g := range guests {
			fmt.Printf("  %s  %-20s  %s\n", g.ID, g.Name, fmtAgo(g.EnrolledAt))
		}
		i18n.Say("Revoke one: gtmux share revoke <id>", "吊销某个：gtmux share revoke <id>")
	}
	i18n.Say("Turn on: gtmux share on   ·   allow a pane: gtmux share add %N   ·   new link: gtmux share new",
		"开启：gtmux share on   ·   允许某 pane：gtmux share add %N   ·   新链接：gtmux share new")
	return 0
}

func shareSetEnabled(base, token string, on bool) int {
	body, _ := json.Marshal(map[string]any{"enabled": on})
	if _, ok := postShareConfig(base, token, body); !ok {
		return 1
	}
	return shareStatus(base, token)
}

func shareEditPanes(base, token string, panes []string, add bool) int {
	if len(panes) == 0 {
		i18n.Sae("gtmux share: name at least one pane (e.g. %3)", "gtmux share: 至少给一个 pane（如 %3）")
		return 2
	}
	st, ok := getShareState(base, token)
	if !ok {
		return 1
	}
	set := map[string]bool{}
	for _, p := range st.Panes {
		set[p] = true
	}
	for _, p := range panes {
		if add {
			set[p] = true
		} else {
			delete(set, p)
		}
	}
	next := make([]string, 0, len(set))
	for p := range set {
		next = append(next, p)
	}
	body, _ := json.Marshal(map[string]any{"panes": next})
	if _, ok := postShareConfig(base, token, body); !ok {
		return 1
	}
	return shareStatus(base, token)
}

func shareNew(base, token, label string, port int) int {
	body, _ := json.Marshal(map[string]string{"label": label})
	req, _ := http.NewRequest(http.MethodPost, base+"/api/share/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return shareUnreachable()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux share new: serve returned %d", resp.StatusCode),
			fmt.Sprintf("gtmux share new: 服务返回 %d", resp.StatusCode))
		return 1
	}
	var out struct{ Token, ID, Name string }
	_ = json.NewDecoder(resp.Body).Decode(&out)

	shareBase := readTunnelURL()
	local := shareBase == ""
	if local {
		shareBase = base
	}
	link := shareBase + "/#t=" + out.Token
	fmt.Println()
	i18n.Say("New guest share link ("+out.ID+"):", "新的分享链接（"+out.ID+"）：")
	fmt.Printf("  %s\n", link)
	if local {
		i18n.Say("  (this is a LOCAL address — run `gtmux tunnel` for a link others can open)",
			"  （这是本机地址 —— 想让别人能打开,先跑 `gtmux tunnel`）")
	}
	i18n.Say("Share it with a collaborator. They can type ONLY into your allowlisted panes, and only while shared input is ON. Revoke: gtmux share revoke "+out.ID,
		"发给协作者。他们只能输入你白名单里的 pane,且仅在分享输入开启时。吊销：gtmux share revoke "+out.ID)
	printBrandQR(os.Stdout, link)
	return 0
}

// --- helpers ---

func getShareState(base, token string) (shareStateJSON, bool) {
	req, _ := http.NewRequest(http.MethodGet, base+"/api/share/config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		shareUnreachable()
		return shareStateJSON{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux share: serve returned %d", resp.StatusCode),
			fmt.Sprintf("gtmux share: 服务返回 %d", resp.StatusCode))
		return shareStateJSON{}, false
	}
	var st shareStateJSON
	_ = json.NewDecoder(resp.Body).Decode(&st)
	return st, true
}

func postShareConfig(base, token string, body []byte) (shareStateJSON, bool) {
	req, _ := http.NewRequest(http.MethodPost, base+"/api/share/config", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		shareUnreachable()
		return shareStateJSON{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux share: serve returned %d", resp.StatusCode),
			fmt.Sprintf("gtmux share: 服务返回 %d", resp.StatusCode))
		return shareStateJSON{}, false
	}
	var st shareStateJSON
	_ = json.NewDecoder(resp.Body).Decode(&st)
	return st, true
}

// listGuests returns roster entries with scope "guest" (the share links).
func listGuests(base, token string) []deviceListEntry {
	req, _ := http.NewRequest(http.MethodGet, base+"/api/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Devices []deviceListEntry `json:"devices"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	var guests []deviceListEntry
	for _, d := range out.Devices {
		if d.Scope == "guest" {
			guests = append(guests, d)
		}
	}
	return guests
}

func readTunnelURL() string {
	if b, err := os.ReadFile(tunnelURLPath()); err == nil {
		return strings.TrimSpace(string(b))
	}
	return ""
}

func shareUnreachable() int {
	i18n.Sae("gtmux share: can't reach the local serve — start it with `gtmux serve` (or `gtmux tunnel`).",
		"gtmux share: 连不上本地 serve —— 先用 `gtmux serve`（或 `gtmux tunnel`）启动。")
	return 1
}

func atoiOr(s string, def int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 || n > 65535 {
		return def
	}
	return n
}

func shareUsage() int {
	i18n.Sae(
		"usage: gtmux share [on|off | add <pane…> | remove <pane…> | new [--label <name>] | revoke <id>]\n"+
			"  Let a collaborator on the shared web page type into the terminal — ONLY with your\n"+
			"  consent (share on) and ONLY into panes you allow (share add %N). Default: off, none.",
		"用法：gtmux share [on|off | add <pane…> | remove <pane…> | new [--label <名>] | revoke <id>]\n"+
			"  让协作者在分享的 web 页面里往终端输入 —— 仅在你同意（share on）且仅限你允许的\n"+
			"  pane（share add %N）。默认：关闭、无 pane。")
	return 0
}
