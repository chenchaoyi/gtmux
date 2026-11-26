package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/userservice"
)

// Enter the storage-only repair dialog before acquiring ordinary shared
// admission. The migration itself takes exclusive admission and rechecks.
func doctorStorageEntry(command string, args []string) (int, bool) {
	if command != "doctor" && command != "dr" {
		return 0, false
	}
	if len(args) > 0 && (layoutDoctorFlag(args[0]) || args[0] == "--help" || args[0] == "-h") {
		return 0, false
	}
	if !slices.Contains(args, "--fix") && len(args) != 0 &&
		!slices.Contains(args, "--no-prompt") && !slices.Contains(args, "--json") {
		return 0, false
	}
	fault := state.CheckStorageLayout()
	interactiveFix := len(args) == 1 && args[0] == "--fix" && isTTY()
	if fault == nil && !interactiveFix {
		return 0, false
	}
	status, err := state.ReadStorageLayout()
	if err == nil {
		workflow, grouped, pending := status.PendingChange()
		if interactiveFix && runtime.GOOS == "darwin" && !tmux.InTmux() {
			_, planErr := os.Lstat(storageServicePlanPath())
			if _, cleanupErr := os.Lstat(storageServiceCleanupPath()); !os.IsNotExist(cleanupErr) {
				if err := finishStorageServiceCleanup(systemStorageServiceIO()); err != nil {
					return guidedStorageRecoveryFailure(err), true
				}
				i18n.Say("Storage service recovery cleanup finished.", "存储维护的服务恢复记录已清理。")
				return 0, true
			}
			hasPlan := !os.IsNotExist(planErr)
			recoverServices := status.WorkflowActive() && hasPlan
			groupWorkflow := hasPlan && (status.ServicePreparation() || pending && workflow && grouped) ||
				fault == nil && !status.WorkflowActive() && status.Mode == "grouped"
			if recoverServices || groupWorkflow {
				manager := doctorStorageLaunchd()
				if groupWorkflow && !pending && os.IsNotExist(planErr) {
					jobs, inspectErr := manager.Inspect()
					if inspectErr != nil {
						return guidedStorageFailure(inspectErr), true
					}
					if err := checkStorageProcesses(userservice.StorageLaunchdPIDs(jobs)); err != nil {
						i18n.Sae("Storage grouping deferred: "+err.Error(), "暂缓数据目录迁移："+err.Error())
						return 0, false
					}
				}
				return guidedDoctorStorageChange(manager, checkStorageProcesses, checkEarlyStorageWriters, confirmRisky), true
			}
		}
		switch {
		case pending && interactiveFix:
			return applyDoctorStorageChange(workflow, grouped, checkEarlyStorageWriters, confirmRisky), true
		case fault == nil && interactiveFix && !status.WorkflowActive() && status.Mode == "grouped":
			if checkEarlyStorageWriters() != nil {
				return 0, false
			}
			return applyDoctorStorageChange(true, true, checkEarlyStorageWriters, confirmRisky), true
		}
	}
	if fault == nil {
		return 0, false
	}
	// A valid prepared fence blocks ordinary admission without a readback error.
	if _, _, pending := status.PendingChange(); err == nil && !pending && !status.ServicePreparation() {
		err = fault
	}
	sections := []dsection{{i18n.Tr("Storage", "数据目录"), []dcheck{workflowLayoutRow(status, err)}}}
	if slices.Contains(args, "--json") && !slices.Contains(args, "--fix") {
		return writeDoctorJSON(os.Stdout, sections, time.Now()), true
	}
	renderSections(sections)
	renderDoctorResults(os.Stdout, sections)
	return 1, true
}

func checkEarlyStorageWriters() error {
	if tmux.InTmux() {
		return fmt.Errorf("this terminal is inside tmux")
	}
	return checkStorageWriters()
}

func doctorStorageLaunchd() userservice.StorageLaunchd {
	return userservice.StorageLaunchd{
		Home:       state.Home(),
		Executable: selfPath(),
		Labels:     []string{serveAgentLabel, tunnelAgentLabel, selfTunnelAgentLabel},
	}
}

func checkStorageMigrationWindow() error {
	if tmux.InTmux() {
		return errors.New(i18n.Tr("the current terminal is inside tmux", "当前终端在 tmux 内"))
	}
	if runtime.GOOS != "darwin" {
		return checkStorageWriters()
	}
	manager := doctorStorageLaunchd()
	jobs, err := manager.Inspect()
	if err != nil {
		return err
	}
	if err := checkStorageProcesses(userservice.StorageLaunchdPIDs(jobs)); err != nil {
		return err
	}
	if err := checkLegacyMenuBar(manager); err != nil {
		return err
	}
	for _, job := range jobs {
		if job.Parked || job.Loaded && job.Override[0] == "disabled" {
			return fmt.Errorf(i18n.Tr("review the settings for %s before migration", "迁移前需核对 %s 的设置"), job.Label)
		}
	}
	return nil
}

func storageMigrationAdvice() string {
	hasData, err := state.WorkflowMigrationHasData()
	if err != nil {
		return i18n.Tr("Migration to layout-v3 requires review. Migration deferred: ", "迁移到 layout-v3 前需要核对。迁移暂缓：") + err.Error() +
			i18n.Tr(". Inspect the paths with gtmux doctor --storage-layout.", "。请运行 gtmux doctor --storage-layout 核对路径。")
	}
	if !hasData {
		return ""
	}
	intro := i18n.Tr("You can move HQ wake data and global locks to layout-v3. ",
		"可将 HQ 唤醒数据和全局锁迁移到 layout-v3。")
	if err := checkStorageMigrationWindow(); err != nil {
		return intro + i18n.Tr("Migration deferred: ", "迁移暂缓：") + err.Error() +
			i18n.Tr(". After resolving this, run gtmux doctor --fix from a standalone terminal after reboot and before restoring tmux.",
				"。处理后，在重启且尚未恢复 tmux 时，从独立终端运行 gtmux doctor --fix。")
	}
	return intro + i18n.Tr("No blocker found in this check. If you have rebooted and not restored tmux yet, run gtmux doctor --fix from a standalone terminal to review and confirm the move.",
		"本次检查未发现阻碍。若已重启且尚未恢复 tmux，从独立终端运行 gtmux doctor --fix 查看并确认迁移。")
}

func applyDoctorStorageChange(workflow, grouped bool, writers func() error, confirmReady func(string) bool) int {
	lease, err := storageMaintenanceLease()
	if err != nil {
		return guidedStorageFailure(err)
	}
	defer lease.Close()
	if err := writers(); err != nil {
		i18n.Sae("Storage maintenance is blocked: "+err.Error(),
			"数据目录维护被阻止："+err.Error())
		return 1
	}
	if workflow {
		if err := previewDoctorWorkflow(grouped); err != nil {
			i18n.Sae("Storage preview failed: "+err.Error(), "数据目录预览失败："+err.Error())
			return 1
		}
	}
	if !confirmReady(i18n.Tr(
		"Only proceed after reboot, before opening or restoring any tmux session, with gtmux, agent hooks and automatic service starts stopped. Apply the recorded storage change [y/N] ",
		"请在重启后、打开或恢复任何 tmux 会话之前操作，并停用 gtmux、agent hook 和服务自动启动。确认执行存储变更 [y/N] ")) {
		return 0
	}
	change := state.ChangeStorageLayout
	if workflow {
		change = state.ChangeWorkflowLayout
	}
	if err := change(grouped, writers); err != nil {
		i18n.Sae("Storage maintenance stopped: "+err.Error(),
			"数据目录维护已停住："+err.Error())
		return 1
	}
	status, err := state.ReadStorageLayout()
	if err != nil {
		i18n.Sae("Storage readback failed: "+err.Error(),
			"数据目录复查失败："+err.Error())
		return 1
	}
	renderSections([]dsection{{i18n.Tr("Storage", "数据目录"), []dcheck{workflowLayoutRow(status, nil)}}})
	return 0
}

func workflowLayoutRow(status state.StorageLayoutStatus, err error) dcheck {
	label := i18n.Tr("data layout", "数据目录布局")
	if status.ServicePreparation() && err == nil {
		return dcheck{stRec, label, i18n.Tr("service recovery preparing", "后台服务已进入维护"),
			i18n.Tr("Run gtmux doctor --fix before opening tmux to continue or cancel the prepared move.",
				"请在打开 tmux 前运行 gtmux doctor --fix，继续或撤回已准备的迁移。")}
	}
	if _, grouped, pending := status.PendingChange(); pending {
		action := i18n.Tr("finish grouping", "继续分组")
		if !grouped {
			action = i18n.Tr("finish rolling back", "继续回退")
		}
		if err == nil {
			return dcheck{stMiss, label, action,
				i18n.Tr("After reboot, before restoring tmux, run gtmux doctor --fix in a fresh terminal to review the recorded change.",
					"重启后、恢复 tmux 前，在独立终端运行 gtmux doctor --fix 查看既有迁移记录并继续。")}
		}
	}
	if err != nil {
		return dcheck{stMiss, label, i18n.Tr("needs review", "需要核对"),
			err.Error() + i18n.Tr(". Preserve both paths and use gtmux doctor --storage-layout to inspect the recorded layout.",
				"。保留两处原件，使用 gtmux doctor --storage-layout 核对已记录的布局。")}
	}
	for _, store := range status.Stores {
		if store.Unexpected {
			return dcheck{stMiss, label, i18n.Tr("inactive path exists", "非活动路径有数据"),
				store.Name + i18n.Tr(" · inspect both copies with gtmux doctor --storage-layout",
					" · 用 gtmux doctor --storage-layout 核对两处原件")}
		}
	}
	if status.WorkflowActive() {
		if runtime.GOOS == "darwin" {
			if _, err := os.Lstat(storageServicePlanPath()); !os.IsNotExist(err) {
				return dcheck{stRec, label,
					i18n.Tr("layout-v3 · service recovery pending", "layout-v3 · 后台服务待恢复"),
					i18n.Tr("Run gtmux doctor --fix in a standalone terminal to restore the saved settings.",
						"请在独立终端运行 gtmux doctor --fix，恢复已记录的后台服务设置。")}
			}
		}
		return dcheck{stOK, label, "layout-v3",
			i18n.Tr("HQ wake data and global leases are grouped. Early-boot layout lock, desktop endpoints and input spool use their stable paths.",
				"HQ 唤醒数据和全局锁已分组。启动布局锁、桌面端点和输入队列使用各自的固定路径。")}
	}
	value := "layout-v2"
	if status.Mode != "grouped" {
		value = i18n.Tr("flat", "平铺")
	}
	return dcheck{stInfo, label, value, ""}
}

func rowWorkflowStorageLayout() dcheck {
	status, err := state.ReadStorageLayout()
	row := workflowLayoutRow(status, err)
	if row.status == stInfo && row.value == "layout-v2" {
		row.note = storageMigrationAdvice()
	}
	return row
}

func rowWakeStorage() dcheck {
	usage, err := hqnudge.InspectStorage()
	label := i18n.Tr("HQ wake delivery", "HQ 唤醒投递")
	if err != nil {
		return dcheck{stMiss, label, i18n.Tr("inspection incomplete", "盘点未完成"), err.Error()}
	}
	value := fmt.Sprintf(i18n.Tr("%d queued · %d claimed · %d refused and retained · %d unknown · %d awaiting proof · %d unrecognized",
		"%d 待投递 · %d 已领取 · %d 已拒绝留存 · %d 结果未知 · %d 待送达核实 · %d 未识别"),
		usage.Queued, usage.Claimed, usage.Refused, usage.Uncertain, usage.AwaitingProof, usage.Unclassified)
	var notes []string
	if usage.Uncertain > 0 {
		notes = append(notes, i18n.Tr("Unknown outcomes remain on hold. Check delivery evidence for each batch.",
			"结果未知的原件继续保留。请按批次核对送达证据。"))
	}
	if usage.ConflictingSlots > 0 {
		slots := i18n.Pl(usage.ConflictingSlots, i18n.Tr("wake slot", "个投递槽位"))
		notes = append(notes, fmt.Sprintf(i18n.Tr("Distinct claimed and retained files share %s. Keep both files.",
			"%s同时存在不同的领取文件和留存文件。请保留两份原件。"), slots))
	}
	if usage.Unclassified > 0 {
		notes = append(notes, i18n.Tr("Verify ownership and delivery evidence for unrecognized entries.",
			"请核对未识别条目的归属和投递证据。"))
	}
	if len(notes) > 0 {
		return dcheck{stRec, label, value, strings.Join(notes, " ")}
	}
	return dcheck{stInfo, label, value, filepath.Dir(state.WorkflowPath("hq-nudges"))}
}

func rowCacheStaging() dcheck {
	plan, err := state.InspectCacheStaging(time.Now())
	label := i18n.Tr("cache staging", "缓存暂存")
	if err != nil {
		return dcheck{stRec, label, i18n.Tr("could not inspect", "无法盘点"), err.Error()}
	}
	if plan.Count == 0 {
		return dcheck{stOK, label, i18n.Tr("no expired stages", "无过期暂存"), ""}
	}
	return dcheck{stRec, label, fmt.Sprintf(i18n.Tr("%d files · %s logical size", "%d 个文件 · 逻辑大小 %s"), plan.Count, humanBytes(plan.Bytes)),
		i18n.Tr("Review cleanup with gtmux doctor --fix.", "可用 gtmux doctor --fix 核对并清理。")}
}

func (s *fixState) stepCacheStaging() int {
	plan, err := state.InspectCacheStaging(time.Now())
	if err != nil || plan.Count == 0 {
		return 0
	}
	if !s.ask(i18n.Tr("Remove expired cache stages", "清理过期缓存暂存"),
		fmt.Sprintf(i18n.Tr("%d incomplete cache files, %s total logical size", "%d 个未完成的缓存文件，逻辑大小共 %s"), plan.Count, humanBytes(plan.Bytes))) {
		return 0
	}
	removed, err := plan.Prune()
	if err != nil {
		s.rc = 1
		i18n.Sae("Cache cleanup stopped: "+err.Error(), "缓存清理已停住："+err.Error())
	}
	i18n.Say(fmt.Sprintf("Removed %d cache staging files.", removed), fmt.Sprintf("已清理 %d 个缓存暂存文件。", removed))
	return removed
}
