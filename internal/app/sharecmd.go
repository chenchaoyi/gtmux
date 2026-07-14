package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// cmdShare implements `gtmux share` — the host's control over SHARED web input: a
// collaborator on the shared web page may type into the terminal ONLY with consent
// AND only into allowlisted panes. It talks to the LOCAL running serve (which owns the
// policy in memory) over its master-token API, like `gtmux devices`.
//
//	gtmux share                       show consent, allowlists, and guest links
//	gtmux share on | off              turn shared INPUT on/off (typing consent)
//	gtmux share add <pane…>           allow a pane (e.g. %3) for guest INPUT (implies view)
//	gtmux share remove <pane…>        disallow a pane for input
//	gtmux share view add <pane…>      let guests SEE a pane
//	gtmux share view remove <pane…>   stop guests seeing a pane (also removes its input)
//	gtmux share view clear            guests see nothing again
//	gtmux share new [--label <name>]  mint a guest share link (URL + QR)
//	gtmux share revoke <id>           kill one guest link
func cmdShare(args []string) int {
	port := defaultServePort
	var rest []string
	label := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			return shareUsage()
		case a == "--json":
			jsonOut = true
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
		return shareStatus(base, token, jsonOut)
	case "on":
		return shareSetEnabled(base, token, true)
	case "off":
		return shareSetEnabled(base, token, false)
	case "add":
		return shareEditPanes(base, token, rest, true)
	case "remove", "rm":
		return shareEditPanes(base, token, rest, false)
	case "view":
		return shareView(base, token, rest)
	case "new":
		return shareNew(base, token, label, port, jsonOut)
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
	Enabled   bool     `json:"enabled"`
	Panes     []string `json:"panes"`
	ViewPanes []string `json:"view_panes"`
}

// shareStatusOut is the `gtmux share status --json` contract: the consent state,
// both allowlists (input `panes` + `view_panes`), the guest links, and the base URL a
// link is built on — carrying NO token (a consumer never needs to read the token
// roster).
type shareStatusOut struct {
	Enabled   bool         `json:"enabled"`
	Panes     []string     `json:"panes"`
	ViewPanes []string     `json:"view_panes"`
	Guests    []shareGuest `json:"guests"`
	Base      string       `json:"base,omitempty"`
}

type shareGuest struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	EnrolledAt int64  `json:"enrolled_at"`
}

// buildShareStatus is the pure mapper (state + roster + base → the --json shape),
// unit-tested without a live serve.
func buildShareStatus(st shareStateJSON, guests []deviceListEntry, base string) shareStatusOut {
	out := shareStatusOut{Enabled: st.Enabled, Panes: st.Panes, ViewPanes: st.ViewPanes, Base: base, Guests: []shareGuest{}}
	if out.Panes == nil {
		out.Panes = []string{}
	}
	if out.ViewPanes == nil {
		out.ViewPanes = []string{}
	}
	for _, g := range guests {
		out.Guests = append(out.Guests, shareGuest{ID: g.ID, Label: g.Name, EnrolledAt: g.EnrolledAt})
	}
	return out
}

func shareStatus(base, token string, jsonOut bool) int {
	st, ok := getShareState(base, token)
	if !ok {
		return 1
	}
	if jsonOut {
		guests := listGuests(base, token)
		shareBase := readTunnelURL()
		if shareBase == "" {
			shareBase = base
		}
		b, _ := json.MarshalIndent(buildShareStatus(st, guests, shareBase), "", "  ")
		fmt.Println(string(b))
		return 0
	}
	if st.Enabled {
		i18n.Say("shared web input: ON", "分享输入：已开启")
	} else {
		i18n.Say("shared web input: OFF (guests can't type)", "分享输入：已关闭（访客不能输入）")
	}
	if len(st.ViewPanes) == 0 {
		i18n.Say("  viewable panes: (none — guests see nothing)", "  可见 pane：（无 —— 访客什么都看不到）")
	} else {
		fmt.Printf("  %s %s\n", i18n.Tr("viewable panes:", "可见 pane："), strings.Join(st.ViewPanes, " "))
	}
	if len(st.Panes) == 0 {
		i18n.Say("  typable panes:  (none)", "  可输入 pane：（无）")
	} else {
		fmt.Printf("  %s %s\n", i18n.Tr("typable panes: ", "可输入 pane："), strings.Join(st.Panes, " "))
	}
	// Guest links = roster entries with scope "guest".
	if guests := listGuests(base, token); len(guests) > 0 {
		i18n.Say(fmt.Sprintf("%d guest link(s):", len(guests)), fmt.Sprintf("%d 个分享链接：", len(guests)))
		for _, g := range guests {
			fmt.Printf("  %s  %-20s  %s\n", g.ID, g.Name, fmtAgo(g.EnrolledAt))
		}
		i18n.Say("Revoke one: gtmux share revoke <id>", "吊销某个：gtmux share revoke <id>")
	}
	i18n.Say("Let a guest SEE a pane: gtmux share view add %N   ·   let them TYPE: gtmux share on + gtmux share add %N   ·   new link: gtmux share new",
		"让访客看某 pane：gtmux share view add %N   ·   让其输入：gtmux share on + gtmux share add %N   ·   新链接：gtmux share new")
	return 0
}

// shareView handles `gtmux share view <add|remove|clear> [pane…]` — the VIEW
// allowlist (which panes a guest may SEE), independent of the input consent toggle.
func shareView(base, token string, rest []string) int {
	if len(rest) == 0 {
		i18n.Sae("gtmux share view: need add|remove|clear", "gtmux share view: 需要 add|remove|clear")
		return 2
	}
	op, panes := rest[0], rest[1:]
	switch op {
	case "add":
		return shareEditViewPanes(base, token, panes, true)
	case "remove", "rm":
		return shareEditViewPanes(base, token, panes, false)
	case "clear":
		return shareClearView(base, token)
	default:
		i18n.Sae("gtmux share view: unknown '"+op+"' (want add|remove|clear)",
			"gtmux share view: 未知 '"+op+"'（应为 add|remove|clear）")
		return 2
	}
}

// shareEditViewPanes adds/removes panes on the VIEW allowlist. Removing a pane from
// view also removes it from the input allowlist (input ⊆ view): a guest can never
// type into a pane it cannot see.
func shareEditViewPanes(base, token string, panes []string, add bool) int {
	if len(panes) == 0 {
		i18n.Sae("gtmux share view: name at least one pane (e.g. %3)", "gtmux share view: 至少给一个 pane（如 %3）")
		return 2
	}
	st, ok := getShareState(base, token)
	if !ok {
		return 1
	}
	view := strSet(st.ViewPanes)
	input := strSet(st.Panes)
	for _, p := range panes {
		if add {
			view[p] = true
		} else {
			delete(view, p)
			delete(input, p) // removing view removes input
		}
	}
	body := map[string]any{"view_panes": sortedSlice(view)}
	if !add {
		body["panes"] = sortedSlice(input)
	}
	payload, _ := json.Marshal(body)
	if _, ok := postShareConfig(base, token, payload); !ok {
		return 1
	}
	return shareStatus(base, token, false)
}

// shareClearView empties the VIEW allowlist (and thus the input allowlist too) — a
// guest goes back to seeing nothing.
func shareClearView(base, token string) int {
	body, _ := json.Marshal(map[string]any{"view_panes": []string{}, "panes": []string{}})
	if _, ok := postShareConfig(base, token, body); !ok {
		return 1
	}
	return shareStatus(base, token, false)
}

func strSet(items []string) map[string]bool {
	s := map[string]bool{}
	for _, p := range items {
		if p != "" {
			s[p] = true
		}
	}
	return s
}

func sortedSlice(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func shareSetEnabled(base, token string, on bool) int {
	body, _ := json.Marshal(map[string]any{"enabled": on})
	if _, ok := postShareConfig(base, token, body); !ok {
		return 1
	}
	return shareStatus(base, token, false)
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
	return shareStatus(base, token, false)
}

// shareNewOut is the `gtmux share new --json` contract: the minted link's id,
// label, and full URL. NO bare token — the URL carries the `#t=` token, so the
// secret lives in exactly one field a consumer already treats as sensitive.
type shareNewOut struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// buildShareNew is the pure link assembler (base + minted token → the --json
// shape), unit-tested without a live serve.
func buildShareNew(id, label, token, base string) shareNewOut {
	return shareNewOut{ID: id, Label: label, URL: base + "/#t=" + token}
}

func shareNew(base, token, label string, port int, jsonOut bool) int {
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
	if jsonOut {
		b, _ := json.MarshalIndent(buildShareNew(out.ID, out.Name, out.Token, shareBase), "", "  ")
		fmt.Println(string(b))
		return 0
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
		"usage: gtmux share [on|off | add <pane…> | remove <pane…> |\n"+
			"                    view <add|remove|clear> [pane…] | new [--label <name>] | revoke <id>] [--json]\n"+
			"  Scope what a collaborator on the shared web page can do, per pane:\n"+
			"    · SEE a pane:   gtmux share view add %N   (default: guests see NOTHING)\n"+
			"    · TYPE a pane:  gtmux share on  +  gtmux share add %N   (implies view; consent OFF by default)\n"+
			"  Removing a pane's view removes its input too (input ⊆ view).\n"+
			"  --json makes `status` and `new` emit machine-readable output (no token).",
		"用法：gtmux share [on|off | add <pane…> | remove <pane…> |\n"+
			"                  view <add|remove|clear> [pane…] | new [--label <名>] | revoke <id>] [--json]\n"+
			"  按 pane 控制协作者在分享 web 页面能做什么：\n"+
			"    · 让其看某 pane：  gtmux share view add %N  （默认访客什么都看不到）\n"+
			"    · 让其输入某 pane：gtmux share on  +  gtmux share add %N （自动含可见；默认不同意输入）\n"+
			"  取消某 pane 的可见会连带取消其输入（input ⊆ view）。\n"+
			"  --json 让 `status` / `new` 输出机器可读格式（不含 token）。")
	return 0
}
