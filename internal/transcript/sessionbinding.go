package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// Neighbour is another conversation beside a bound log: its session id and log path.
type Neighbour struct {
	ID, Path string
}

// Neighbours lists the conversations beside a bound log that changed at or after
// since (unix seconds; a cheap stat filter before anything is parsed), with the session
// id each one actually has. "Beside" is the agent's own layout:
//
//   - Codex keeps every project's rollouts together in date directories, named
//     rollout-<time>-<id>.jsonl. Its neighbours are the rollouts whose session_meta
//     names the bound log's working directory, and the id is session_meta's, not the
//     file name. Taking the file name for the id found nothing for Codex (%12,
//     2026-10-06), and looking only in the bound log's own directory would have
//     counted other projects' sessions as neighbours.
//   - Every other agent: the logs with the same extension in the bound log's
//     directory (Claude's per-project folder), named <id><ext>.
func Neighbours(agent, boundPath string, since int64) []Neighbour {
	var out []Neighbour
	if normalizeAgent(agent) == "codex" {
		_, cwd := codexSessionMeta(boundPath)
		if cwd == "" {
			return nil // no working directory to be beside
		}
		files, _ := filepath.Glob(filepath.Join(codexHome(), "sessions", "*", "*", "*", "rollout-*.jsonl"))
		for _, f := range files {
			if f == boundPath || !changedSince(f, since) {
				continue
			}
			if id, fcwd := codexSessionMeta(f); id != "" && fcwd == cwd {
				out = append(out, Neighbour{ID: id, Path: f})
			}
		}
		return out
	}
	dir, ext := filepath.Dir(boundPath), filepath.Ext(boundPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ext {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if path == boundPath || !changedSince(path, since) {
			continue
		}
		out = append(out, Neighbour{ID: strings.TrimSuffix(e.Name(), ext), Path: path})
	}
	return out
}

func changedSince(path string, since int64) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.ModTime().Unix() >= since
}
