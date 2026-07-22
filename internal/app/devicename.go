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
func deviceDisplayName(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(s), "gtmux") {
		return s
	}
	cleaned := strings.Trim(strings.TrimPrefix(s[:0]+s[len("gtmux"):], ""), " •·\t")
	if cleaned == "" {
		return s
	}
	return cleaned
}
