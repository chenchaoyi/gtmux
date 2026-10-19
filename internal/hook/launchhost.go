package hook

import (
	"os"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

const launchHostEnv = "GTMUX_HOOK_LAUNCH_HOST"

var observeLaunchHost = launchhost.Observe
var startHookWorker = startDetachedHookWorker

func launchIdentityEvent(event string) bool {
	return event == "SessionStart" || event == "UserPromptSubmit" || event == "Resumed"
}
func hookLaunchHost(agent, event string, detached bool) launchhost.Evidence {
	if !launchIdentityEvent(event) {
		return launchhost.Evidence{}
	}
	if detached {
		return launchhost.Decode(os.Getenv(launchHostEnv), time.Now().Unix())
	}
	m, ok := agents.For(agent)
	if !ok {
		return launchhost.Evidence{}
	}
	commands := append([]string{m.Key}, m.Detect...)
	commands = append(commands, m.InstallDetect...)
	commands = append(commands, m.TerminalClients...)
	if len(m.Resume) > 0 {
		commands = append(commands, m.Resume[0])
	}
	return observeLaunchHost(commands)
}
