package diag

import "net"

const (
	AccessExternal = "external"
	AccessLocal    = "local"
	AccessUnknown  = "unknown"
	AccessAll      = "all"
)

func validAccessOrigin(origin string) bool {
	return origin == AccessExternal || origin == AccessLocal || origin == AccessUnknown
}

func accessOriginBasis(basis string) string {
	switch basis {
	case "paired_client", "network_peer", "forwarded_peer", "local_tool", "unresolved", "legacy_device", "legacy_remote", "legacy_unknown":
		return basis
	}
	return "unresolved"
}

// Historical interpretation is read-only and needs positive recorded evidence.
// Credential scope, actor labels and loopback addresses cannot identify a client.
func restoreAccessOrigin(e AccessEntry) AccessEntry {
	if e.Origin != "" {
		if !validAccessOrigin(e.Origin) {
			e.Origin, e.OriginBasis = AccessUnknown, "unresolved"
		}
		return e
	}
	e.Origin, e.OriginBasis = AccessUnknown, "legacy_unknown"
	if e.DeviceID != "" {
		e.Origin, e.OriginBasis = AccessExternal, "legacy_device"
	} else if ip := net.ParseIP(e.SourceIP); remoteAccessIP(ip) {
		e.Origin, e.OriginBasis = AccessExternal, "legacy_remote"
	}
	return e
}

func remoteAccessIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsMulticast()
}
