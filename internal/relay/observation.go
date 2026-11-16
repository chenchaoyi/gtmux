package relay

import (
	"encoding/json"
	"io"
	"strings"
)

// IsObservationExport recognizes a complete, nonempty relay-list export. It
// never returns records or bodies to a caller. The caller bounds the reader and
// verifies file identity; unknown, partial or ordinary structured notes stay data.
func IsObservationExport(input io.Reader) bool {
	d := json.NewDecoder(input)
	token, err := d.Token()
	if err != nil || token != json.Delim('[') {
		return false
	}
	count := 0
	for d.More() {
		if count >= 10000 {
			return false
		}
		var r Request
		if d.Decode(&r) != nil || !validID(r.ID) || r.Kind != "report" && r.Kind != "ask" ||
			strings.TrimSpace(r.Body) == "" || r.Session == "" || r.CreatedAt <= 0 ||
			r.For != "hq" && r.For != "user" {
			return false
		}
		switch r.State {
		case "pending", "claimed", "replied", "closed":
		default:
			return false
		}
		count++
	}
	if token, err = d.Token(); err != nil || token != json.Delim(']') || count == 0 {
		return false
	}
	var extra any
	return d.Decode(&extra) == io.EOF
}
