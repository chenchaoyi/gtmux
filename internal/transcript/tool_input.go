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
	return providerToolStep("", name, title, raw, summary)
}

// Only adapters with a known native schema may interpret provider-only aliases.
func providerToolStep(provider, name, title string, raw json.RawMessage, summary string) Step {
	step := Step{Kind: "tool", Title: title, Detail: summary, Operation: toolOperation(name)}
	var args map[string]any
	if json.Unmarshal(raw, &args) != nil {
		return step
	}
	for _, field := range []struct{ key, kind string }{
		{"cmd", "command"}, {"command", "command"}, {"code", "code"}, {"script", "code"},
		{"file_path", "path"}, {"filePath", "path"}, {"path", "path"}, {"pattern", "query"}, {"query", "query"},
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
		step.fullInput = value
		step.Input, step.DetailsTruncated = boundedToolText(value, maxToolInputBytes)
		step.InputKind = kind
		break
	}
	for _, key := range []string{"workdir", "cwd"} {
		if value, ok := args[key].(string); ok && strings.TrimSpace(value) != "" {
			var cut bool
			step.fullWorkdir = value
			step.Workdir, cut = boundedToolText(value, 4096)
			step.DetailsTruncated = step.DetailsTruncated || cut
			break
		}
	}
	if parameters, limited := toolParameters(step.Operation, args); parameters != "" {
		step.fullInput, _ = selectedToolParameters(step.Operation, args, int(^uint(0)>>1))
		var cut bool
		step.Input, cut = boundedToolText(parameters, maxToolInputBytes)
		step.InputKind = "parameters"
		step.DetailsTruncated = step.DetailsTruncated || cut || limited
		if step.Detail == "" {
			for _, key := range []string{"description", "title", "task_name", "message", "prompt"} {
				if value, ok := args[key].(string); ok && strings.TrimSpace(value) != "" {
					prefix, _ := boundedToolText(value, maxToolInputBytes)
					step.Detail = clip(strings.Join(strings.Fields(prefix), " "), 80)
					break
				}
			}
		}
	}
	applyActivityFacts(&step, name, args, provider)
	return step
}

// Keep known task/wait inputs inspectable without exposing unrelated credentials,
// nested configuration or tool results. Every adapter shares the same byte budgets.
func toolParameters(operation string, args map[string]any) (string, bool) {
	return selectedToolParameters(operation, args, maxToolInputBytes)
}
func selectedToolParameters(operation string, args map[string]any, limit int) (string, bool) {
	var keys []string
	switch operation {
	case "edit":
		// Retain only recorded edit arguments, never arbitrary tool configuration.
		if args["content"] == nil && args["old_string"] == nil && args["new_string"] == nil && args["patchText"] == nil && args["oldText"] == nil && args["newText"] == nil && args["oldString"] == nil && args["newString"] == nil {
			return "", false
		}
		keys = []string{"path", "file_path", "content", "old_string", "new_string", "oldText", "newText", "filePath", "oldString", "newString", "replace_all", "patchText"}
	case "agent":
		keys = []string{"task_name", "description", "title", "message", "prompt", "agent_type", "subagent_type", "target", "agent_id", "resume", "model", "reasoning_effort", "fork_turns", "run_in_background"}
	case "wait":
		keys = []string{"cell_id", "ids", "agent_id", "task_id", "duration_ms", "timeout_ms", "timeout", "yield_time_ms", "max_tokens", "terminate", "block"}
	default:
		return "", false
	}
	selected := make(map[string]any)
	remaining, truncated := limit, false
	text := func(value string) string {
		bounded, cut := boundedToolText(value, remaining)
		remaining -= len(bounded)
		truncated = truncated || cut
		return bounded
	}
	for _, key := range keys {
		value := args[key]
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" || operation == "edit" {
				selected[key] = text(v)
			}
		case bool, float64:
			selected[key] = v
		case []any:
			valid := true
			for _, item := range v {
				if _, ok := item.(string); !ok {
					valid = false
					break
				}
			}
			if valid {
				values := make([]string, 0)
				for i, item := range v {
					if remaining == 0 || (limit == maxToolInputBytes && i >= 128) {
						truncated = true
						break
					}
					values = append(values, text(item.(string)))
				}
				selected[key] = values
			}
		}
	}
	if len(selected) == 0 {
		return "", false
	}
	encoded, _ := json.MarshalIndent(selected, "", "  ")
	return string(encoded), truncated
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
	if webRunner(name) {
		return "web"
	}
	if i := strings.LastIndex(name, "__"); i >= 0 {
		name = name[i+2:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	switch strings.ToLower(name) {
	case "read", "readfile", "read_file", "read_text_file", "github_fetch_file", "github_fetch_blob":
		return "read"
	case "ls", "list", "list_files", "list_directory", "glob":
		return "list"
	case "grep", "rg", "search", "search_query", "find", "find_files":
		return "search"
	case "bash", "shell", "exec", "exec_command", "local_shell", "run_command":
		return "command"
	case "edit", "write", "apply_patch", "patch", "write_file", "edit_file":
		return "edit"
	case "web_search", "websearch", "webfetch", "web_fetch", "browse", "open_url", "fetch", "fetchurl":
		return "web"
	case "js", "python":
		return "code"
	case "wait", "wait_for", "waitfor", "sleep", "wait_agent", "taskoutput":
		return "wait"
	case "agent", "agentswarm", "subagent", "task", "spawn_agent", "delegate", "followup_task", "resume_agent":
		return "agent"
	default:
		return "tool"
	}
}
