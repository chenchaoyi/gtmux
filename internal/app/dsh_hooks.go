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

const dshPluginMarker = "// gtmux native DeepSeek Harness plugin;"
const dshOverlayMarker = "# gtmux native DeepSeek Harness overlay\n"

func dshOverlayPath(inst agentInstaller) string {
	return filepath.Join(filepath.Dir(inst.configPath()), "gtmux.cordis.yml")
}

// Both files are owned by gtmux. User profiles and Codex compatibility hooks are
// never rewritten; the overlay is selected explicitly on the next Web launch.
func updateDSHPlugin(inst agentInstaller, bin string, install bool) error {
	path := inst.configPath()
	overlay := dshOverlayPath(inst)
	for p, prefix := range map[string]string{path: dshPluginMarker, overlay: dshOverlayMarker} {
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
	return writeAgentPlugin(overlay, dshOverlayYAML(inst))
}
func dshOverlayYAML(inst agentInstaller) string {
	uri := (&url.URL{Scheme: "file", Path: inst.configPath()}).String()
	return dshOverlayMarker + "- insert:\n    - id: gtmux-native-dsh\n      name: " + strconv.Quote(uri) + "\n"
}
func installDSHPlugin(inst agentInstaller, bin string) error { return updateDSHPlugin(inst, bin, true) }

func installDSHHooks(inst agentInstaller) int {
	if err := writeAgentHookFiles(inst, selfPath()); err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 1
	}
	printDSHLaunch(inst)
	return 0
}

func printDSHLaunch(inst agentInstaller) {
	i18n.Say("Installed the native DSH plugin. On your next Web launch, add:", "已安装 DSH 原生插件。下次启动 Web 时添加：")
	fmt.Printf("dsh web --patch %s\n", shellQuote(dshOverlayPath(inst)))
	i18n.Say("The running Web server is unchanged. Conversation history is already readable without this plugin.", "正在运行的 Web 服务未改动。读取已有对话无需该插件。")
}

func rowDSHHook(inst agentInstaller) dcheck {
	label := inst.display + " hook"
	module, moduleErr := os.ReadFile(inst.configPath())
	overlay, overlayErr := os.ReadFile(dshOverlayPath(inst))
	repair := i18n.Tr("gtmux doctor --fix installs both native plugin files", "gtmux doctor --fix 可安装两份原生插件文件")
	if moduleErr != nil || overlayErr != nil {
		return dcheck{stRec, label, i18n.Tr("not fully installed", "尚未装全"), repair}
	}
	if !strings.HasPrefix(string(module), dshPluginMarker) || !strings.HasPrefix(string(overlay), dshOverlayMarker) {
		return dcheck{stRec, label, i18n.Tr("unrecognized plugin files", "插件文件归属不明"), i18n.Tr("preserve the existing files; doctor will not overwrite them", "请先核对现有文件；doctor 不会覆盖归属不明的文件")}
	}
	if !agentPluginCurrent(inst) {
		return dcheck{stRec, label, i18n.Tr("outdated native plugin", "原生插件需要更新"), repair}
	}
	if string(overlay) != dshOverlayYAML(inst) {
		return dcheck{stRec, label, i18n.Tr("launch overlay needs repair", "启动覆盖文件需要修复"), repair}
	}
	note := i18n.Tr("Loading in running Web is unverified. Next launch: ", "尚未核实运行中的 Web 是否已加载。下次启动：") + "dsh web --patch " + shellQuote(dshOverlayPath(inst))
	return dcheck{stOK, label, i18n.Tr("installed", "已装"), note}
}
