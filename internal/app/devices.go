package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// cmdDevices implements `gtmux devices` — list the phones paired via per-device
// tokens, and revoke one. It talks to the LOCAL running radar (which owns the
// roster in memory); editing the file under a live serve would just get clobbered,
// so management goes through the server so a revoke takes effect immediately.
//
//	gtmux devices                     list paired devices
//	gtmux devices revoke <id>         revoke a device's token (effective now)
//	gtmux devices --push              list devices annotated with their push token
//	gtmux devices --forget-push <sel> drop push tokens (<id> | orphans | all)
//	gtmux devices [--port N]
func cmdDevices(args []string) int {
	port := defaultServePort
	revokeID := ""
	pushMode := false
	forgetSel := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			i18n.Say("usage: gtmux devices [--port N] [--push] [--forget-push <id|orphans|all>]  |  gtmux devices revoke <id>",
				"用法：gtmux devices [--port N] [--push] [--forget-push <id|orphans|all>]  |  gtmux devices revoke <id>")
			return 0
		case a == "--push":
			pushMode = true
		case a == "--forget-push":
			if i+1 < len(args) {
				forgetSel = args[i+1]
				i++
			} else {
				i18n.Sae("gtmux devices --forget-push: missing <id|orphans|all>",
					"gtmux devices --forget-push: 缺少 <id|orphans|all>")
				return 2
			}
		case a == "revoke":
			if i+1 < len(args) {
				revokeID = args[i+1]
				i++
			} else {
				i18n.Sae("gtmux devices revoke: missing <id>", "gtmux devices revoke: 缺少 <id>")
				return 2
			}
		case a == "--port" || a == "-p":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err != nil || n <= 0 || n > 65535 {
					i18n.Sae("gtmux devices: invalid --port", "gtmux devices: 无效的 --port")
					return 2
				}
				port = n
				i++
			}
		case strings.HasPrefix(a, "--port="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--port="))
			if err != nil || n <= 0 || n > 65535 {
				i18n.Sae("gtmux devices: invalid --port", "gtmux devices: 无效的 --port")
				return 2
			}
			port = n
		}
	}

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	token := resolveServeToken("")
	if forgetSel != "" {
		return forgetPush(base, token, forgetSel)
	}
	if pushMode {
		return listPush(base, token)
	}
	if revokeID != "" {
		return revokeDevice(base, token, revokeID)
	}
	return listDevices(base, token)
}

// pushTokenRow mirrors GET /api/push/tokens (redacted — prefix only, never the token).
type pushTokenRow struct {
	DeviceID    string   `json:"deviceId"`
	TokenPrefix string   `json:"tokenPrefix"`
	Platform    string   `json:"platform"`
	Env         string   `json:"env"`
	Kinds       []string `json:"kinds"`
}

// listPush renders the roster annotated with each device's push token (env·kinds), and
// lists any UNLINKED (legacy, empty-deviceId) tokens separately — the ones a revoke
// can't drop, cleared with `--forget-push orphans`.
func listPush(base, token string) int {
	req, _ := http.NewRequest(http.MethodGet, base+"/api/push/tokens", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		i18n.Sae("gtmux devices: can't reach the local serve — start it with `gtmux serve`.",
			"gtmux devices: 连不上本地 serve —— 先用 `gtmux serve` 启动。")
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux devices: serve returned %d (push tokens are host-only)", resp.StatusCode),
			fmt.Sprintf("gtmux devices: 服务返回 %d（推送 token 仅限本机）", resp.StatusCode))
		return 1
	}
	var out struct {
		Tokens []pushTokenRow `json:"tokens"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		i18n.Sae("gtmux devices: bad response", "gtmux devices: 响应解析失败")
		return 1
	}
	byDevice := map[string]pushTokenRow{}
	var orphans []pushTokenRow
	for _, t := range out.Tokens {
		if t.DeviceID == "" {
			orphans = append(orphans, t)
		} else {
			byDevice[t.DeviceID] = t
		}
	}
	devices, ok := fetchDevices(base, token)
	if !ok {
		return 1
	}
	i18n.Say("Push tokens by device:", "各设备的推送 token：")
	if len(devices) == 0 {
		i18n.Say("  (no paired devices)", "  （没有配对设备）")
	}
	for _, d := range devices {
		mark := i18n.Tr("— no push token", "— 无推送 token")
		if t, has := byDevice[d.ID]; has {
			env := t.Env
			if env == "" {
				env = "?"
			}
			kinds := i18n.Tr("all", "全部")
			if len(t.Kinds) > 0 {
				kinds = strings.Join(t.Kinds, ",")
			}
			mark = fmt.Sprintf("✓ push %s·%s", env, kinds)
		}
		fmt.Printf("  %s  %-24s  %s\n", d.ID, d.Name, mark)
	}
	if len(orphans) > 0 {
		fmt.Println()
		i18n.Say(fmt.Sprintf("%d unlinked push token(s) (registered before device-binding):", len(orphans)),
			fmt.Sprintf("%d 个未关联的推送 token（在设备绑定之前注册的）：", len(orphans)))
		for _, t := range orphans {
			fmt.Printf("  %s…  %s\n", t.TokenPrefix, t.Platform)
		}
		i18n.Say("Clear them:  gtmux devices --forget-push orphans", "清除：  gtmux devices --forget-push orphans")
	}
	return 0
}

// forgetPush drops push tokens by selector: an id → that device's token(s); "orphans" →
// only unlinked legacy tokens; "all" → every token. Revoking a device already drops its
// token, so this is mainly for orphans and belt-and-suspenders cleanup.
func forgetPush(base, token, sel string) int {
	payload := map[string]any{}
	switch sel {
	case "orphans":
		payload["orphans"] = true
	case "all":
		payload["all"] = true
	default:
		payload["deviceId"] = sel
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/push/forget", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		i18n.Sae("gtmux devices: can't reach the local serve — start it with `gtmux serve`.",
			"gtmux devices: 连不上本地 serve —— 先用 `gtmux serve` 启动。")
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux devices: serve returned %d (push tokens are host-only)", resp.StatusCode),
			fmt.Sprintf("gtmux devices: 服务返回 %d（推送 token 仅限本机）", resp.StatusCode))
		return 1
	}
	var out struct {
		Forgotten int `json:"forgotten"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	i18n.Say(fmt.Sprintf("✓ dropped %d push token(s) — those devices stop receiving notifications.", out.Forgotten),
		fmt.Sprintf("✓ 已删除 %d 个推送 token —— 这些设备不再收到通知。", out.Forgotten))
	return 0
}

// deviceListEntry mirrors the GET /api/devices shape (no tokens). Scope is "" for a
// paired device or "guest" for a share link.
type deviceListEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EnrolledAt int64  `json:"enrolledAt"`
	LastSeen   int64  `json:"lastSeen"`
	Scope      string `json:"scope,omitempty"`
	// Per-link guest scope (pair-share-model); absent on owner devices.
	ViewPanes  []string `json:"viewPanes,omitempty"`
	InputPanes []string `json:"inputPanes,omitempty"`
	ExpiresAt  int64    `json:"expiresAt,omitempty"`
}

// fetchDevices GETs the roster (shared by `gtmux devices` and `gtmux pair list`).
func fetchDevices(base, token string) ([]deviceListEntry, bool) {
	req, _ := http.NewRequest(http.MethodGet, base+"/api/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		i18n.Sae("gtmux: can't reach the local serve — start it with `gtmux serve` (or `gtmux tunnel`).",
			"gtmux: 连不上本地 serve —— 先用 `gtmux serve`（或 `gtmux tunnel`）启动。")
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux: serve returned %d", resp.StatusCode),
			fmt.Sprintf("gtmux: 服务返回 %d", resp.StatusCode))
		return nil, false
	}
	var out struct {
		Devices []deviceListEntry `json:"devices"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		i18n.Sae("gtmux: bad response", "gtmux: 响应解析失败")
		return nil, false
	}
	return out.Devices, true
}

func listDevices(base, token string) int {
	req, _ := http.NewRequest(http.MethodGet, base+"/api/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		i18n.Sae("gtmux devices: can't reach the local radar — start it with `gtmux serve` (or `gtmux tunnel`).",
			"gtmux devices: 连不上本地雷达 —— 先用 `gtmux serve`（或 `gtmux tunnel`）启动。")
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i18n.Sae(fmt.Sprintf("gtmux devices: radar returned %d", resp.StatusCode),
			fmt.Sprintf("gtmux devices: 雷达返回 %d", resp.StatusCode))
		return 1
	}
	var out struct {
		Devices []deviceListEntry `json:"devices"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		i18n.Sae("gtmux devices: bad response", "gtmux devices: 响应解析失败")
		return 1
	}
	if len(out.Devices) == 0 {
		i18n.Say("No paired devices yet. Pair one with `gtmux tunnel` (scan the QR).",
			"还没有配对的设备。用 `gtmux tunnel` 配一台（扫码）。")
		return 0
	}
	i18n.Say(fmt.Sprintf("%d paired device(s):", len(out.Devices)),
		fmt.Sprintf("已配对 %d 台设备：", len(out.Devices)))
	for _, d := range out.Devices {
		fmt.Printf("  %s  %-24s  paired %s%s\n", d.ID, deviceDisplayName(d.Name), fmtAgo(d.EnrolledAt), lastSeenSuffix(d.LastSeen))
	}
	i18n.Say("Revoke one:  gtmux devices revoke <id>", "吊销某台：  gtmux devices revoke <id>")
	return 0
}

func revokeDevice(base, token, id string) int {
	body, _ := json.Marshal(map[string]string{"id": id})
	req, _ := http.NewRequest(http.MethodPost, base+"/api/devices/revoke", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		i18n.Sae("gtmux devices: can't reach the local radar — start it with `gtmux serve`.",
			"gtmux devices: 连不上本地雷达 —— 先用 `gtmux serve` 启动。")
		return 1
	}
	defer resp.Body.Close()
	var out struct {
		Revoked bool `json:"revoked"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if !out.Revoked {
		i18n.Sae("No device with id "+id+".", "没有 id 为 "+id+" 的设备。")
		return 1
	}
	i18n.Say("✓ revoked "+id+" — its token no longer works.",
		"✓ 已吊销 "+id+" —— 该 token 即刻失效。")
	return 0
}

// fmtAgo renders a unix time as a coarse "Nm/h/d ago" (bilingual-neutral digits).
func fmtAgo(unix int64) string {
	if unix == 0 {
		return "?"
	}
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return i18n.Tr("just now", "刚刚")
	case d < time.Hour:
		return fmt.Sprintf("%dm %s", int(d.Minutes()), i18n.Tr("ago", "前"))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %s", int(d.Hours()), i18n.Tr("ago", "前"))
	default:
		return fmt.Sprintf("%dd %s", int(d.Hours()/24), i18n.Tr("ago", "前"))
	}
}

func lastSeenSuffix(unix int64) string {
	if unix == 0 {
		return ""
	}
	return "  ·  " + i18n.Tr("last seen ", "最近活跃 ") + fmtAgo(unix)
}
