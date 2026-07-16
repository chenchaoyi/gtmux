package connect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// remotes.json persists the OWNER device tokens this machine's terminal earned by
// redeeming pair codes (pair-share-model S2): {"<base-url>": "<device-token>"}.
// Mode 0600 — it holds live credentials; `gtmux pair revoke` on the host kills an
// entry's power instantly (the next request fails auth).
func remotesPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "remotes.json")
}

// LoadRemoteToken returns the persisted owner token for base ("" when none).
func LoadRemoteToken(base string) string {
	b, err := os.ReadFile(remotesPath())
	if err != nil {
		return ""
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	return m[normalizeRemoteKey(base)]
}

// SaveRemoteToken persists base → token (0600, creating the config dir).
func SaveRemoteToken(base, token string) error {
	m := map[string]string{}
	if b, err := os.ReadFile(remotesPath()); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	m[normalizeRemoteKey(base)] = token
	if err := os.MkdirAll(filepath.Dir(remotesPath()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(remotesPath(), b, 0o600)
}

// normalizeRemoteKey keys entries by the normalized base URL so `gtmux attach host`
// and the full URL land on the same record.
func normalizeRemoteKey(base string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/")
}
