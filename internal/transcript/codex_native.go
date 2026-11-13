package transcript

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func codexNativeType(kind string) bool {
	switch kind {
	case "CommandExecution", "FileChange", "McpToolCall", "Extension":
		return true
	}
	return false
}

// Inspect structural headers only. Reasoning, message mirrors and tool results
// remain excluded even when their text mentions a supported action type.
func codexNativeHeader(d *json.Decoder) bool {
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return true
		}
		if key != "item" {
			var discard json.RawMessage
			if d.Decode(&discard) != nil {
				return true
			}
			continue
		}
		if token, err := d.Token(); err != nil || token != json.Delim('{') {
			return true
		}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return true
			}
			if key == "type" {
				var kind string
				if d.Decode(&kind) != nil {
					return true
				}
				return codexNativeType(kind)
			}
			var discard json.RawMessage
			if d.Decode(&discard) != nil {
				return true
			}
		}
		return false
	}
	return false
}

type codexNativeItem struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Command   json.RawMessage `json:"command"`
	Cwd       string          `json:"cwd"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Kind      string          `json:"kind"`
	Query     string          `json:"query"`
	Status    string          `json:"status"`
	AppName   string          `json:"appName"`
	Result    recordedValue   `json:"result"`
	Output    recordedValue   `json:"aggregated_output"`
	Duration  struct {
		Secs  int64 `json:"secs"`
		Nanos int64 `json:"nanos"`
	} `json:"duration"`
	ParsedCmd []struct {
		Type string `json:"type"`
		Path string `json:"path"`
		Name string `json:"name"`
	} `json:"parsed_cmd"`
	Changes map[string]struct {
		Type        string `json:"type"`
		Content     string `json:"content,omitempty"`
		UnifiedDiff string `json:"unified_diff,omitempty"`
		MovePath    string `json:"move_path,omitempty"`
	} `json:"changes"`
}

// Polls retain selected input and bounded metadata only. Large result/output values
// are fingerprinted without a retained copy; only the scoped full reader captures them.

func codexNativeEvent(line string, st *parseState) bool {
	if !strings.Contains(line, `"item_completed"`) {
		return false
	}
	var record struct {
		Type    string `json:"type"`
		Payload struct {
			Type string          `json:"type"`
			Item codexNativeItem `json:"item"`
		} `json:"payload"`
	}
	record.Payload.Item.Result.capture = st.fullTools
	record.Payload.Item.Output.capture = st.fullTools
	if json.Unmarshal([]byte(line), &record) != nil || record.Type != "event_msg" || record.Payload.Type != "item_completed" {
		return false
	}
	item := record.Payload.Item
	if !codexNativeType(item.Type) {
		return true
	}
	var step Step
	switch item.Type {
	case "CommandExecution":
		args, _ := json.Marshal(map[string]any{"command": item.Command, "cwd": item.Cwd})
		step = toolStep("exec_command", "exec", args, "")
		if step.Input == "" {
			return true
		}
		operations := make(map[string]bool)
		unknown := false
		var paths []string
		for _, parsed := range item.ParsedCmd {
			var op string
			switch parsed.Type {
			case "read":
				op = "read"
			case "list_files":
				op = "list"
			case "search":
				op = "search"
			default:
				unknown = true
			}
			if op != "" {
				operations[op] = true
				if parsed.Path != "" {
					paths = append(paths, parsed.Path)
				} else if parsed.Name != "" {
					paths = append(paths, parsed.Name)
				}
			}
		}
		if len(operations) == 1 && !unknown {
			for op := range operations {
				step.Operation = op
			}
		}
		step.Detail = recordedPathSummary(paths)
		if step.Operation == "read" && len(paths) == len(item.ParsedCmd) {
			readTargetFacts(&step, paths)
		}
		if step.Detail == "" {
			step.Detail = clip(strings.Join(strings.Fields(step.Input), " "), 80)
		}
	case "FileChange":
		if len(item.Changes) == 0 {
			return true
		}
		var paths []string
		for path := range item.Changes {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		input, _ := json.MarshalIndent(map[string]any{"changes": item.Changes, "status": item.Status}, "", "  ")
		step = Step{Kind: "tool", Title: "apply_patch", Operation: "edit", InputKind: "changes", Detail: recordedPathSummary(paths), fullInput: string(input)}
		step.Input, step.DetailsTruncated = boundedToolText(step.fullInput, maxToolInputBytes)
	case "McpToolCall":
		if item.Tool == "" {
			return true
		}
		step = toolStep(item.Tool, codexToolName(item.Tool), item.Arguments, "")
	case "Extension":
		if item.Kind == "web.search" && item.Query != "" {
			args, _ := json.Marshal(map[string]string{"query": item.Query})
			step = toolStep("web_search", "web_search", args, clip(item.Query, 80))
		} else {
			// Known clock.sleep mirrors the already recorded function call. Its
			// elapsed duration is not a requested input; unknown extensions stay out.
			return true
		}
	}
	if item.Type == "McpToolCall" || item.Type == "CommandExecution" {
		args, result := item.Arguments, item.Result
		if item.Type == "CommandExecution" {
			args, _ = json.Marshal(map[string]any{"command": item.Command, "cwd": item.Cwd})
			result = item.Output
		}
		step.Call, step.detailIdentity = recordedCall(item.AppName, item.Tool, item.Status, item.Duration.Secs, item.Duration.Nanos, args, result, st.fullTools)
	}
	st.addRecordedStep("codex", step, item.ID)
	return true
}

// Keep filenames visible in a compact row; complete paths remain in recorded input.
func recordedPathSummary(paths []string) string {
	names := make(map[string]int)
	for _, path := range paths {
		names[filepath.Base(path)]++
	}
	shown := make([]string, 0, 3)
	for _, path := range paths {
		name := filepath.Base(path)
		if names[name] > 1 {
			name = filepath.Join(filepath.Base(filepath.Dir(path)), name)
		}
		shown = append(shown, name)
		if len(shown) == 3 {
			break
		}
	}
	result := strings.Join(shown, ", ")
	if len(paths) > len(shown) {
		result += " +" + strconv.Itoa(len(paths)-len(shown))
	}
	return clip(result, 80)
}
