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
// token was minted for (confirmed later by GET /api/share).
type Target struct {
	URL   string
	Token string
	Scope Scope
}

var shareLinkRe = regexp.MustCompile(`^(https?://[^#]+?)/*#(.*)$`)
var shareTokenRe = regexp.MustCompile(`(?:^|[?&])t=([^&]+)`)

// ParseTarget resolves a `gtmux connect` argument into a Target. A guest share link
// (`https://host/#t=<token>`, what `gtmux share new` mints) yields a GUEST target
// carrying that token. Otherwise the argument is a host and `token` (from --token) is
// the OWNER bearer; the host is normalized (http:// + :8765 defaults) like the mobile
// app. An owner target with no token is an error.
func ParseTarget(arg, token string) (Target, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return Target{}, fmt.Errorf("no target: give a host or a share link")
	}
	if m := shareLinkRe.FindStringSubmatch(arg); m != nil {
		if tm := shareTokenRe.FindStringSubmatch(m[2]); tm != nil && tm[1] != "" {
			return Target{URL: strings.TrimRight(m[1], "/"), Token: tm[1], Scope: ScopeGuest}, nil
		}
	}
	url := normalizeHost(arg)
	if url == "" {
		return Target{}, fmt.Errorf("bad host: %q", arg)
	}
	if strings.TrimSpace(token) == "" {
		return Target{}, fmt.Errorf("connecting to %s needs --token (or pass a share link)", url)
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
