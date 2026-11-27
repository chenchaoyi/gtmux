package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// agyHookStdout is where agy hook contract JSON responses are written. Replaced in tests.
var agyHookStdout io.Writer = os.Stdout

// writeAgyHookResponse emits the JSON object Antigravity CLI expects on stdout for every
// hook command. Detached workers skip writing because the synchronous front-end already
// answered agy before handing off slow work.
func writeAgyHookResponse(agentKey, rawEvent string, args []string) {
	if agentKey != "agy" || argsHave(args, "--detached") {
		return
	}
	if rawEvent == "Stop" {
		_, _ = fmt.Fprintln(agyHookStdout, `{"decision":"stop"}`)
		return
	}
	_, _ = fmt.Fprintln(agyHookStdout, `{}`)
}

// classifyAgyPayload refines agy lifecycle events using payload fields documented in the
// Antigravity hooks contract:
//   - PreInvocation fires before every model call in a turn; only the opening call
//     (invocationNum <= 1) starts a user turn, while later calls act as Resumed so they
//     do not double-count user prompts or emit duplicate receipts.
//   - Stop carries terminationReason to distinguish normal completion, cancellation, and
//     execution failures.
func classifyAgyPayload(event string, c Class, raw []byte) Class {
	var p struct {
		InvocationNum     int    `json:"invocationNum"`
		TerminationReason string `json:"terminationReason"`
	}
	_ = json.Unmarshal(raw, &p)
	switch event {
	case "PreInvocation":
		if p.InvocationNum > 1 {
			return Class{Lifecycle: "Resumed"}
		}
		return c
	case "Stop":
		switch p.TerminationReason {
		case "USER_CANCELED":
			return Class{Lifecycle: "Interrupt"}
		case "ERROR", "MAX_INVOCATIONS", "MAX_FORCED_INVOCATIONS", "MAX_TOKEN_BUDGET_EXCEEDED":
			return Class{Lifecycle: "StopFailure"}
		default:
			return c
		}
	}
	return c
}

// cleanAgyPromptText unwraps the <USER_REQUEST> envelope agy writes around user input
// while keeping plain prompts untouched.
func cleanAgyPromptText(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	const openTag = "<USER_REQUEST>"
	const closeTag = "</USER_REQUEST>"
	if start := strings.Index(s, openTag); start >= 0 {
		rest := s[start+len(openTag):]
		if end := strings.Index(rest, closeTag); end >= 0 {
			return strings.TrimSpace(rest[:end])
		}
		return strings.TrimSpace(rest)
	}
	for _, marker := range []string{"<ADDITIONAL_METADATA>", "<USER_SETTINGS_CHANGE>"} {
		if idx := strings.Index(s, marker); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	return s
}
