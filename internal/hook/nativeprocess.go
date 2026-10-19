package hook

import (
	"os"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

const agentProcessEnv = "GTMUX_HOOK_AGENT_PROCESS"

var observeAgentProcess = launchhost.ObserveAgent

func hookAgentProcess(agent string, detached bool) launchhost.AgentProcess {
	if agent != "codex" {
		return launchhost.AgentProcess{}
	}
	if detached {
		return launchhost.DecodeAgent(os.Getenv(agentProcessEnv), time.Now().Unix())
	}
	m, ok := agents.For(agent)
	if !ok {
		return launchhost.AgentProcess{}
	}
	commands := append([]string{m.Key}, m.Detect...)
	commands = append(commands, m.InstallDetect...)
	return observeAgentProcess(commands)
}
