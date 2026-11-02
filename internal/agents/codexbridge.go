package agents

import "strings"

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
