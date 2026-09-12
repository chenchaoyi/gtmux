// Package humanize renders quantities the way a person reads them. It is a leaf: no
// gtmux package below it, so the knowledge base, the supervisor and doctor can all say
// "3d ago" the same way without one importing the other.
package humanize

import (
	"strconv"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// AgeShort renders an age in seconds as a terse "3d" / "40h" / "12m" / "just now".
// Maintenance ages span minutes to weeks, so a single coarse unit is the readable choice —
// but hours run to 48, not 24: the daily self-check's interesting window is "a bit over a
// day", and rounding 40h down to "1d" made a flagged row read as if it were on time.
func AgeShort(secs int64) string {
	switch {
	case secs < 60:
		return i18n.Tr("just now", "刚刚")
	case secs < 3600:
		return strconv.FormatInt(secs/60, 10) + "m"
	case secs < 48*3600:
		return strconv.FormatInt(secs/3600, 10) + "h"
	default:
		return strconv.FormatInt(secs/(24*3600), 10) + "d"
	}
}
