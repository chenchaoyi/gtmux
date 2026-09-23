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
	Addresses []string `json:"addresses"`
	Current   string   `json:"current,omitempty"`
}

func (s *Server) handleAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("GET only"))
		return
	}
	addrs := readTunnelAddresses()
	reply := addressesReply{Addresses: addrs}
	if len(addrs) > 0 {
		reply.Current = addrs[0]
	}
	writeJSON(w, http.StatusOK, reply)
}

// readTunnelAddresses reads the list the tunnel published, current first. Anything that is
// not an https URL is dropped: a client is going to send its token to these, so a line that
// does not parse must never become an address it tries.
func readTunnelAddresses() []string {
	b, err := os.ReadFile(state.TunnelAddressesPath())
	if err != nil {
		return []string{}
	}
	var raw []string
	if err := json.Unmarshal(b, &raw); err != nil {
		return []string{}
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
	return out
}
