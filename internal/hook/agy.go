package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/chenchaoyi/gtmux/internal/transcript"
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
// and strips standalone <SYSTEM_MESSAGE> blocks using the shared transcript cleaner.
func cleanAgyPromptText(raw string) string {
	return transcript.CleanAgyUserContent(raw)
}
