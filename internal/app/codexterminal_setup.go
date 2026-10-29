package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const codexTerminalBegin = "# >>> gtmux Codex terminal bridge >>>"
const codexTerminalEnd = "# <<< gtmux Codex terminal bridge <<<"

func codexTerminalBlock(bin string) string {
	return codexTerminalBegin + `
case $- in
  *i*)
    if [ -n "${TMUX-}" ]; then
      case "$(command -v codex 2>/dev/null)" in
        /*) function codex { command ` + shellQuote(bin) + ` codex -- "$@"; } ;;
      esac
    fi
    ;;
esac
` + codexTerminalEnd + "\n"
}

func replaceCodexTerminalBlock(original, block string) (string, error) {
	begin, end := strings.Index(original, codexTerminalBegin), strings.Index(original, codexTerminalEnd)
	if strings.Count(original, codexTerminalBegin) > 1 || strings.Count(original, codexTerminalEnd) > 1 ||
		(begin < 0) != (end < 0) || begin >= 0 && end < begin {
		return original, errors.New("incomplete or duplicate Codex terminal block; shell file unchanged")
	}
	if begin >= 0 {
		end += len(codexTerminalEnd)
		if end < len(original) && original[end] == '\n' {
			end++
		}
		return original[:begin] + block + original[end:], nil
	}
	if block == "" {
		return original, nil
	}
	if original != "" && !strings.HasSuffix(original, "\n") {
		original += "\n"
	}
	return original + block, nil
}

func configureCodexTerminal(path, bin string, enabled bool) error {
	original, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	block := ""
	if enabled {
		block = codexTerminalBlock(bin)
	}
	updated, err := replaceCodexTerminalBlock(string(original), block)
	if err != nil || updated == string(original) {
		return err
	}
	if err := state.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	backupFile(path)
	return state.WriteForeign(path, []byte(updated), 0644)
}

func (s *fixState) stepCodexTerminal() int {
	if nativeCodexBinary() == "" {
		return 0
	}
	shell := filepath.Base(os.Getenv("SHELL"))
	if shell != "bash" && shell != "zsh" {
		return 0
	}
	rc, _ := shellRCPath()
	block := codexTerminalBlock(s.hookBin())
	original, _ := os.ReadFile(rc)
	if strings.Contains(string(original), block) {
		return 0
	}
	detail := i18n.Tr("Add a managed block to "+tildeify(rc)+":\n"+block+"New interactive tmux shells use the verified Codex frontend. Existing aliases, functions and running sessions keep their current behavior.",
		"在 "+tildeify(rc)+" 中加入可移除的配置段：\n"+block+"新开的 tmux 交互终端会核实 Codex 会话归属。现有别名、自定义函数和运行中的会话保持原样。")
	if !s.ask(i18n.Tr("Codex terminal identity", "Codex 终端会话归属"), detail) {
		return 0
	}
	if err := configureCodexTerminal(rc, s.hookBin(), true); err != nil {
		i18n.Sae("Could not configure Codex terminal: "+err.Error(), "配置 Codex 终端失败："+err.Error())
		s.rc = 1
		return 0
	}
	i18n.Say("✓ Codex terminal identity is configured for new tmux shells.", "✓ 已配置 Codex 会话归属，对新开的 tmux 终端生效。")
	return 1
}
