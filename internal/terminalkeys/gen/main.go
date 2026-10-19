// Generate the demo catalog from the same encoder used by the server.
package main

import (
	"encoding/json"
	"flag"
	"os"

	"github.com/chenchaoyi/gtmux/internal/terminalkeys"
)

func main() {
	out := flag.String("out", "", "output path")
	flag.Parse()
	b, err := json.Marshal(struct {
		Keys   []terminalkeys.Key            `json:"keys"`
		Legacy map[string]terminalkeys.Event `json:"legacy_events"`
	}{terminalkeys.Catalog(), terminalkeys.LegacyEvents()})
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		panic(err)
	}
}
