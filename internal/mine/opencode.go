package mine

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
)

// opencode keeps no readable log of its own; gtmux writes one for it (octrans/, see
// internal/transcript/opencode.go): {timestamp, role, text} per line, one file per
// session id. It carries no tool calls or results and no cwd, so this reader yields
// correction leads only, with no project. A line has no id either; the timestamp plus a
// hash of the text stands in — stable across passes, unique in practice.
type opencodeRec struct {
	Timestamp string `json:"timestamp"`
	Role      string `json:"role"`
	Text      string `json:"text"`
}

func readOpencode(path string, start int64, o readOpts) (readResult, int64, error) {
	session := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	return readLog(path, start, o, func(string) bool { return false }, func(line string, c *convo) {
		c.meta(session, "")
		var r opencodeRec
		if json.Unmarshal([]byte(line), &r) != nil {
			return
		}
		switch r.Role {
		case "assistant":
			c.assistant(r.Text)
		case "user":
			h := sha1.Sum([]byte(r.Text))
			c.human(r.Text, parseTS(r.Timestamp), r.Timestamp+"/"+hex.EncodeToString(h[:6]))
		}
	})
}
