package mine

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Antigravity CLI (`agy`) journals a conversation under
// $AGY_HOME/brain/<conversation-id>/.system_generated/logs/transcript.jsonl.
// Each line is a flat step object: USER_EXPLICIT/USER_INPUT carries the user's
// prompt (typically wrapped in <USER_REQUEST>...</USER_REQUEST>), and
// MODEL/PLANNER_RESPONSE carries assistant replies (status "DONE"|"ERROR").
type agyMineRec struct {
	StepIndex int    `json:"step_index"`
	Source    string `json:"source"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	Content   string `json:"content"`
}

func readAgy(path string, start int64, o readOpts) (readResult, int64, error) {
	// .../brain/<conversation-id>/.system_generated/logs/transcript.jsonl
	session := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	n := 0
	return readLog(path, start, o, func(string) bool { return false }, func(line string, c *convo) {
		c.meta(session, "")
		var r agyMineRec
		if json.Unmarshal([]byte(line), &r) != nil {
			return
		}
		n++
		switch r.Type {
		case "USER_INPUT":
			if r.Source != "" && r.Source != "USER_EXPLICIT" {
				return
			}
			text := transcript.CleanAgyUserContent(r.Content)
			if text == "" {
				return
			}
			var ts int64
			if parsed, err := time.Parse(time.RFC3339Nano, r.CreatedAt); err == nil {
				ts = parsed.Unix()
			}
			id := strconv.Itoa(r.StepIndex)
			if r.StepIndex == 0 && n > 1 {
				id = strconv.Itoa(n)
			}
			c.human(text, ts, id)
		case "PLANNER_RESPONSE":
			if r.Source != "" && r.Source != "MODEL" {
				return
			}
			if strings.EqualFold(r.Status, "ERROR") {
				return
			}
			if text := strings.TrimSpace(r.Content); text != "" {
				c.assistant(text)
			}
		}
	})
}
