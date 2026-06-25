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
//	gtmux devices                list paired devices
//	gtmux devices revoke <id>    revoke a device's token (effective now)
//	gtmux devices [--port N]
func cmdDevices(args []string) int {
	port := defaultServePort
	revokeID := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			i18n.Say("usage: gtmux devices [--port N]  |  gtmux devices revoke <id>",
				"用法：gtmux devices [--port N]  |  gtmux devices revoke <id>")
			return 0
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
	if revokeID != "" {
		return revokeDevice(base, token, revokeID)
	}
	return listDevices(base, token)
}

// deviceListEntry mirrors the GET /api/devices shape (no tokens).
type deviceListEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EnrolledAt int64  `json:"enrolledAt"`
	LastSeen   int64  `json:"lastSeen"`
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
		fmt.Printf("  %s  %-24s  paired %s%s\n", d.ID, d.Name, fmtAgo(d.EnrolledAt), lastSeenSuffix(d.LastSeen))
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
