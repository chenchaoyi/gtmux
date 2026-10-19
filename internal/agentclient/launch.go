package agentclient

import (
	"strings"
)

// Command selects a declared interactive client, rather than launching a Web
// server under a harness's generic CLI name. Other clients add an adapter here.
func Command(agent, client, sessionID string) (string, bool) {
	if !declared(agent, client) {
		return "", false
	}
	var argv []string
	switch client {
	case "dst":
		argv = []string{"dst", "--backend", "dsh"}
	default:
		return "", false
	}
	if sessionID != "" {
		argv = append(argv, "--resume", sessionID)
	}
	for i := range argv {
		argv[i] = "'" + strings.ReplaceAll(argv[i], "'", `'\''`) + "'"
	}
	return strings.Join(argv, " "), true
}
