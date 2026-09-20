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
// token was minted for (confirmed later by GET /api/share). EnrollCode is set when the
// target is a PAIR link (`#c=<code>`, pair-share-model S2) or a share link in its short
// form (`#code=<code>`, share-link-is-the-code): the code is redeemed for the token this
// terminal then keeps, so the next connection is just `gtmux attach <host>`.
type Target struct {
	URL        string
	Token      string
	Scope      Scope
	EnrollCode string
}

var shareLinkRe = regexp.MustCompile(`^(https?://[^#]+?)/*#(.*)$`)

// `#code=` is what a share link looks like now: the short code IS the link, and the
// same word is what the web page reads, so the two surfaces can be described in one
// sentence. `#g=` (and the older `#t=`) carried the raw 64-character token; both are
// still accepted, so every link already handed out keeps working.
var shareCodeRe = regexp.MustCompile(`(?:^|[?&])code=([^&]+)`)
var shareTokenRe = regexp.MustCompile(`(?:^|[?&])[gt]=([^&]+)`)
var enrollCodeRe = regexp.MustCompile(`(?:^|[?&])c=([^&]+)`)

// ParseTarget resolves a `gtmux connect` argument into a Target. A guest share link
// (`https://host#code=<code>`, what `gtmux share new` mints; the older `#g=<token>` and
// `#t=` forms are still accepted) yields a GUEST target. A PAIR link (`https://host/#c=<code>`, what `gtmux pair`
// prints) yields an OWNER target carrying the enroll code — redeemed once for a
// persisted device token. Otherwise the argument is a host: `token` (from --token)
// or a previously-persisted remote token (remotes.json) is the OWNER bearer; the
// host is normalized (http:// + :8765 defaults) like the mobile app. An owner
// target with no credential is an error.
func ParseTarget(arg, token string, code ...string) (Target, error) {
	var codeArg string
	if len(code) > 0 {
		codeArg = strings.TrimSpace(code[0])
	}
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return Target{}, fmt.Errorf("no target: give a host or a share link")
	}
	if m := shareLinkRe.FindStringSubmatch(arg); m != nil {
		base := strings.TrimRight(m[1], "/")
		if cm := shareCodeRe.FindStringSubmatch(m[2]); cm != nil && cm[1] != "" {
			return Target{URL: base, EnrollCode: cm[1], Scope: ScopeGuest}, nil
		}
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
		if codeArg != "" {
			// A short code someone read out (share-one-time-code): the caller redeems it
			// against this host and keeps what comes back, so the URL is all that is
			// needed here.
			return Target{URL: url, EnrollCode: codeArg, Scope: ScopeGuest}, nil
		}
		return Target{}, fmt.Errorf("connecting to %s needs --token, a code (--code), a pair link (gtmux pair), or a share link", url)
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
	scheme := "http://"
	if strings.HasPrefix(h, "https://") {
		scheme = "https://"
	}
	rest := h[len(scheme):]
	// The default port belongs to the HOST — never after a path. A tunnel target is
	// path-based (https://tunnel.example/pNNNNN), and blindly appending ":8765" to the
	// whole string produced "https://tunnel.example/pNNNNN:8765", which reaches nothing.
	host, path := rest, ""
	if i := strings.Index(rest, "/"); i >= 0 {
		host, path = rest[:i], rest[i:]
	}
	// Only a bare http host gets the serve default; an https endpoint (a tunnel behind
	// TLS) is on 443, and a host that already names a port is left alone.
	if scheme == "http://" && !strings.Contains(host, ":") {
		host += ":8765"
	}
	return scheme + host + path
}
