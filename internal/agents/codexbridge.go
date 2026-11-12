package agents

import (
	"encoding/json"
	"strings"
)

// CodexBridgeEligible describes the interactive argv the managed frontend can
// supervise. Launch and saved-command detection share it so restore recognizes
// the frontend it constructs. It grants no live input or session ownership.
func CodexBridgeEligible(args []string) bool {
	positionals := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return true
		}
		key, _, inline := strings.Cut(a, "=")
		switch key {
		case "--model", "-m", "--sandbox", "-s", "--ask-for-approval", "-a", "--cd", "-C", "--add-dir", "--image", "-i":
			if !inline {
				i++
				if i >= len(args) {
					return false
				}
			}
		case "-c", "--config":
			value := ""
			if inline {
				_, value, _ = strings.Cut(a, "=")
			} else {
				i++
				if i >= len(args) {
					return false
				}
				value = args[i]
			}
			if !CodexPresentationOverride(value) {
				return false
			}
		case "--no-alt-screen", "--full-auto", "--approve-for-me", "--search", "--last", "--all":
		case "resume", "fork":
			if positionals > 0 {
				return false
			}
			positionals++
		default:
			if strings.HasPrefix(a, "-") {
				return false
			}
			if positionals == 0 {
				switch a {
				case "exec", "e", "app-server", "mcp-server", "mcp", "login", "logout", "review", "completion", "cloud", "debug", "features", "apply", "a", "help", "queue", "unarchive", "sandbox", "tcp-tunnel", "execpolicy", "responses-api-proxy", "cloud-tasks", "agents", "plugin", "update", "remote-control", "doctor":
					return false
				}
			}
			positionals++
		}
	}
	return true
}

// CodexPresentationOverride admits one display-only setting without admitting
// provider, home, endpoint, profile or permission changes. A complete JSON value
// is also a valid TOML boolean/string array; trailing assignments are rejected.
func CodexPresentationOverride(value string) bool {
	if len(value) > 4096 {
		return false
	}
	key, raw, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(key) != "tui.notifications" {
		return false
	}
	raw = strings.TrimSpace(raw)
	if raw == "true" || raw == "false" {
		return true
	}
	if !strings.HasPrefix(raw, "[") {
		return false
	}
	var names []any
	if json.Unmarshal([]byte(raw), &names) != nil {
		return false
	}
	for _, name := range names {
		if _, ok := name.(string); !ok {
			return false
		}
	}
	return true
}
