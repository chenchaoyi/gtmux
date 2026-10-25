package app

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

//go:embed dsh_plugin.mjs
var dshPluginTemplate string

//go:embed dsh_terminal_plugin.mjs
var dshTerminalPluginTemplate string

const dshTerminalMarker = "// gtmux native DeepSeek Harness terminal bridge;"
const dshTerminalOverlayMarker = "# gtmux native DeepSeek Harness terminal overlay\n"

func dshTerminalOverlayPath(inst agentInstaller) string {
	return filepath.Join(filepath.Dir(inst.configPath()), "gtmux.dst.cordis.yml")
}

func dshTerminalArtifacts(inst agentInstaller) map[string]string {
	root := filepath.Join(filepath.Dir(inst.configPath()), "terminal-dst")
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "bridge.mjs")}).String()
	manifest := map[string]any{
		"$schema": "urn:gtmux:dsh-terminal-component:1", "manifestVersion": "0.15",
		"id": "gtmux.dsh-terminal", "name": "Gtmux DeepSeek Harness terminal bridge", "version": "1.0.0",
		"facets":        map[string]any{"host": map[string]string{"entry": "./bridge.mjs", "apiVersion": "v1alpha1"}},
		"requires":      map[string]any{"contracts": []any{map[string]string{"apiVersion": "commands.dsh/v1alpha1", "kind": "Command"}}},
		"permissions":   []any{map[string]string{"name": "commands.invoke", "scope": "gtmux.dsh-send", "reason": "Deliver an addressed user request through its exact receiving agent"}},
		"contributes":   map[string]any{"commands": []any{map[string]string{"id": "gtmux.dsh-send", "title": "Deliver gtmux message"}}},
		"subscriptions": []any{}, "license": "MIT",
		"source": map[string]string{"repository": "https://github.com/chenchaoyi/gtmux"},
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	return map[string]string{
		filepath.Join(root, "bridge.mjs"):      dshTerminalPluginTemplate,
		filepath.Join(root, "package.json"):    "{\"name\":\"@gtmux/dsh-terminal-dst\",\"version\":\"1.0.0\",\"type\":\"module\",\"private\":true,\"gtmuxOwned\":true}\n",
		filepath.Join(root, "dsh-plugin.json"): string(b) + "\n",
		dshTerminalOverlayPath(inst):           dshTerminalOverlayMarker + "- insert:\n    - id: gtmux-native-dsh-terminal\n      name: " + strconv.Quote(uri) + "\n",
	}
}

func dshTerminalOwned(path string, data []byte) bool {
	switch filepath.Base(path) {
	case "bridge.mjs":
		return strings.HasPrefix(string(data), dshTerminalMarker)
	case "gtmux.dst.cordis.yml":
		return strings.HasPrefix(string(data), dshTerminalOverlayMarker)
	case "package.json":
		var v struct {
			Name       string
			GtmuxOwned bool
		}
		return json.Unmarshal(data, &v) == nil && v.Name == "@gtmux/dsh-terminal-dst" && v.GtmuxOwned
	case "dsh-plugin.json":
		var v struct {
			ID     string
			Source struct{ Repository string }
		}
		return json.Unmarshal(data, &v) == nil && v.ID == "gtmux.dsh-terminal" && v.Source.Repository == "https://github.com/chenchaoyi/gtmux"
	}
	return false
}

func dshPluginJS(bin string) string { return dshPluginForVersion(bin, Version) }

func dshPluginForVersion(bin, version string) string {
	return strings.NewReplacer("__GTMUX_BIN__", strconv.Quote(bin), "__GTMUX_VERSION__", strconv.Quote(version)).Replace(dshPluginTemplate)
}

// Account-request metadata changing across releases does not make a bridge stale.
// Every executable statement must still match the current generated template.
func dshPluginCompatible(module []byte, bin string) bool {
	version := dshPluginVersion(module)
	return version != "" && string(module) == dshPluginForVersion(bin, version)
}

func dshPluginVersion(module []byte) string {
	_, tail, ok := strings.Cut(string(module), "account.getBalance({version: ")
	if !ok {
		return ""
	}
	quoted, _, ok := strings.Cut(tail, ", locale:")
	if !ok {
		return ""
	}
	var version string
	if json.Unmarshal([]byte(quoted), &version) != nil {
		return ""
	}
	return version
}

func dshBridgeFromNewerRelease(module []byte) bool {
	installed := "v" + strings.TrimPrefix(dshPluginVersion(module), "v")
	current := "v" + strings.TrimPrefix(Version, "v")
	return comparableRelease(installed) && (!comparableRelease(current) || newerRelease(installed, current))
}

const dshPluginMarker = "// gtmux native DeepSeek Harness plugin;"
const dshOverlayMarker = "# gtmux native DeepSeek Harness overlay\n"

func dshOverlayPath(inst agentInstaller) string {
	return filepath.Join(filepath.Dir(inst.configPath()), "gtmux.cordis.yml")
}

// Owned artifacts are separate from user configuration. The dst profile writer
// changes only its marked block; Web still selects an explicit launch overlay.
func updateDSHPlugin(inst agentInstaller, bin string, install bool) error {
	path := inst.configPath()
	overlay := dshOverlayPath(inst)
	artifacts := dshTerminalArtifacts(inst)
	profile, original, next, err := prepareDSTProfile(inst, install)
	if err != nil {
		return err
	}

	// Preflight every provider-owned artifact before writing any of them.
	for p := range artifacts {
		b, err := readDSHArtifact(p)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && !dshTerminalOwned(p, b) {
			return fmt.Errorf("refusing to replace or remove an unrecognized file at %s", p)
		}
	}
	for p, prefix := range map[string]string{path: dshPluginMarker, overlay: dshOverlayMarker} {
		b, err := readDSHArtifact(p)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && !strings.HasPrefix(string(b), prefix) {
			return fmt.Errorf("refusing to replace or remove an unrecognized file at %s", p)
		}
	}
	if !install {
		if err := writeDSTProfile(profile, original, next); err != nil {
			return err
		}
		for p := range artifacts {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
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
	for p, content := range artifacts {
		if err := writeAgentPlugin(p, content); err != nil {
			return err
		}
	}
	if err := writeAgentPlugin(overlay, dshOverlayYAML(inst)); err != nil {
		return err
	}
	return writeDSTProfile(profile, original, next)
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
	if agentHookExecutablePresent("dsh") && lookTool("dst") != "" {
		i18n.Say("For a managed DeepSeek Harness terminal session, launch:", "启动受管 DeepSeek Harness 终端会话：")
		fmt.Println("dst --backend dsh")
	}
	i18n.Say("The running Web server is unchanged. Conversation history is already readable without this plugin.", "正在运行的 Web 服务未改动。读取已有对话无需该插件。")
}

func rowDSHHook(inst agentInstaller) dcheck {
	label := inst.display + " hook"
	module, moduleErr := os.ReadFile(inst.configPath())
	overlay, overlayErr := os.ReadFile(dshOverlayPath(inst))
	repair := i18n.Tr("DeepSeek state updates may be incomplete; saved conversations remain readable. Review repairs to install or update the gtmux connection component (gtmux doctor --fix). Load it on the next launch with the gtmux configuration.", "DeepSeek 运行、提问及完成状态的同步可能不完整，已有对话仍可读取。可在“查看修复方案”中安装或更新状态连接组件（gtmux doctor --fix），下次带 gtmux 配置启动时加载。")
	if moduleErr != nil || overlayErr != nil {
		return dcheck{stRec, label, i18n.Tr("not fully installed", "尚未装全"), repair}
	}
	if !strings.HasPrefix(string(module), dshPluginMarker) || !strings.HasPrefix(string(overlay), dshOverlayMarker) {
		return dcheck{stRec, label, i18n.Tr("unrecognized plugin files", "插件文件归属不明"), i18n.Tr("preserve the existing files; doctor will not overwrite them", "请先核对现有文件；doctor 不会覆盖归属不明的文件")}
	}
	if !agentPluginCurrent(inst) {
		if dshBridgeFromNewerRelease(module) {
			return dcheck{stInfo, label, i18n.Tr("managed by a newer gtmux release", "组件来自更新的 gtmux 版本"), i18n.Tr("Update the CLI and menu-bar app together. Existing bridge files will not be downgraded.", "请将 CLI 和菜单栏更新为同一版本；现有连接组件不会自动降级。")}
		}
		return dcheck{stRec, label, i18n.Tr("state connection component needs updating", "状态连接组件需要更新"), repair}
	}
	if string(overlay) != dshOverlayYAML(inst) {
		return dcheck{stRec, label, i18n.Tr("Web launch configuration needs repair", "Web 启动配置需要修复"), repair}
	}
	for path, wanted := range dshTerminalArtifacts(inst) {
		actual, err := os.ReadFile(path)
		if err != nil || string(actual) != wanted {
			return dcheck{stRec, label, i18n.Tr("terminal connection component needs repair", "终端会话接入组件需要修复"), repair}
		}
	}
	if path, old, next, err := prepareDSTProfile(inst, true); err != nil || path != "" && string(old) != string(next) {
		return dcheck{stRec, label, i18n.Tr("dst connection configuration needs repair", "dst 会话接入配置需要修复"), repair}
	}
	note := i18n.Tr("Loading in running Web is unverified. Next launch: ", "尚未核实运行中的 Web 是否已加载。下次启动：") + "dsh web --patch " + shellQuote(dshOverlayPath(inst))
	return dcheck{stOK, label, i18n.Tr("installed", "已装"), note}
}

// Refuse links and oversized artifacts before recognizing an ownership marker.
func readDSHArtifact(path string) ([]byte, error) {
	var b []byte
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	st, owned := fi.Sys().(*syscall.Stat_t)
	if !fi.Mode().IsRegular() || fi.Size() > 1<<20 || !owned || int(st.Uid) != os.Getuid() || fi.Mode().Perm()&0022 != 0 {
		return nil, fmt.Errorf("untrusted DSH artifact")
	}
	f, err := state.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(fi, opened) {
		return nil, fmt.Errorf("DSH artifact changed")
	}
	b, err = io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil || len(b) > 1<<20 {
		return nil, fmt.Errorf("DSH artifact exceeds limit")
	}
	return b, nil
}
