// Package usercfg centralizes access to gtmux's user config file
// (~/.config/gtmux/config.json). The path plus the read+unmarshal boilerplate used to
// be re-hardcoded in every package that reads a knob (agentenv, dispatch, events,
// hqfeed, hqwake, resource, hook, app) — so a relocation, or the kind of
// symlink-normalization fix that already bit the HQ pane resolver, needed a coordinated
// edit in ~8 places. Now it lives here. (Named `usercfg`, not `config`/`cfg`, because
// several of those packages already have a local `config` type or `cfg` variable.)
package usercfg

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Path is gtmux's user config file — the single source of truth for its location.
func Path() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json")
}

// Load reads the config file and unmarshals it into dst. A MISSING file is NOT an error
// — dst keeps whatever (default) values it already holds and Load returns nil — because
// every knob falls back to a default; callers set their defaults on dst BEFORE calling.
// A present-but-malformed file returns the unmarshal error, which callers that want
// "malformed → keep defaults" turn into a default return (`if usercfg.Load(&c) != nil`).
func Load(dst any) error {
	b, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(b, dst)
}
