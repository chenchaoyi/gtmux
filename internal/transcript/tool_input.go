package transcript

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const (
	maxToolInputBytes      = 8 << 10
	maxTurnToolDetailBytes = 128 << 10
)

// toolStep adds inspectable input without retaining results or dumping arbitrary JSON.
// Summaries remain compatible with older clients; richer fields are optional.
func toolStep(name, title string, raw json.RawMessage, summary string) Step {
	step := Step{Kind: "tool", Title: title, Detail: summary, Operation: toolOperation(name)}
	var args map[string]any
	if json.Unmarshal(raw, &args) != nil {
		return step
	}
	for _, field := range []struct{ key, kind string }{
		{"cmd", "command"}, {"command", "command"}, {"code", "code"}, {"script", "code"},
		{"file_path", "path"}, {"path", "path"}, {"pattern", "query"}, {"query", "query"},
		{"url", "url"}, {"description", "description"}, {"title", "description"}, {"prompt", "prompt"},
	} {
		value, ok := args[field.key].(string)
		kind := field.kind
		if !ok && kind == "command" {
			if values, array := args[field.key].([]any); array && len(values) > 0 {
				valid := true
				for _, value := range values {
					if _, ok := value.(string); !ok {
						valid = false
						break
					}
				}
				if valid {
					encoded, _ := json.Marshal(values)
					value, kind = string(encoded), "arguments"
				}
			}
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		step.Input, step.DetailsTruncated = boundedToolText(value, maxToolInputBytes)
		step.InputKind = kind
		break
	}
	for _, key := range []string{"workdir", "cwd"} {
		if value, ok := args[key].(string); ok && strings.TrimSpace(value) != "" {
			var cut bool
			step.Workdir, cut = boundedToolText(value, 4096)
			step.DetailsTruncated = step.DetailsTruncated || cut
			break
		}
	}
	return step
}

func boundedToolText(value string, limit int) (string, bool) {
	if len(value) <= limit {
		return strings.Clone(value), false
	}
	if limit <= 0 {
		return "", value != ""
	}
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return strings.Clone(value[:limit]), true
}

func toolOperation(name string) string {
	if i := strings.LastIndex(name, "__"); i >= 0 {
		name = name[i+2:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	switch strings.ToLower(name) {
	case "read", "readfile", "read_file", "read_text_file":
		return "read"
	case "ls", "list", "list_files", "list_directory", "glob":
		return "list"
	case "grep", "rg", "search", "search_query", "find", "find_files":
		return "search"
	case "bash", "shell", "exec", "exec_command", "local_shell", "run_command":
		return "command"
	case "edit", "write", "apply_patch", "patch", "write_file", "edit_file":
		return "edit"
	case "web_search", "webfetch", "web_fetch", "browse", "open_url", "fetch":
		return "web"
	case "js", "python":
		return "code"
	case "wait", "wait_for", "waitfor", "sleep":
		return "wait"
	case "agent", "task", "spawn_agent":
		return "agent"
	default:
		return "tool"
	}
}
