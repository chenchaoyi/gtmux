package connect

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// remotes.json persists the OWNER device tokens this machine's terminal earned by
// redeeming pair codes (pair-share-model S2): {"<base-url>": "<device-token>"}.
// Mode 0600 — it holds live credentials; `gtmux pair revoke` on the host kills an
// entry's power instantly (the next request fails auth).
func remotesPath() string {
	return filepath.Join(state.ConfigDir(), "remotes.json")
}

// Remote is what this terminal knows about one host: the token, and every address that
// host said it answers at (GET /api/addresses). The addresses are how a bare
// `gtmux attach <host>` still finds a Mac that moved to another Direct server
// (openspec/changes/direct-server-choice); they are not credentials.
type Remote struct {
	Token string   `json:"token"`
	Alts  []string `json:"alts,omitempty"`
}

// loadRemotes reads the file in BOTH shapes. Records written before this change are a
// bare token string, and a terminal that has paired before must not lose its pairing to
// a format change.
func loadRemotes() map[string]Remote {
	out := map[string]Remote{}
	b, err := os.ReadFile(remotesPath())
	if err != nil {
		return out
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		return out
	}
	for k, v := range raw {
		var tok string
		if json.Unmarshal(v, &tok) == nil {
			out[k] = Remote{Token: tok}
			continue
		}
		var r Remote
		if json.Unmarshal(v, &r) == nil && r.Token != "" {
			out[k] = r
		}
	}
	return out
}

func saveRemotes(m map[string]Remote) error {
	if err := os.MkdirAll(filepath.Dir(remotesPath()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(remotesPath(), b, 0o600)
}

// LoadRemote returns everything known about base.
func LoadRemote(base string) Remote { return loadRemotes()[normalizeRemoteKey(base)] }

// LoadRemoteToken returns the persisted owner token for base ("" when none).
func LoadRemoteToken(base string) string { return LoadRemote(base).Token }

// SaveRemoteToken persists base → token (0600, creating the config dir), keeping any
// addresses already known for that host.
func SaveRemoteToken(base, token string) error {
	m := loadRemotes()
	key := normalizeRemoteKey(base)
	r := m[key]
	r.Token = token
	m[key] = r
	return saveRemotes(m)
}

// SaveRemoteAddresses records where else this host answers. Nothing else changes: the
// token stays exactly as it was.
func SaveRemoteAddresses(base string, alts []string) error {
	m := loadRemotes()
	key := normalizeRemoteKey(base)
	r, ok := m[key]
	if !ok {
		return nil // a host this terminal has no pairing with has nothing to remember
	}
	r.Alts = alts
	m[key] = r
	return saveRemotes(m)
}

// MoveRemote records that a host now answers at another address: the record moves with
// its token, so the next `gtmux attach <new host>` needs no pairing, and the old key is
// dropped rather than left to look live.
func MoveRemote(from, to string) error {
	m := loadRemotes()
	fromKey, toKey := normalizeRemoteKey(from), normalizeRemoteKey(to)
	r, ok := m[fromKey]
	if !ok || fromKey == toKey {
		return nil
	}
	delete(m, fromKey)
	m[toKey] = r
	return saveRemotes(m)
}

// normalizeRemoteKey keys entries by the normalized base URL so `gtmux attach host`
// and the full URL land on the same record.
func normalizeRemoteKey(base string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/")
}

// FindMoved asks each address this terminal remembers for a host, in order, and answers
// with the first that is there AND takes our token. "" when none is: the host is simply
// unreachable, which is what the caller then says.
//
// Trying another address is safe by construction: a Mac's reverse port is unique across
// the whole fleet, so nothing but that Mac is ever behind it, on any Direct server.
func FindMoved(ctx context.Context, base, token string) string {
	for _, alt := range LoadRemote(base).Alts {
		if alt == "" || normalizeRemoteKey(alt) == normalizeRemoteKey(base) {
			continue
		}
		probe := NewClient(alt, token)
		if !probe.Health(ctx) {
			continue
		}
		if _, err := probe.Share(ctx); err != nil {
			continue // answered, but not to us: not this host
		}
		return alt
	}
	return ""
}
