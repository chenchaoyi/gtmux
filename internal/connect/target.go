// Package connect is the `gtmux connect` remote terminal client: a Bubble Tea TUI
// that drives a remote gtmux serve's sessions from another machine's terminal, in
// owner or guest scope, over the same HTTP/SSE contract the web page + mobile app use.
// (remote-terminal-client change.) The CLI stays cgo-free — Bubble Tea is pure Go.
package connect

import (
	"fmt"
	"regexp"
	"strings"
)

// Scope is how the connection is authorized: owner (a device/master token — full) or
// guest (a `gtmux share` token — restricted by the host's view/input allowlists). The
// server enforces it; the TUI only reflects it.
type Scope string

const (
	ScopeOwner Scope = "owner"
	ScopeGuest Scope = "guest"
)

// Target is a resolved connection: the base URL, the bearer token, and the scope the
// token was minted for (confirmed later by GET /api/share). EnrollCode is set when
// the target is a PAIR link (`#c=<code>`, pair-share-model S2): the code is redeemed
// once for an owner device token before connecting.
type Target struct {
	URL        string
	Token      string
	Scope      Scope
	EnrollCode string
}

var shareLinkRe = regexp.MustCompile(`^(https?://[^#]+?)/*#(.*)$`)

// `#g=` is the guest fragment `gtmux share new` mints; `#t=` is the legacy form —
// still accepted so links minted before the rename keep working.
var shareTokenRe = regexp.MustCompile(`(?:^|[?&])[gt]=([^&]+)`)
var enrollCodeRe = regexp.MustCompile(`(?:^|[?&])c=([^&]+)`)

// ParseTarget resolves a `gtmux connect` argument into a Target. A guest share link
// (`https://host/#g=<token>`, what `gtmux share new` mints; legacy `#t=` also
// accepted) yields a GUEST target
// carrying that token. A PAIR link (`https://host/#c=<code>`, what `gtmux pair`
// prints) yields an OWNER target carrying the enroll code — redeemed once for a
// persisted device token. Otherwise the argument is a host: `token` (from --token)
// or a previously-persisted remote token (remotes.json) is the OWNER bearer; the
// host is normalized (http:// + :8765 defaults) like the mobile app. An owner
// target with no credential is an error.
func ParseTarget(arg, token string) (Target, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return Target{}, fmt.Errorf("no target: give a host or a share link")
	}
	if m := shareLinkRe.FindStringSubmatch(arg); m != nil {
		base := strings.TrimRight(m[1], "/")
		if tm := shareTokenRe.FindStringSubmatch(m[2]); tm != nil && tm[1] != "" {
			return Target{URL: base, Token: tm[1], Scope: ScopeGuest}, nil
		}
		if cm := enrollCodeRe.FindStringSubmatch(m[2]); cm != nil && cm[1] != "" {
			return Target{URL: base, EnrollCode: cm[1], Scope: ScopeOwner}, nil
		}
	}
	url := normalizeHost(arg)
	if url == "" {
		return Target{}, fmt.Errorf("bad host: %q", arg)
	}
	if strings.TrimSpace(token) == "" {
		// A host this terminal already paired with authenticates from remotes.json.
		if saved := LoadRemoteToken(url); saved != "" {
			return Target{URL: url, Token: saved, Scope: ScopeOwner}, nil
		}
		return Target{}, fmt.Errorf("connecting to %s needs --token, a pair link (gtmux pair), or a share link", url)
	}
	return Target{URL: url, Token: strings.TrimSpace(token), Scope: ScopeOwner}, nil
}

// normalizeHost turns a bare host into a base URL: default scheme http:// and default
// port :8765, trailing slashes trimmed. Empty for empty input.
func normalizeHost(input string) string {
	h := strings.TrimSpace(input)
	h = strings.TrimRight(h, "/")
	if h == "" {
		return ""
	}
	if !strings.HasPrefix(h, "http://") && !strings.HasPrefix(h, "https://") {
		h = "http://" + h
	}
	hostPart := h
	if i := strings.Index(hostPart, "://"); i >= 0 {
		hostPart = hostPart[i+3:]
	}
	// Add the default port only when the host has none (and isn't a path-only edge).
	if !strings.Contains(hostPart, ":") {
		h += ":8765"
	}
	return h
}
