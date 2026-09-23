package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/state"
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
		if res.StatusCode == 404 {
			// A provisioner older than this gtmux: it has one server and no way to say so.
			// That is not an error — it is the deployment every Direct user had until now.
			return localDirectServer(), readSelfTunnelServer(), nil
		}
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

// localDirectServer is what this Mac knows without asking anyone: the server its own
// config names. It answers for a provisioner that has no server list, so `--servers` shows
// the truth (one server, the one in use) instead of an error about the network.
func localDirectServer() []directServer {
	url, _ := readSelfTunnelConf()
	if url == "" {
		return nil
	}
	id := readSelfTunnelServer()
	if id == "" {
		id = "default"
	}
	return []directServer{{ID: id, URL: url}}
}

// pingDirect times one round trip to a server's liveness path. ok=false means the server
// did not answer AT ALL — no route, no TLS, no reply before the timeout — which is a state
// a reader needs: it is why their phone cannot reach this Mac, and the reason to move.
//
// Any HTTP status counts as an answer, 404 included: a server installed before this path
// existed still answers, and calling that "no answer" would tell a reader their healthy
// server is down.
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
// from HERE, and marks the one it is on. With --json it is the same answer as data, which
// is what the menu bar renders: that surface is a consumer of this command, never a second
// implementation of it.
func cmdTunnelServers(asJSON bool) int {
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
	if asJSON {
		return printDirectServersJSON(servers, took, current)
	}
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

// printDirectServersJSON is the machine-readable half: every server as the provisioner
// described it, plus what this Mac measured. A server that did not answer carries no round
// trip at all rather than a zero, which a reader would draw as instant.
func printDirectServersJSON(servers []directServer, took map[string]time.Duration, current string) int {
	type row struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		Region    string `json:"region,omitempty"`
		Name      string `json:"name"`
		Current   bool   `json:"current"`
		Accepting bool   `json:"accepting"`
		Answering bool   `json:"answering"`
		RTTms     *int64 `json:"rtt_ms,omitempty"`
	}
	out := make([]row, 0, len(servers))
	for _, s := range servers {
		r := row{ID: s.ID, URL: s.URL, Region: s.Region, Name: s.name(),
			Current: s.ID == current, Accepting: s.Accepting == nil || *s.Accepting}
		if d, ok := took[s.ID]; ok {
			ms := d.Milliseconds()
			r.Answering, r.RTTms = true, &ms
		}
		out = append(out, r)
	}
	b, err := json.MarshalIndent(map[string]any{"servers": out, "current": current}, "", "  ")
	if err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	fmt.Println(string(b))
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
		pairURL := selfTunnelPairURL(url)
		writeTunnelURL(pairURL)
		publishTunnelAddresses(pairURL)
	}
	launchctl("unload", selfTunnelAgentPath())
	if err := launchctl("load", selfTunnelAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: couldn't restart the tunnel: "+err.Error()+" (start it again with `gtmux tunnel --backend self --service`)",
			"gtmux tunnel: 隧道没能重启："+err.Error()+"（用 `gtmux tunnel --backend self --service` 重新开）")
		return 1
	}
	return 0
}

// Every address this Mac could answer at (openspec/changes/direct-server-choice).
//
// A phone stores the address it paired to, which carries the server's host name. When the
// Mac moves to another server that address dies, and before this the phone had no way to
// learn the new one: the Mac was simply unreachable until someone scanned a fresh pairing
// code. So the tunnel writes down where else this Mac can be found — its own port on each
// server it may use — serve hands that list to authenticated clients, and they try the
// others when the saved one stops answering.
//
// Probing another server is safe by construction: a device's port is unique across the
// whole fleet, so nothing but this Mac is ever behind /p<port>, on any server. A probe that
// finds nothing finds nothing.

// tunnelAddressFile is what the tunnel writes down for serve to hand out: where this Mac
// answers, and WHICH SERVER it is on. The name travels with it because only this side can
// resolve it — a phone reading "sh" would be reading an id we keep out of every surface.
type tunnelAddressFile struct {
	Addresses []string      `json:"addresses"`
	Server    *tunnelServer `json:"server,omitempty"`
}

// tunnelServer is the current server as a reader should meet it: a place, in both
// languages, so each surface renders the one its reader uses.
type tunnelServer struct {
	ID string `json:"id"`
	EN string `json:"en,omitempty"`
	ZH string `json:"zh,omitempty"`
}

// writeTunnelAddresses records the list, current first, with the server this Mac is on.
// Best-effort: an address list that could not be written costs a phone a rescan after a
// move, never anything live.
func writeTunnelAddresses(addrs []string, srv *tunnelServer) {
	if len(addrs) == 0 {
		return
	}
	b, err := json.Marshal(tunnelAddressFile{Addresses: addrs, Server: srv})
	if err != nil {
		return
	}
	_ = os.WriteFile(state.TunnelAddressesPath(), append(b, '\n'), 0o600)
}

func removeTunnelAddresses() { _ = os.Remove(state.TunnelAddressesPath()) }

// publishTunnelAddresses writes the list for the address this tunnel is serving. It runs
// in the background: a slow or unreachable provisioner must never hold up the tunnel, and
// the current address is already published before this starts.
func publishTunnelAddresses(current string) {
	if current == "" {
		return
	}
	writeTunnelAddresses([]string{current}, nil)
	go func() {
		addrs, srv := directAddresses(current)
		writeTunnelAddresses(addrs, srv)
	}()
}

// directAddresses is the pairing URL this Mac is reachable at now, followed by the same
// path on every other server it may use, and the server it is on. The provisioner is asked
// once, and a failure to reach it is not an error: the current address alone is what every
// older gtmux published.
func directAddresses(current string) ([]string, *tunnelServer) {
	out := []string{}
	if current != "" {
		out = append(out, current)
	}
	port := readSelfTunnelPort()
	if port == 0 {
		return out, nil
	}
	servers, currentID, err := fetchDirectServers()
	if err != nil {
		return out, nil
	}
	if currentID == "" {
		currentID = readSelfTunnelServer()
	}
	var srv *tunnelServer
	for _, s := range servers {
		if s.ID == currentID {
			srv = &tunnelServer{ID: s.ID, EN: s.Label.EN, ZH: s.Label.ZH}
			if srv.EN == "" && srv.ZH == "" && s.Region != "" {
				srv.EN, srv.ZH = s.Region, s.Region
			}
		}
		u := selfTunnelPairURLPort(s.URL, port)
		if u != "" && u != current {
			out = append(out, u)
		}
	}
	return out, srv
}

// The owner's phone asking which routes exist, and asking for a move
// (openspec/changes/phone-moves-the-route). serve carries the request; this is the same
// operation the menu bar and `gtmux tunnel --server <id>` perform, so there is one way a
// Mac moves, whoever asked.

// directRoutesForServe is every route this Mac may take, with the address it has on each,
// so the phone can time them ITSELF. It carries no round trip: the Mac's measurement
// answers a different question than "what does my connection cost from here".
func directRoutesForServe() ([]server.RouteInfo, error) {
	// Routes exist only while this Mac IS on Direct. On the standard tunnel, or on a LAN
	// address, there is nothing to choose, and a list of places a phone cannot be sent to
	// is worse than no list: the Mac answers "no routes" and the phone shows nothing.
	// The Mac decides this, not the phone, so no surface has to guess.
	tunnelURL := readTunnelURL()
	if tunnelURL == "" {
		return nil, nil
	}
	servers, current, err := fetchDirectServers()
	if err != nil {
		return nil, err
	}
	onDirect := false
	for _, s := range servers {
		if base := strings.TrimRight(s.URL, "/"); base != "" && strings.HasPrefix(tunnelURL, base) {
			onDirect = true
			break
		}
	}
	if !onDirect {
		return nil, nil
	}
	if current == "" {
		current = readSelfTunnelServer()
	}
	port := readSelfTunnelPort()
	out := make([]server.RouteInfo, 0, len(servers))
	for _, s := range servers {
		out = append(out, server.RouteInfo{
			ID:      s.ID,
			Name:    s.name(),
			EN:      s.Label.EN,
			ZH:      s.Label.ZH,
			URL:     selfTunnelPairURLPort(s.URL, port),
			Current: s.ID == current,
		})
	}
	return out, nil
}

// moveDirectRoute performs the move for a remote owner. It refuses a route this Mac may
// not use, and it republishes the pairing address and the address list before restarting
// the tunnel, exactly as the local path does — the phone that asked is about to look for
// this Mac through those addresses.
func moveDirectRoute(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("no route named")
	}
	_, secret := readSelfTunnelConf()
	if secret == "" {
		return fmt.Errorf("direct is not unlocked on this Mac")
	}
	url, port, err := moveDirect(id, secret)
	if err != nil {
		return err
	}
	if err := writeSelfTunnelConf(url, secret, port, id); err != nil {
		return err
	}
	if selfTunnelRunning() {
		if rc := restartSelfTunnel(); rc != 0 {
			return fmt.Errorf("moved, but the tunnel did not restart")
		}
		return nil
	}
	// No always-on tunnel: the config now names the new route, and the next start uses
	// it. Say nothing else — the caller asked for a move, and the move happened.
	pairURL := selfTunnelPairURLPort(url, port)
	writeTunnelURL(pairURL)
	publishTunnelAddresses(pairURL)
	return nil
}
