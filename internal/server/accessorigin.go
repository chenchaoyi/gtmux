package server

import (
	"net"
	"net/http"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// Identity comes from authentication, not client-supplied display/actor headers.
// A paired browser or phone stays a client even behind a headerless local proxy.
func accessOrigin(r *http.Request) (origin, basis string) {
	if d, ok := r.Context().Value(accessDeviceCtxKey).(EnrolledDevice); ok && d.ID != "" {
		return diag.AccessExternal, "paired_client"
	}
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	ip := net.ParseIP(peer)
	if isRemoteAccessIP(ip) {
		return diag.AccessExternal, "network_peer"
	}
	if ip != nil && ip.IsLoopback() {
		// clientIP already restricts forwarded-address trust to the local proxy hop.
		if forwarded := net.ParseIP(clientIP(r)); isRemoteAccessIP(forwarded) {
			return diag.AccessExternal, "forwarded_peer"
		}
		for _, header := range []string{"X-Forwarded-For", "Forwarded", "Cf-Ray", "Cf-Connecting-IP", "X-Forwarded-Proto"} {
			if r.Header.Get(header) != "" {
				return diag.AccessUnknown, "unresolved"
			}
		}
		// A loopback master credential alone is ambiguous. Only a recognized,
		// authenticated tool declaration establishes this narrower local class.
		actor := actorOf(r.Context())
		if callerScope(r.Context()) == scopeMaster && (actor == "menubar" || actor == "system" || actor == "update" || actor == "hq" || actor == "user" || strings.HasPrefix(actor, "agent:%")) {
			return diag.AccessLocal, "local_tool"
		}
	}
	return diag.AccessUnknown, "unresolved"
}

func isRemoteAccessIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsMulticast()
}
