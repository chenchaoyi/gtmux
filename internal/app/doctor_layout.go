package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/userservice"
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
		(command == "doctor" || command == "dr") && len(args) == 1 && (layoutDoctorFlag(args[0]) || args[0] == "--help" || args[0] == "-h")
}

func readOnlyDoctor(command string, args []string) bool {
	if command != "doctor" && command != "dr" {
		return false
	}
	for _, arg := range args {
		switch arg {
		case "--json", "--no-prompt", "--progress":
		default:
			return false
		}
	}
	return true
}

func enterRuntimeLayout(command string, args []string) (*state.Lease, bool) {
	if permitsLayoutTransition(command, args) {
		return nil, true
	}
	if _, err := os.Lstat(state.Dir()); os.IsNotExist(err) {
		if readOnlyDoctor(command, args) {
			return nil, true
		}
		if err := initializeNewPurposeStorage(); err != nil {
			guidedStorageFailure(err)
			return nil, false
		}
	}
	return enterCommandLayout(command, args)
}

func enterCommandLayout(command string, args []string) (*state.Lease, bool) {
	if permitsLayoutTransition(command, args) {
		return nil, true
	}
	lease, err := state.EnterStorageCommand()
	if err != nil {
		i18n.Sae("gtmux: storage unavailable. Run gtmux doctor --storage-layout. "+err.Error(),
			"gtmux：数据目录暂不可用。请运行 gtmux doctor --storage-layout。"+err.Error())
		return nil, false
	}
	return lease, true
}

func cmdDoctorLayout(flag string) int {
	if flag == "--storage-layout" {
		status, err := state.ReadStorageLayout()
		if err != nil {
			i18n.Sae("Storage layout: "+err.Error(), "数据目录布局："+err.Error())
			return 1
		}
		if status.Mode == "purpose" {
			renderSections([]dsection{{i18n.Tr("Storage", "数据目录"), []dcheck{workflowLayoutRow(status, nil)}}})
			if status.Workflow == "categories-active" {
				rows, e := state.InspectPurposePlacement(status)
				if e != nil {
					return guidedStorageFailure(e)
				}
				for _, row := range rows {
					fmt.Println("  " + row.Current)
				}
			}
			return 0
		}
		mode := i18n.Tr("flat", "平铺")
		if status.Mode == "grouped" {
			mode = i18n.Tr("grouped", "分组")
		}
		fmt.Println(i18n.Tr("Storage layout", "数据目录布局"), "·", mode)
		if status.Workflow != "" {
			renderSections([]dsection{{i18n.Tr("Workflow storage", "流程存储"), []dcheck{workflowLayoutRow(status, nil)}}})
		}
		if status.Continuing != "" {
			next := "--group-storage"
			if status.Continuing == "flat" {
				next = "--flatten-storage"
			}
			fmt.Println(i18n.Tr("Transition pending. Continue with", "迁移待继续，执行"), "gtmux doctor "+next)
		}
		if status.StagedFiles > 0 {
			fmt.Println(i18n.Tr("Interrupted write staging files", "中断写入留下的暂存文件"), status.StagedFiles)
		}
		if status.RetainedPath != "" {
			fmt.Println(i18n.Tr("Staging recovery path", "暂存文件保全位置"), status.RetainedPath)
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
				if store.Progress != "" {
					fmt.Printf("  %s → %s · %s\n", store.Name, state.FormatLayoutStore(store), layoutProgressText(store.Progress))
				} else {
					fmt.Printf("  %s → %s\n", store.Name, state.FormatLayoutStore(store))
				}
			}
		}
		if stores, err := state.InspectWorkflowPaths(); err != nil {
			i18n.Sae(err.Error(), err.Error())
			return 1
		} else {
			for _, store := range stores {
				place := i18n.Tr("at original path", "位于原路径")
				if store.AtGroupedPath {
					place = i18n.Tr("at grouped path", "位于分组路径")
				}
				fmt.Printf("  %s → %s · %s\n", store.Original, store.Grouped, place)
			}
		}
		if conflict {
			return 1
		}
		if _, _, pending := status.PendingChange(); !pending && !status.ServicePreparation() && status.Continuing == "" {
			rows, err := state.InspectPurposePlacement(status)
			if err != nil {
				i18n.Sae(err.Error(), err.Error())
				return 1
			}
			if len(rows) > 0 {
				fmt.Println(i18n.Tr("Storage categories (preview)", "按用途查看目录（预览）"))
			}
			for _, row := range rows {
				note := ""
				if row.Conflict {
					note = i18n.Tr(" · destination already exists; preserve both paths", " · 目标位置已存在，请保留原件")
					conflict = true
				}
				fmt.Printf("  %s → %s%s\n", row.Current, row.Destination, note)
			}
			if conflict {
				return 1
			}
		}
		return 0
	}

	lease, err := storageMaintenanceLease()
	if err != nil {
		return guidedStorageFailure(err)
	}
	defer lease.Close()
	if _, err := os.Lstat(storageServicePlanPath()); !os.IsNotExist(err) {
		return guidedStorageFailure(fmt.Errorf("run gtmux doctor --fix to finish the saved service recovery first"))
	}
	status, err := state.ReadStorageLayout()
	if err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 1
	}
	if err := checkEarlyStorageWriters(); err != nil {
		i18n.Sae(err.Error(), "无法确认写入方已停用："+err.Error())
		return 1
	}
	if !confirmRisky(i18n.Tr(
		"After reboot and before restoring tmux, have you stopped gtmux, its menu bar, agent hooks and automatic restarts? Move storage paths [y/N] ",
		"重启后、恢复 tmux 前，是否已停用 gtmux、菜单栏、agent hook 和自动重启？确认迁移文件路径 [y/N] ")) {
		return 0
	}
	grouped := flag == "--group-storage"
	if status.Mode == "purpose" {
		if grouped {
			return guidedStorageFailure(errors.New("storage already uses categories; run gtmux doctor --fix for any recorded repair"))
		}
		if err := state.ChangePurposeLayout(false, checkEarlyPurposeWriters); err != nil {
			return guidedStorageFailure(err)
		}
		status, err = state.ReadStorageLayout()
		if err != nil {
			return guidedStorageFailure(err)
		}
	}
	if status.Workflow != "" && !grouped {
		if err := state.ChangeWorkflowLayout(false, checkEarlyStorageWriters); err != nil {
			i18n.Sae("Storage layout change stopped: "+err.Error(), "数据目录迁移已停住："+err.Error())
			return 1
		}
	}
	change := state.ChangeStorageLayout
	if grouped && status.Mode == "grouped" && status.Continuing == "" {
		change = state.ChangeWorkflowLayout
	}
	if err := change(grouped, checkEarlyStorageWriters); err != nil {
		i18n.Sae("Storage layout change stopped: "+err.Error(), "数据目录迁移已停住："+err.Error())
		return 1
	}
	i18n.Say("Storage layout ready. Inspect it with gtmux doctor --storage-layout.", "数据目录布局已就绪，可运行 gtmux doctor --storage-layout 查看。")
	return 0
}

func layoutProgressText(progress string) string {
	if progress == "moved" {
		return i18n.Tr("moved", "已迁移")
	}
	return i18n.Tr("pending", "待迁移")
}

// The operator still controls old hooks and custom or renamed writers.
func checkStorageWriters() error {
	if err := checkStorageProcesses(nil); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		if err := checkLegacyMenuBar(userservice.StorageLaunchd{Home: state.Home()}); err != nil {
			return err
		}
	}
	return userservice.CheckStorageStopped(state.Home(), serveAgentLabel, tunnelAgentLabel, selfTunnelAgentLabel)
}

func checkStorageProcesses(servicePIDs []int) error {
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
		if name == "tmux" || strings.HasPrefix(name, "tmux:") {
			return fmt.Errorf(i18n.Tr("%s is still running (PID %s)", "%s 仍在运行（PID %s）"), name, fields[1])
		}
		if name == "gtmuxbar" || name == "gtmux-bar" || name == "gtmux-menubar" {
			return fmt.Errorf(i18n.Tr("quit the gtmux menu bar app (PID %s) and rerun Doctor",
				"请退出 gtmux 菜单栏 App（PID %s），然后重新运行 gtmux doctor --fix"), fields[1])
		}
		if name == "gtmux" {
			pid, _ := strconv.Atoi(fields[1])
			if !slices.Contains(servicePIDs, pid) {
				return fmt.Errorf("%s is still running (PID %s)", name, fields[1])
			}
		}
	}
	return nil
}
