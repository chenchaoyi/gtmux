package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func layoutDoctorFlag(flag string) bool {
	switch flag {
	case "--storage-layout", "--group-storage", "--flatten-storage":
		return true
	}
	return false
}

func permitsLayoutTransition(command string, args []string) bool {
	return command == "" || command == "help" || command == "-h" || command == "--help" ||
		command == "version" || command == "-v" || command == "--version" ||
		command == "doctor" && len(args) == 1 && (layoutDoctorFlag(args[0]) || args[0] == "--help" || args[0] == "-h")
}

func ensureCommandLayout(command string, args []string) bool {
	if permitsLayoutTransition(command, args) {
		return true
	}
	if err := state.CheckStorageLayout(); err != nil {
		i18n.Sae("gtmux: "+err.Error(), "gtmux：数据目录布局需核查，请运行 gtmux doctor --storage-layout。"+err.Error())
		return false
	}
	return true
}

func cmdDoctorLayout(flag string) int {
	if flag == "--storage-layout" {
		status, err := state.ReadStorageLayout()
		if err != nil {
			i18n.Sae("Storage layout: "+err.Error(), "数据目录布局："+err.Error())
			return 1
		}
		mode := i18n.Tr("flat", "平铺")
		if status.Mode == "grouped" {
			mode = i18n.Tr("grouped", "分组")
		}
		fmt.Println(i18n.Tr("Storage layout", "数据目录布局"), "·", mode)
		if status.Continuing != "" {
			next := "--group-storage"
			if status.Continuing == "flat" {
				next = "--flatten-storage"
			}
			fmt.Println(i18n.Tr("Transition pending. Continue with", "迁移待继续，执行"), "gtmux doctor "+next)
		}
		conflict := status.UnmarkedGrouped
		if status.UnmarkedGrouped {
			fmt.Println(i18n.Tr("Grouped directory has no layout marker", "分组目录缺少布局标记"))
		}
		for _, store := range status.Stores {
			if store.Unexpected && status.Continuing == "" {
				fmt.Println(i18n.Tr("Review inactive path", "需要核查非活动位置"), "·", store.Name)
				conflict = true
			}
			if store.Present || store.Unexpected {
				fmt.Printf("  %s → %s\n", store.Name, state.FormatLayoutStore(store))
			}
		}
		if conflict {
			return 1
		}
		return 0
	}

	if err := checkStorageWriters(); err != nil {
		i18n.Sae(err.Error(), "无法确认写入方已停用："+err.Error())
		return 1
	}
	if !confirmRisky(i18n.Tr(
		"Have you stopped gtmux, the menu bar, tmux, agent hooks and automatic restarts? Move storage paths [y/N] ",
		"已退出 gtmux、菜单栏、tmux，并暂停 agent hook 及自动重启吗？确认迁移文件路径 [y/N] ")) {
		return 0
	}
	if err := state.ChangeStorageLayout(flag == "--group-storage", checkStorageWriters); err != nil {
		i18n.Sae("Storage layout change stopped: "+err.Error(), "数据目录迁移已停住："+err.Error())
		return 1
	}
	i18n.Say("Storage layout ready. Inspect it with gtmux doctor --storage-layout.", "数据目录布局已就绪，可运行 gtmux doctor --storage-layout 查看。")
	return 0
}

// The operator must pause admissions and restarts, including older hooks.
// A process snapshot is an extra rejection check, not proof about old writers.
func checkStorageWriters() error {
	output, err := exec.Command("ps", "-axo", "uid=,pid=,comm=").Output()
	if err != nil {
		return fmt.Errorf("could not inspect local storage writers: %w", err)
	}
	uid := strconv.Itoa(os.Getuid())
	pid := strconv.Itoa(os.Getpid())
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != uid || fields[1] == pid {
			continue
		}
		name := strings.ToLower(filepath.Base(strings.Join(fields[2:], " ")))
		if name == "gtmux" || name == "gtmuxbar" || name == "gtmux-bar" ||
			name == "tmux" || strings.HasPrefix(name, "tmux:") {
			return fmt.Errorf("a gtmux or tmux process is still running (PID %s)", fields[1])
		}
	}
	return nil
}
