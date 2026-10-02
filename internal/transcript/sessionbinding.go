package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"regexp"
)

const SessionBindingPrefix = "[gtmux session binding] "

var sessionBindingLine = regexp.MustCompile(`^\[gtmux session binding\] ([0-9a-f]{32})$`)
var sessionBindingTail = regexp.MustCompile(`(?:^|\n)\[gtmux session binding\] ([0-9a-f]{32})$`)

// SessionBindingToken recognizes only a complete reserved trailing metadata line.
func SessionBindingToken(prompt string) string {
	m := sessionBindingTail.FindStringSubmatch(prompt)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// CodexBindingPrompts reads only submitted user messages, never assistant echoes
// or tool output. The bounded tail also covers a late retry in an existing log.
func CodexBindingPrompts(sid string) []string {
	const limit = 8 << 20
	var prompts []string
	for _, path := range codexLogPaths(sid) {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}
		if info.Size() > limit {
			_, _ = f.Seek(info.Size()-limit, io.SeekStart)
		}
		scanner := bufio.NewScanner(io.LimitReader(f, limit))
		scanner.Buffer(make([]byte, 4096), limit)
		for scanner.Scan() {
			var line codexLine
			var payload codexPayload
			if json.Unmarshal(scanner.Bytes(), &line) != nil || json.Unmarshal(line.Payload, &payload) != nil {
				continue
			}
			var prompt string
			if line.Type == "event_msg" && payload.Type == "user_message" {
				prompt = payload.Message
			} else if line.Type == "response_item" && payload.Type == "message" && payload.Role == "user" {
				for _, part := range payload.Content {
					if part.Type == "input_text" {
						prompt += part.Text
					}
				}
			}
			if SessionBindingToken(prompt) != "" {
				prompts = append(prompts, prompt)
			}
		}
		f.Close()
	}
	return prompts
}
