package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/userservice"
)

func storageMaintenanceLease() (*state.Lease, error) {
	return state.TryLeaseIn(filepath.Join(state.Dir(), "locks"), "storage-maintenance")
}

func storageServicePlanPath() string {
	return filepath.Join(state.Dir(), "layout-recovery", "launchd-maintenance.json")
}

func readStorageServicePlan() ([]userservice.StorageLaunchdJob, error) {
	f, err := state.OpenPrivateRead(storageServicePlanPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const limit = 8 << 10
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	var jobs []userservice.StorageLaunchdJob
	if err != nil || len(data) > limit || json.Unmarshal(data, &jobs) != nil || len(jobs) == 0 {
		return nil, errors.New("the saved launchd maintenance plan needs review")
	}
	return jobs, nil
}

func saveStorageServicePlan(jobs []userservice.StorageLaunchdJob) error {
	data, err := json.Marshal(jobs)
	if err != nil {
		return err
	}
	return state.WritePrivateFile(storageServicePlanPath(), data)
}

func restoreDoctorStorageServices(manager userservice.StorageLaunchd, plan []userservice.StorageLaunchdJob) error {
	if len(plan) == 0 {
		return nil
	}
	if err := state.CheckStorageLayout(); err != nil {
		return err
	}
	if err := manager.Restore(plan); err != nil {
		return err
	}
	if err := os.Remove(storageServicePlanPath()); err != nil {
		return err
	}
	return state.SyncPrivateDirectory(filepath.Dir(storageServicePlanPath()))
}

func guidedStorageFailure(err error) int {
	i18n.Sae("Storage maintenance paused: "+err.Error(), "数据目录维护已暂停："+err.Error())
	return 1
}

func guidedDoctorStorageChange(manager userservice.StorageLaunchd, processes func([]int) error, writers func() error, confirm func(string) bool) int {
	lease, err := storageMaintenanceLease()
	if err != nil {
		return guidedStorageFailure(err)
	}
	defer lease.Close()
	plan, err := readStorageServicePlan()
	if err != nil {
		return guidedStorageFailure(err)
	}
	status, err := state.ReadStorageLayout()
	if err != nil {
		return guidedStorageFailure(err)
	}
	if status.WorkflowActive() {
		if err := restoreDoctorStorageServices(manager, plan); err != nil {
			return guidedStorageFailure(err)
		}
		if len(plan) > 0 {
			i18n.Say("Previous gtmux service settings restored.", "已恢复 gtmux 原有后台服务设置。")
		}
		return 0
	}
	workflow, grouping, pending := status.PendingChange()
	if status.Mode != "grouped" || pending && (!workflow || !grouping) {
		return guidedStorageFailure(errors.New("review the recorded direction with gtmux doctor --storage-layout"))
	}
	jobs, err := manager.Inspect()
	if err != nil {
		return guidedStorageFailure(err)
	}
	if plan == nil {
		for _, job := range jobs {
			if job.Loaded && job.Disabled[0] {
				return guidedStorageFailure(fmt.Errorf("the loaded job %s is already disabled and needs review", job.Label))
			}
		}
	}
	if err := processes(userservice.StorageLaunchdPIDs(jobs)); err != nil {
		return guidedStorageFailure(err)
	}
	if err := previewDoctorWorkflow(true); err != nil {
		return guidedStorageFailure(err)
	}
	if !confirm(i18n.Tr(
		"After reboot and before opening or restoring tmux, temporarily pause the managed gtmux services, group storage, and restore their previous settings? [y/N] ",
		"请确认已重启且尚未打开或恢复 tmux。现在临时暂停 gtmux 后台服务、迁移数据并恢复原设置？[y/N] ")) {
		if len(plan) > 0 && !pending {
			if err := restoreDoctorStorageServices(manager, plan); err != nil {
				return guidedStorageFailure(err)
			}
		}
		return 0
	}
	if plan == nil && len(jobs) > 0 {
		if err := saveStorageServicePlan(jobs); err != nil {
			return guidedStorageFailure(err)
		}
		plan = jobs
	}
	i18n.Say("Pausing managed gtmux services.", "正在暂停 gtmux 后台服务。")
	changeErr := manager.Pause(plan, processes)
	if changeErr == nil {
		changeErr = state.ChangeWorkflowLayout(true, writers)
	}
	if err := state.CheckStorageLayout(); err != nil {
		i18n.Sae("Keep tmux closed and rerun gtmux doctor --fix to continue. The service recovery plan is preserved.",
			"请保持 tmux 关闭，重新运行 gtmux doctor --fix 继续。原有后台服务的恢复记录已保留。")
		return guidedStorageFailure(errors.Join(changeErr, err))
	}
	status, err = state.ReadStorageLayout()
	if err != nil {
		return guidedStorageFailure(errors.Join(changeErr, err))
	}
	if err := restoreDoctorStorageServices(manager, plan); err != nil {
		i18n.Sae("Run gtmux doctor --fix again to restore the recorded gtmux service settings.",
			"请重新运行 gtmux doctor --fix，恢复已记录的后台服务设置。")
		return guidedStorageFailure(errors.Join(changeErr, err))
	}
	if changeErr != nil {
		return guidedStorageFailure(changeErr)
	}
	renderSections([]dsection{{i18n.Tr("Storage", "数据目录"), []dcheck{workflowLayoutRow(status, nil)}}})
	i18n.Say("Storage is ready. You can open the menu bar app and run gtmux restore.",
		"数据目录已就绪。可以打开菜单栏 App，并运行 gtmux restore 恢复会话。")
	return 0
}

func previewDoctorWorkflow(grouped bool) error {
	stores, err := state.InspectWorkflowPaths()
	if err != nil {
		return err
	}
	if grouped {
		i18n.Say("Move HQ wake data and global lease files into the workflow layout.",
			"这次分组会迁移 HQ 唤醒数据和全局锁文件。")
	} else {
		i18n.Say("Return HQ wake data and global lease files to their previous paths.",
			"这次回退会将 HQ 唤醒数据和全局锁文件移回原路径。")
	}
	for _, store := range stores {
		from, to := store.Original, store.Grouped
		if !grouped {
			from, to = to, from
		}
		fmt.Printf("  %s → %s\n", from, to)
	}
	return nil
}
