package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Where else this Mac answers (openspec/changes/direct-server-choice).
//
// A paired device stores the address it scanned, and that address carries the host name of
// one Direct server. When the Mac moves to another server, the address dies and the device
// has nothing to try: it reports the Mac unreachable until someone scans a new pairing
// code. So a device asks the Mac itself where else it can be found, and keeps the answer.
//
// The list is written by the tunnel (internal/app), which is what knows the servers. serve
// only hands it over, to callers that are already authenticated — the addresses are not a
// secret, but they are nobody else's business either.

type addressesReply struct {
	Addresses []string       `json:"addresses"`
	Current   string         `json:"current,omitempty"`
	Server    *addressServer `json:"server,omitempty"`
}

// addressServer is which server carries this Mac right now, as a PLACE: its name in both
// languages, so each surface renders the one its reader uses. Absent when there is nothing
// to name (a LAN address, the standard tunnel, a serve with no Direct server list).
type addressServer struct {
	ID string `json:"id"`
	EN string `json:"en,omitempty"`
	ZH string `json:"zh,omitempty"`
}

func (s *Server) handleAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("GET only"))
		return
	}
	addrs, srv := readTunnelAddresses()
	reply := addressesReply{Addresses: addrs, Server: srv}
	if len(addrs) > 0 {
		reply.Current = addrs[0]
	}
	writeJSON(w, http.StatusOK, reply)
}

// readTunnelAddresses reads what the tunnel published, current first. Anything that is not
// an https URL is dropped: a client is going to send its token to these, so a line that
// does not parse must never become an address it tries.
//
// Two shapes are read. The one written today is an object with the addresses and the
// server; a file left by the version before that is a bare array, and a running tunnel is
// not restarted just because gtmux was updated under it.
func readTunnelAddresses() ([]string, *addressServer) {
	b, err := os.ReadFile(state.TunnelAddressesPath())
	if err != nil {
		return []string{}, nil
	}
	var raw []string
	var srv *addressServer
	var file struct {
		Addresses []string       `json:"addresses"`
		Server    *addressServer `json:"server"`
	}
	if err := json.Unmarshal(b, &file); err == nil && len(file.Addresses) > 0 {
		raw, srv = file.Addresses, file.Server
	} else if err := json.Unmarshal(b, &raw); err != nil {
		return []string{}, nil
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, a := range raw {
		a = strings.TrimSpace(a)
		u, err := url.Parse(a)
		if err != nil || u.Scheme != "https" || u.Host == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
		if len(out) == 8 { // a pool, not a phone book
			break
		}
	}
	if srv != nil && srv.ID == "" {
		srv = nil // a server with no id names nothing
	}
	return out, srv
}
