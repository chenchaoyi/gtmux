package transcript

import (
	"math"
	"path/filepath"
	"strings"
)

func webRunner(name string) bool { return name == "web.run" || name == "web__run" }

func activityDetail(value string) string { return strings.Clone(clip(value, 80)) }

func activityText(value any) string {
	v, ok := value.(string)
	if !ok || strings.TrimSpace(v) == "" {
		return ""
	}
	return v
}

// A recorded canonical skill path identifies reading its instructions, never
// applying/executing the skill. No filesystem or shell-text lookup is involved.
func recordedSkill(path string) string {
	if path == "" || filepath.Clean(path) != path || filepath.Base(path) != "SKILL.md" {
		return ""
	}
	dir := filepath.Dir(path)
	if filepath.Base(filepath.Dir(dir)) != "skills" {
		return ""
	}
	name := filepath.Base(dir)
	if name == "." || name == ".." || strings.ContainsAny(name, "\r\n\x00") {
		return ""
	}
	return name
}

func readTargetFacts(step *Step, paths []string) {
	if len(paths) == 0 || len(paths) > 10000 {
		return
	}
	unique := make(map[string]bool)
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return
		}
		unique[path] = true
	}
	step.SubjectCount = len(unique)
	if len(unique) == 1 {
		for path := range unique {
			if skill := recordedSkill(path); skill != "" {
				step.Action, step.Detail = "skill", activityDetail(skill)
			}
		}
	}
}

func applyActivityFacts(step *Step, name string, args map[string]any) {
	method := name
	if i := strings.LastIndex(method, "__"); i >= 0 {
		method = method[i+2:]
	}
	if i := strings.LastIndex(method, "."); i >= 0 {
		method = method[i+1:]
	}
	method = strings.ToLower(method)
	if method == "skill" {
		if skill := activityText(args["skill"]); skill != "" {
			step.Action, step.Detail, step.SubjectCount = "skill", activityDetail(skill), 1
		}
	}
	if step.Operation == "read" && step.InputKind == "path" {
		readTargetFacts(step, []string{step.fullInput})
	}
	switch method {
	case "websearch", "web_search":
		step.Action = "web_search"
		if value := activityText(args["query"]); value != "" {
			step.Detail = activityDetail(value)
		}
	case "webfetch", "web_fetch", "open_url", "fetchurl":
		step.Action = "web_open"
		if value := activityText(args["url"]); value != "" {
			step.Detail = activityDetail(value)
		}
	}
	if !webRunner(name) {
		return
	}
	// Classify only complete, single-action calls from the known web wrapper.
	// Mixed/foreign actions remain neutral, with original inputs untouched.
	action, details := "", []string{}
	for key, raw := range args {
		if key == "response_length" {
			v := activityText(raw)
			if v != "short" && v != "medium" && v != "long" {
				return
			}
			continue
		}
		kind := map[string]string{"search_query": "web_search", "open": "web_open", "click": "web_open", "find": "web_find"}[key]
		if kind == "" || action != "" {
			return
		}
		values, ok := raw.([]any)
		if !ok || len(values) == 0 || len(values) > 64 {
			return
		}
		for _, rawValue := range values {
			v, ok := rawValue.(map[string]any)
			if !ok {
				return
			}
			field := "ref_id"
			if key == "search_query" {
				field = "q"
			}
			text := activityText(v[field])
			if text == "" {
				return
			}
			if key == "find" {
				text = activityText(v["pattern"])
				if text == "" {
					return
				}
			}
			if key == "click" {
				id, valid := v["id"].(float64)
				if !valid || id < 0 || id != math.Trunc(id) || id > 1<<53-1 {
					return
				}
			}
			if len(details) < 3 {
				details = append(details, activityDetail(text))
			}
		}
		action = kind
	}
	if action != "" {
		step.Action, step.Detail = action, activityDetail(strings.Join(details, ", "))
	}
}
