package app

import (
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

//go:embed dsh_plugin.mjs
var dshPluginTemplate string

func dshPluginJS(bin string) string {
	return strings.ReplaceAll(dshPluginTemplate, "__GTMUX_BIN__", strconv.Quote(bin))
}
func dshOverlayPath(inst agentInstaller) string {
	return filepath.Join(filepath.Dir(inst.configPath()), "gtmux.cordis.yml")
}

// Both files are owned by gtmux. User profiles and Codex compatibility hooks are
// never rewritten; the overlay is selected explicitly on the next Web launch.
func updateDSHPlugin(inst agentInstaller, bin string, install bool) error {
	path := inst.configPath()
	overlay := dshOverlayPath(inst)
	const marker = "// gtmux native DeepSeek Harness plugin;"
	const yamlMarker = "# gtmux native DeepSeek Harness overlay\n"
	for p, prefix := range map[string]string{path: marker, overlay: yamlMarker} {
		b, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && !strings.HasPrefix(string(b), prefix) {
			return fmt.Errorf("refusing to replace or remove an unrecognized file at %s", p)
		}
	}
	if !install {
		for _, p := range []string{overlay, path} {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	if err := writeAgentPlugin(path, inst.plugin(bin)); err != nil {
		return err
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	return writeAgentPlugin(overlay, yamlMarker+"- insert:\n    - id: gtmux-native-dsh\n      name: "+strconv.Quote(uri)+"\n")
}
func installDSHHooks(inst agentInstaller) int {
	if err := updateDSHPlugin(inst, selfPath(), true); err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 1
	}
	i18n.Say("Installed the native DSH plugin. On your next Web launch, add:", "已安装 DSH 原生插件。下次启动 Web 时添加：")
	fmt.Printf("dsh web --patch %s\n", shellQuote(dshOverlayPath(inst)))
	i18n.Say("The running Web server is unchanged. Conversation history is already readable without this plugin.", "正在运行的 Web 服务未改动。读取已有对话无需该插件。")
	return 0
}
