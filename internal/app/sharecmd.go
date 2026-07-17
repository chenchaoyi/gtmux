package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
//	gtmux share link <id>             re-show an existing link's URL (+ QR)
//	gtmux share revoke <id>           kill one guest link
func cmdShare(args []string) int {
	port := defaultServePort
	var rest []string
	label := ""
	jsonOut := false
	var viewFlag, typeFlag *[]string // per-link scope (pair-share-model); nil = not given
	expires := ""                    // "" = not given; "never" clears
	takeList := func(v string) *[]string {
		parts := []string{}
		for _, p := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
			if p != "" {
				parts = append(parts, p)
			}
		}
		return &parts
	}
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
		case a == "--view":
			if i+1 < len(args) {
				viewFlag = takeList(args[i+1])
				i++
			}
		case strings.HasPrefix(a, "--view="):
			viewFlag = takeList(strings.TrimPrefix(a, "--view="))
		case a == "--type":
			if i+1 < len(args) {
				typeFlag = takeList(args[i+1])
				i++
			}
		case strings.HasPrefix(a, "--type="):
			typeFlag = takeList(strings.TrimPrefix(a, "--type="))
		case a == "--expires":
			if i+1 < len(args) {
				expires = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--expires="):
			expires = strings.TrimPrefix(a, "--expires=")
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
		return shareNew(base, token, label, port, jsonOut, viewFlag, typeFlag, expires)
	case "link":
		if len(rest) == 0 {
			i18n.Sae("gtmux share link: missing <id>", "gtmux share link: 缺少 <id>")
			return 2
		}
		return shareLink(base, token, rest[0], jsonOut)
	case "set":
		if len(rest) == 0 {
			i18n.Sae("gtmux share set: missing <id>", "gtmux share set: 缺少 <id>")
			return 2
		}
		return shareSet(base, token, rest[0], viewFlag, typeFlag, expires)
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

// parseExpires turns "24h" / "45m" / "7d" / "never" into (seconds, clear, ok).
// "" is not-given (0,false,true); "never" clears an expiry.
func parseExpires(s string) (secs int64, clear, ok bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "":
		return 0, false, true
	case "never", "0":
		return 0, true, true
	}
	mult := int64(0)
	switch s[len(s)-1] {
	case 'm':
		mult = 60
	case 'h':
		mult = 3600
	case 'd':
		mult = 86400
	default:
		return 0, false, false
	}
	n := atoiOr(s[:len(s)-1], -1)
	if n <= 0 {
		return 0, false, false
	}
	return int64(n) * mult, false, true
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
	// Per-link scope (pair-share-model), additive.
	ViewPanes []string `json:"view_panes"`
	Panes     []string `json:"panes"`
	ExpiresAt int64    `json:"expires_at,omitempty"`
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
		sg := shareGuest{ID: g.ID, Label: g.Name, EnrolledAt: g.EnrolledAt,
			ViewPanes: g.ViewPanes, Panes: g.InputPanes, ExpiresAt: g.ExpiresAt}
		if sg.ViewPanes == nil {
			sg.ViewPanes = []string{}
		}
		if sg.Panes == nil {
			sg.Panes = []string{}
		}
		out.Guests = append(out.Guests, sg)
	}
	return out
}

// guestScopeSummary renders one link's scope for the status list:
// "2 view · 1 type · expires 3h" (or "expired").
func guestScopeSummary(g deviceListEntry) string {
	s := fmt.Sprintf("%d view · %d type", len(g.ViewPanes), len(g.InputPanes))
	if g.ExpiresAt > 0 {
		left := g.ExpiresAt - time.Now().Unix()
		if left <= 0 {
			return s + " · " + i18n.Tr("expired", "已过期")
		}
		s += " · " + i18n.Tr("expires ", "剩 ") + fmtDurShort(left)
	}
	return s
}

// fmtDurShort renders seconds as "45m" / "3h" / "2d".
func fmtDurShort(secs int64) string {
	switch {
	case secs < 3600:
		return fmt.Sprintf("%dm", secs/60)
	case secs < 86400:
		return fmt.Sprintf("%dh", secs/3600)
	default:
		return fmt.Sprintf("%dd", secs/86400)
	}
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
	// Guest links = roster entries with scope "guest", each with ITS OWN scope.
	if guests := listGuests(base, token); len(guests) > 0 {
		i18n.Say(fmt.Sprintf("%d guest link(s):", len(guests)), fmt.Sprintf("%d 个分享链接：", len(guests)))
		for _, g := range guests {
			fmt.Printf("  %s  %-20s  %s  %s\n", g.ID, g.Name, fmtAgo(g.EnrolledAt), guestScopeSummary(g))
		}
		i18n.Say("Edit one: gtmux share set <id> --view %A,%B --type %A   ·   revoke: gtmux share revoke <id>",
			"改某个：gtmux share set <id> --view %A,%B --type %A   ·   吊销：gtmux share revoke <id>")
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
	printFanOutNotice(base, token)
	return shareStatus(base, token, false)
}

// shareClearView empties the VIEW allowlist (and thus the input allowlist too) — a
// guest goes back to seeing nothing.
func shareClearView(base, token string) int {
	body, _ := json.Marshal(map[string]any{"view_panes": []string{}, "panes": []string{}})
	if _, ok := postShareConfig(base, token, body); !ok {
		return 1
	}
	printFanOutNotice(base, token)
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
	printFanOutNotice(base, token)
	return shareStatus(base, token, false)
}

// printFanOutNotice reminds that a legacy GLOBAL edit fans out to every existing
// link (pair-share-model) — per-link tailoring should use `share set` instead.
func printFanOutNotice(base, token string) {
	if n := len(listGuests(base, token)); n > 0 {
		i18n.Say(fmt.Sprintf("(global edit — applied to all %d existing link(s); per-link: gtmux share set <id>)", n),
			fmt.Sprintf("（全局修改 —— 已应用到全部 %d 个链接;按链接改用 gtmux share set <id>）", n))
	}
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

// shareSet edits ONE guest link's scope (pair-share-model): per-flag replace —
// an omitted flag leaves that facet untouched.
func shareSet(base, token, id string, view, input *[]string, expires string) int {
	secs, clear, ok := parseExpires(expires)
	if !ok {
		i18n.Sae("gtmux share set: bad --expires (want e.g. 45m, 24h, 7d, never)",
			"gtmux share set: --expires 格式不对(如 45m、24h、7d、never)")
		return 2
	}
	if view == nil && input == nil && expires == "" {
		i18n.Sae("gtmux share set: nothing to change (give --view / --type / --expires)",
			"gtmux share set: 没有要改的(用 --view / --type / --expires)")
		return 2
	}
	payload := map[string]any{"id": id}
	if view != nil {
		payload["view"] = *view
	}
	if input != nil {
		payload["input"] = *input
	}
	if clear {
		payload["clearExpiry"] = true
	} else if secs > 0 {
		payload["expiresInSec"] = secs
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/share/set", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return shareUnreachable()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux share set: serve returned %d", resp.StatusCode),
			fmt.Sprintf("gtmux share set: 服务返回 %d", resp.StatusCode))
		return 1
	}
	return shareStatus(base, token, false)
}

func shareNew(base, token, label string, port int, jsonOut bool, view, input *[]string, expires string) int {
	secs, _, okExp := parseExpires(expires)
	if !okExp {
		i18n.Sae("gtmux share new: bad --expires (want e.g. 45m, 24h, 7d)",
			"gtmux share new: --expires 格式不对(如 45m、24h、7d)")
		return 2
	}
	payload := map[string]any{"label": label}
	if view != nil {
		payload["view"] = *view
	}
	if input != nil {
		payload["input"] = *input
	}
	if secs > 0 {
		payload["expiresInSec"] = secs
	}
	body, _ := json.Marshal(payload)
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

// shareLink re-hands an EXISTING guest link's URL by id. A link's token is shown
// only at mint time (`share new`), so a host who didn't copy it then had no way to
// get it back short of revoking + re-minting. This asks the local serve for the
// token (GET /api/share/link, full-scope only) and rebuilds the same base + `#t=`
// URL `share new` prints — so a menu-bar/app "Copy link" is one CLI call.
func shareLink(base, token, id string, jsonOut bool) int {
	req, _ := http.NewRequest(http.MethodGet, base+"/api/share/link?id="+url.QueryEscape(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return shareUnreachable()
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		i18n.Sae("gtmux share link: no such link '"+id+"'", "gtmux share link: 没有这个链接 '"+id+"'")
		return 1
	}
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux share link: serve returned %d", resp.StatusCode),
			fmt.Sprintf("gtmux share link: 服务返回 %d", resp.StatusCode))
		return 1
	}
	var out struct{ ID, Label, Token string }
	_ = json.NewDecoder(resp.Body).Decode(&out)

	shareBase := readTunnelURL()
	local := shareBase == ""
	if local {
		shareBase = base
	}
	if jsonOut {
		b, _ := json.MarshalIndent(buildShareNew(out.ID, out.Label, out.Token, shareBase), "", "  ")
		fmt.Println(string(b))
		return 0
	}
	link := shareBase + "/#t=" + out.Token
	fmt.Println()
	i18n.Say("Share link ("+out.ID+"):", "分享链接（"+out.ID+"）：")
	fmt.Printf("  %s\n", link)
	if local {
		i18n.Say("  (LOCAL address — run `gtmux tunnel` for a link others can open)",
			"  （本机地址 —— 想让别人能打开,先跑 `gtmux tunnel`）")
	}
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
		"usage: gtmux share [on|off | new [--label <name>] [--view <panes>] [--type <panes>] [--expires 24h] |\n"+
			"                    set <id> [--view <panes>] [--type <panes>] [--expires 24h|never] |\n"+
			"                    link <id> | add/remove <pane…> | view <add|remove|clear> [pane…] | revoke <id>] [--json]\n"+
			"  SHARE = a collaborator's scoped access (pair-share-model). Each link has\n"+
			"  ITS OWN scope: which panes they may SEE (--view) and TYPE into (--type ⊆ view),\n"+
			"  plus an optional expiry. Typing also needs the host consent: gtmux share on.\n"+
			"    · one-step link:  gtmux share new --label Alice --view %1,%2 --type %1\n"+
			"    · edit one link:  gtmux share set <id> --type %2 --expires 24h\n"+
			"  The legacy global forms (add/remove, view add/remove/clear) fan out to ALL links.\n"+
			"  --json makes `status` and `new` emit machine-readable output (no token).",
		"用法：gtmux share [on|off | new [--label <名>] [--view <panes>] [--type <panes>] [--expires 24h] |\n"+
			"                  set <id> [--view <panes>] [--type <panes>] [--expires 24h|never] |\n"+
			"                  link <id> | add/remove <pane…> | view <add|remove|clear> [pane…] | revoke <id>] [--json]\n"+
			"  SHARE = 协作者的受限访问(pair-share 模型)。每个链接有自己的范围：\n"+
			"  能看哪些 pane(--view)、能输入哪些(--type ⊆ view),外加可选过期;\n"+
			"  输入还需总闸同意:gtmux share on。\n"+
			"    · 一步建链接:gtmux share new --label 张三 --view %1,%2 --type %1\n"+
			"    · 改某个链接:gtmux share set <id> --type %2 --expires 24h\n"+
			"  旧的全局形式(add/remove、view …)会应用到全部链接。\n"+
			"  --json 让 `status` / `new` 输出机器可读格式(不含 token)。")
	return 0
}
