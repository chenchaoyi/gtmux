package app

import "os"

// Push-relay config. RelayURL is gtmux's hosted relay Worker; RelayToken is baked
// at release build (-ldflags ...RelayToken=...) from a CI secret. Both overridable
// at runtime (GTMUX_RELAY_URL / GTMUX_RELAY_TOKEN) for self-hosters. With these
// set, `gtmux serve` (incl. the one `gtmux tunnel` / always-on starts) forwards
// agent alerts to APNs by default — so the phone gets notifications without any
// extra flags.
var (
	RelayURL   = "https://gtmux-relay.ccy.dev"
	RelayToken = ""
)

// resolveRelay returns the relay URL + token (env overrides the baked defaults).
func resolveRelay() (url, token string) {
	url = RelayURL
	if v := os.Getenv("GTMUX_RELAY_URL"); v != "" {
		url = v
	}
	token = RelayToken
	if v := os.Getenv("GTMUX_RELAY_TOKEN"); v != "" {
		token = v
	}
	return url, token
}
