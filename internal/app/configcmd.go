package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func configPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json")
}

// cmdConfig implements `gtmux config agent-proxy [off|on|<url>]` — set or show how
// agent launches are proxied. The choice is EXPLICIT: gtmux never probes the network
// (TUN vs a double-VPN can't be told apart). The env var GTMUX_AGENT_PROXY overrides
// the config for a per-network switch.
func cmdConfig(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		return configUsage()
	}
	switch args[0] {
	case "agent-proxy":
		return configAgentProxy(args[1:])
	default:
		i18n.Sae("gtmux config: unknown key '"+args[0]+"'", "gtmux config: 未知配置项 '"+args[0]+"'")
		return configUsage()
	}
}

func configAgentProxy(args []string) int {
	if len(args) == 0 { // show the resolved value a launch would use
		i18n.Say("agent-proxy (launch) = "+shownProxy(), "起 agent 代理 = "+shownProxy())
		return 0
	}
	v := strings.TrimSpace(args[0])
	if v != "off" && v != "on" && !strings.Contains(v, "://") {
		i18n.Sae("gtmux config agent-proxy: value must be off | on | <url>",
			"gtmux config agent-proxy: 取值须为 off | on | <url>")
		return 2
	}
	if err := setConfigKey("agentProxy", v); err != nil {
		i18n.Sae("gtmux config: "+err.Error(), "gtmux config: "+err.Error())
		return 1
	}
	i18n.Say("set agentProxy = "+v+"  (launch now: "+shownProxy()+")",
		"已设置 agentProxy = "+v+"（现在起 agent："+shownProxy()+"）")
	return 0
}

// shownProxy is the proxy a launch WOULD apply now (config + env resolved).
func shownProxy() string {
	if a := agentenv.Active(); a != "" {
		return a
	}
	return "off (no proxy)"
}

// setConfigKey merges one key into config.json, preserving any other keys.
func setConfigKey(key string, value any) error {
	m := map[string]any{}
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	m[key] = value
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), append(b, '\n'), 0o644)
}

func configUsage() int {
	i18n.Sae(
		"usage: gtmux config agent-proxy [off|on|<url>]\n"+
			"  off  launch agents bare (office / Clash TUN — direct works)\n"+
			"  on   proxy via 127.0.0.1:7897 (home / double-VPN — direct 403s)\n"+
			"  <url>  an explicit proxy URL\n"+
			"  (no value shows the current resolved proxy; env GTMUX_AGENT_PROXY overrides)",
		"用法：gtmux config agent-proxy [off|on|<url>]\n"+
			"  off   裸起 agent（办公网 / Clash TUN —— 直连可达）\n"+
			"  on    经 127.0.0.1:7897 代理（家里 / 双层VPN —— 直连 403）\n"+
			"  <url> 显式代理 URL\n"+
			"  （不带值则显示当前生效值;环境变量 GTMUX_AGENT_PROXY 优先）")
	return 0
}
