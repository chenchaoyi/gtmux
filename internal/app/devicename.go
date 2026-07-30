package app

import "strings"

// deviceDisplayName cleans a pair-roster entry for display (device-roster-naming).
//
// The phone app used to register itself as "gtmux • iPhone". Inside gtmux's OWN roster
// the "gtmux" prefix carried no information — nothing in that list is not a gtmux device
// — while pushing the part that identifies the device rightward into truncation. New
// pairings no longer send it; stripping it here means the entries already on disk read
// correctly without asking anyone to re-pair.
//
// A device legitimately NAMED "gtmux" keeps its name rather than becoming a blank row.
// Finally a bare generic kind ("browser") is title-cased so the roster doesn't read as
// an unpolished lowercase word — while a person's chosen name ("ccy") is left alone.
func deviceDisplayName(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(s), "gtmux") {
		if cleaned := strings.Trim(s[len("gtmux"):], " •·\t"); cleaned != "" {
			s = cleaned
		}
	}
	return prettifyGenericDeviceName(s)
}

// prettifyGenericDeviceName title-cases the handful of generic auto-assigned kind names
// (a browser enrolls as the literal "browser") so they read as proper labels; anything
// else — a user's own device name — passes through verbatim.
func prettifyGenericDeviceName(s string) string {
	switch strings.ToLower(s) {
	case "browser":
		return "Browser"
	case "terminal":
		return "Terminal"
	}
	return s
}
