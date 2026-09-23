package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// A choice of Direct servers (openspec/changes/direct-server-choice).
//
// Direct was one server, so distance was not negotiable and an outage had nowhere to go.
// The servers are configuration the provisioner serves, and this file is the client half:
// it ASKS which servers exist rather than carrying a list, so a server the operator adds
// today is selectable by a gtmux that shipped months ago.
//
// Moving keeps the device's account and its port; only the host name changes. Paired
// phones follow through the addresses serve reports (see GET /api/addresses), so a move is
// a plain reconnect here.

// directPingPath is answered by the Direct server itself, with no Mac behind it: it is how
// a client times a server it has no account on, and how an operator checks a server with
// no device paired to it.
const directPingPath = "/__gtmux/ping"

// directServer is one server as the provisioner describes it. Everything is optional but
// the id and the URL: a server the operator adds with no label still has to render.
type directServer struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Region    string `json:"region,omitempty"`
	Accepting *bool  `json:"accepting,omitempty"`
	Label     struct {
		EN string `json:"en,omitempty"`
		ZH string `json:"zh,omitempty"`
	} `json:"label,omitempty"`
}

// name is what a reader should see: the label in their language, else the region, else the
// id. A server is a place, and an id like "sh" is not a place to anyone but the operator.
func (s directServer) name() string {
	if l := i18n.Tr(s.Label.EN, s.Label.ZH); strings.TrimSpace(l) != "" {
		return l
	}
	if s.Region != "" {
		return s.Region
	}
	return s.ID
}

// fetchDirectServers asks the provisioner which servers this Mac may use, and which one it
// is on now. The device id goes along so the answer can include servers reserved for this
// Mac's code, and say where it currently sits.
func fetchDirectServers() (servers []directServer, current string, err error) {
	body, _ := json.Marshal(map[string]string{"deviceId": resolveDeviceID()})
	api := tunnelAPI()
	bases := []string{api}
	if fb := tunnelAPIFallback(); fb != "" && fb != api {
		bases = append(bases, fb)
	}
	var lastErr error
	for _, base := range bases {
		req, e := http.NewRequest("POST", strings.TrimRight(base, "/")+"/direct/servers", bytes.NewReader(body))
		if e != nil {
			lastErr = e
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		res, e := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if e != nil {
			lastErr = e
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			lastErr = fmt.Errorf("HTTP %d", res.StatusCode)
			continue
		}
		var r struct {
			Servers []directServer `json:"servers"`
			Current string         `json:"current"`
		}
		if e := json.Unmarshal(data, &r); e != nil {
			lastErr = e
			continue
		}
		return r.Servers, r.Current, nil
	}
	return nil, "", lastErr
}

// pingDirect times one round trip to a server's liveness path. ok=false means the server
// did not answer, which is a state a reader needs: it is why their phone cannot reach this
// Mac, and the reason to move.
func pingDirect(url string) (time.Duration, bool) {
	req, err := http.NewRequest("GET", strings.TrimRight(url, "/")+directPingPath, nil)
	if err != nil {
		return 0, false
	}
	start := time.Now()
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return 0, false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<10))
	_ = res.Body.Close()
	if res.StatusCode >= 400 {
		return 0, false
	}
	return time.Since(start), true
}

// pingAll times every server at once: a list of five servers, one of them dead, should
// take the timeout once, not five times over.
func pingAll(servers []directServer) map[string]time.Duration {
	out := make(map[string]time.Duration, len(servers))
	type res struct {
		id string
		d  time.Duration
		ok bool
	}
	ch := make(chan res, len(servers))
	for _, s := range servers {
		go func(s directServer) {
			d, ok := pingDirect(s.URL)
			ch <- res{s.ID, d, ok}
		}(s)
	}
	for range servers {
		r := <-ch
		if r.ok {
			out[r.id] = r.d
		}
	}
	return out
}

// cmdTunnelServers prints the servers this Mac may use, each with the round trip measured
// from HERE, and marks the one it is on.
func cmdTunnelServers() int {
	servers, current, err := fetchDirectServers()
	if err != nil {
		i18n.Sae("gtmux tunnel: couldn't reach the unlock service (network?): "+err.Error(),
			"gtmux tunnel: 连不上解锁服务（网络？）："+err.Error())
		return 1
	}
	if len(servers) == 0 {
		i18n.Sae("gtmux tunnel: no Direct server is configured.", "gtmux tunnel: 没有配置任何 Direct 服务器。")
		return 1
	}
	if current == "" {
		current = readSelfTunnelServer()
	}
	took := pingAll(servers)
	sort.SliceStable(servers, func(a, b int) bool {
		da, oka := took[servers[a].ID]
		db, okb := took[servers[b].ID]
		if oka != okb {
			return oka // a server that answers sorts above one that does not
		}
		return da < db
	})
	i18n.Say("Direct servers (round trip measured from this Mac):", "Direct 服务器（延迟是这台 Mac 刚刚实测的）：")
	for _, s := range servers {
		mark := "  "
		if s.ID == current {
			mark = "→ "
		}
		rtt := i18n.Tr("no answer", "没有回应")
		if d, ok := took[s.ID]; ok {
			rtt = fmt.Sprintf("%d ms", d.Milliseconds())
		}
		line := fmt.Sprintf("%s%-12s %-10s %s", mark, s.ID, rtt, s.name())
		if s.ID == current {
			line += i18n.Tr("   (in use)", "   （正在使用）")
		} else if s.Accepting != nil && !*s.Accepting {
			line += i18n.Tr("   (not taking new devices)", "   （不接新设备）")
		}
		fmt.Println(line)
	}
	i18n.Say("Move this Mac:  gtmux tunnel --server <id>", "换一台：  gtmux tunnel --server <id>")
	return 0
}

// cmdTunnelMove moves this Mac to another Direct server. The account and the port stay the
// same; the address a phone uses changes, which is what the warning is about.
func cmdTunnelMove(id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		i18n.Sae("usage: gtmux tunnel --server <id>   (list them: gtmux tunnel --servers)",
			"用法：gtmux tunnel --server <id>   （看有哪些：gtmux tunnel --servers）")
		return 2
	}
	_, secret := readSelfTunnelConf()
	if secret == "" {
		i18n.Sae("gtmux tunnel: Direct isn't unlocked on this Mac. Redeem your access code first:  gtmux tunnel --redeem <code>",
			"gtmux tunnel: 这台 Mac 还没解锁 Direct。先用访问码解锁：  gtmux tunnel --redeem <码>")
		return 1
	}
	url, port, err := moveDirect(id, secret)
	if err != nil {
		switch err {
		case errNoSuchServer:
			i18n.Sae("gtmux tunnel: there is no Direct server called "+id+" (list them: gtmux tunnel --servers)",
				"gtmux tunnel: 没有叫 "+id+" 的 Direct 服务器（看有哪些：gtmux tunnel --servers）")
		case errUnknownDevice:
			i18n.Sae("gtmux tunnel: this Mac's Direct account was not accepted. Redeem your code again:  gtmux tunnel --redeem <code>",
				"gtmux tunnel: 这台 Mac 的 Direct 账号没有被接受。重新兑换一次：  gtmux tunnel --redeem <码>")
		default:
			i18n.Sae("gtmux tunnel: couldn't move this Mac (network?): "+err.Error(),
				"gtmux tunnel: 换不过去（网络？）："+err.Error())
		}
		return 1
	}
	if err := writeSelfTunnelConf(url, secret, port, id); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	i18n.Say("✓ This Mac is on "+id+" now: "+url, "✓ 这台 Mac 现在在 "+id+"："+url)
	i18n.Say("  Phones that have connected before follow on their own. A device that paired but never connected has to scan again,",
		"  之前连上来过的设备会自己跟过来。只扫过码、还没连上来过的设备要重新扫一次，")
	i18n.Say("  and guest links minted before this stop working (mint a new one: gtmux share).",
		"  换之前发出的分享链接会失效（重新生成：gtmux share）。")
	if selfTunnelRunning() {
		i18n.Say("  Restarting the tunnel so it connects there.", "  正在重启隧道，让它连过去。")
		return restartSelfTunnel()
	}
	i18n.Say("  Turn the tunnel on when you want it:  gtmux tunnel --backend self",
		"  想开隧道时：  gtmux tunnel --backend self")
	return 0
}

// errNoSuchServer / errUnknownDevice separate "you named a server that isn't there" from
// "this Mac's account was refused": one is a typo, the other means redeeming again.
var (
	errNoSuchServer  = fmt.Errorf("no such Direct server")
	errUnknownDevice = fmt.Errorf("device account not accepted")
)

// moveDirect asks the provisioner to reassign this device, authenticated by the account
// this Mac already holds — the only thing that proves it is this device.
func moveDirect(server, secret string) (url string, port int, err error) {
	body, _ := json.Marshal(map[string]string{
		"deviceId": resolveDeviceID(),
		"secret":   secret,
		"server":   server,
	})
	api := tunnelAPI()
	bases := []string{api}
	if fb := tunnelAPIFallback(); fb != "" && fb != api {
		bases = append(bases, fb)
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		for _, base := range bases {
			req, e := http.NewRequest("POST", strings.TrimRight(base, "/")+"/direct/move", bytes.NewReader(body))
			if e != nil {
				lastErr = e
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			res, e := (&http.Client{Timeout: 20 * time.Second}).Do(req)
			if e != nil {
				lastErr = e
				continue
			}
			data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
			_ = res.Body.Close()
			switch {
			case res.StatusCode == 404:
				return "", 0, errNoSuchServer
			case res.StatusCode == 403:
				return "", 0, errUnknownDevice
			case res.StatusCode != 200:
				lastErr = fmt.Errorf("HTTP %d", res.StatusCode)
				if res.StatusCode < 500 {
					return "", 0, lastErr
				}
				continue
			}
			var r struct {
				URL  string `json:"url"`
				Port int    `json:"port"`
			}
			if e := json.Unmarshal(data, &r); e != nil {
				lastErr = e
				continue
			}
			if r.URL == "" {
				return "", 0, fmt.Errorf("incomplete move response")
			}
			return r.URL, r.Port, nil
		}
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	return "", 0, lastErr
}

// selfTunnelRunning reports whether the always-on Direct tunnel is registered, so a move
// can put the Mac back where it was instead of leaving it off.
func selfTunnelRunning() bool {
	return fileExists(selfTunnelAgentPath())
}

// restartSelfTunnel reloads the always-on Direct tunnel so it dials the server the config
// now names. The tunnel client re-reads selftunnel.conf at start, so a reload is the whole
// move on this side; the pairing address is rewritten first, since that is what the menu
// bar and `gtmux pair` hand out.
func restartSelfTunnel() int {
	if url, _ := readSelfTunnelConf(); url != "" {
		writeTunnelURL(selfTunnelPairURL(url))
	}
	launchctl("unload", selfTunnelAgentPath())
	if err := launchctl("load", selfTunnelAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: couldn't restart the tunnel: "+err.Error()+" (start it again with `gtmux tunnel --backend self --service`)",
			"gtmux tunnel: 隧道没能重启："+err.Error()+"（用 `gtmux tunnel --backend self --service` 重新开）")
		return 1
	}
	return 0
}
