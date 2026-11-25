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

type storageServicePlan struct {
	Phase string
	Jobs  []userservice.StorageLaunchdJob
}

type storageServiceIO struct {
	sync    func(string) error
	remove  func(string) error
	barrier func() error
}

func systemStorageServiceIO() storageServiceIO {
	return storageServiceIO{sync: state.SyncPrivateDirectory, remove: os.Remove, barrier: state.SyncVerifiedStorageLayout}
}

func storageMaintenanceLease() (*state.Lease, error) {
	return state.TryLeaseIn(filepath.Join(state.Dir(), "locks"), "storage-maintenance")
}

func storageServicePlanPath() string {
	return filepath.Join(state.StorageServiceRecoveryDir(), "plan.json")
}

func storageServiceCleanupPath() string {
	return filepath.Join(state.StorageServiceRecoveryDir(), "cleanup-pending")
}

const storageServiceCleanupReceipt = "gtmux launchd recovery verified\n"

func readStorageServicePlan() (*storageServicePlan, error) {
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
	var plan storageServicePlan
	if err != nil || len(data) > limit || json.Unmarshal(data, &plan) != nil || len(plan.Jobs) == 0 {
		return nil, errors.New("the saved launchd maintenance plan needs review")
	}
	switch plan.Phase {
	case "pause", "restore", "verified":
	default:
		return nil, errors.New("the saved launchd maintenance phase needs review")
	}
	seen := make(map[string]bool)
	for _, job := range plan.Jobs {
		if job.Identity.Inode == 0 || seen[job.Label] ||
			plan.Phase == "verified" && job.Stage != "" && job.Stage != "done" && job.Stage != "changed" {
			return nil, errors.New("the saved launchd maintenance jobs need review")
		}
		seen[job.Label] = true
	}
	return &plan, nil
}

func (ops storageServiceIO) save(plan *storageServicePlan) error {
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err := state.WritePrivateFile(storageServicePlanPath(), data); err != nil {
		return err
	}
	// A durable file is insufficient if its newly created ancestors can
	// disappear on reboot. Persist each name before changing launchd.
	if err := ops.sync(filepath.Dir(state.StorageServiceRecoveryDir())); err != nil {
		return err
	}
	return ops.sync(state.Dir())
}

func (ops storageServiceIO) cleanup() error {
	if err := state.WritePrivateFile(storageServiceCleanupPath(), []byte(storageServiceCleanupReceipt)); err != nil {
		return err
	}
	if err := ops.remove(storageServicePlanPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ops.sync(state.StorageServiceRecoveryDir()); err != nil {
		return err
	}
	// The significant deletion is already durable. If a crash makes only
	// this marker reappear, the retry repeats cleanup and no service action.
	return os.Remove(storageServiceCleanupPath())
}

func finishStorageServiceCleanup(ops storageServiceIO) error {
	lease, err := storageMaintenanceLease()
	if err != nil {
		return err
	}
	defer lease.Close()
	file, err := state.OpenPrivateRead(storageServiceCleanupPath())
	if err != nil {
		return err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, int64(len(storageServiceCleanupReceipt)+1)))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if string(data) != storageServiceCleanupReceipt {
		return errors.New("the service cleanup receipt needs review")
	}
	plan, err := readStorageServicePlan()
	if err != nil {
		return err
	}
	if plan != nil && plan.Phase != "verified" {
		return errors.New("service recovery must finish before removing its plan")
	}
	if err := state.CheckStorageLayout(); err != nil {
		return err
	}
	return ops.cleanup()
}

func (ops storageServiceIO) finish(manager userservice.StorageLaunchd, plan *storageServicePlan) error {
	if plan == nil {
		return nil
	}
	if plan.Phase == "verified" {
		return ops.cleanup()
	}
	if err := ops.barrier(); err != nil {
		return err
	}
	plan.Phase = "restore"
	if err := ops.save(plan); err != nil {
		return err
	}
	save := func() error { return ops.save(plan) }
	if err := manager.ReturnProfiles(plan.Jobs, save); err != nil {
		return err
	}
	if err := state.ReleaseWorkflowServiceFence(); err != nil {
		return err
	}
	if err := manager.Restore(plan.Jobs, save); err != nil {
		return err
	}
	for _, job := range plan.Jobs {
		if job.Stage == "changed" {
			i18n.Say("Kept newer launchd settings for "+job.Label+".", "已保留 "+job.Label+" 更新后的设置。")
		}
	}
	plan.Phase = "verified"
	if err := ops.save(plan); err != nil {
		return err
	}
	return ops.cleanup()
}

func guidedStorageFailure(err error) int {
	i18n.Sae("Storage maintenance paused: "+err.Error(), "数据目录维护已暂停："+err.Error())
	return 1
}

func guidedDoctorStorageChange(manager userservice.StorageLaunchd, processes func([]int) error, writers func() error, confirm func(string) bool) int {
	return guidedDoctorStorageChangeWithIO(manager, processes, writers, confirm, systemStorageServiceIO())
}

func guidedDoctorStorageChangeWithIO(manager userservice.StorageLaunchd, processes func([]int) error, writers func() error, confirm func(string) bool, ops storageServiceIO) int {
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
	if status.WorkflowActive() || plan != nil && plan.Phase != "pause" {
		unlock, err := acquireRemoteServicesLock()
		if err != nil {
			return guidedStorageFailure(err)
		}
		defer unlock()
		if err := ops.finish(manager, plan); err != nil {
			return guidedStorageRecoveryFailure(err)
		}
		if plan != nil {
			if status.WorkflowActive() {
				i18n.Say("gtmux service recovery finished. The workflow layout is ready.", "gtmux 后台服务恢复已完成，v3 布局已就绪。")
			} else {
				i18n.Say("gtmux service recovery finished on the previous layout.", "gtmux 后台服务恢复已完成，数据仍使用原布局。")
			}
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
	if !status.ServicePreparation() && !pending {
		for _, job := range jobs {
			if job.Parked {
				return guidedStorageFailure(fmt.Errorf("parked profile %s has no verified storage fence and needs review", job.Label))
			}
		}
	}
	if err := checkLegacyMenuBar(manager); err != nil {
		return guidedStorageFailure(err)
	}
	if err := processes(userservice.StorageLaunchdPIDs(jobs)); err != nil {
		return guidedStorageFailure(err)
	}
	if err := previewDoctorWorkflow(true); err != nil {
		return guidedStorageFailure(err)
	}
	i18n.Say("If gtmux is set to open at login, turn it off in System Settings → General → Login Items before continuing, and keep the menu bar app closed.",
		"如果 gtmux 设置了登录时打开，请先在「系统设置 → 通用 → 登录项」停用，并保持菜单栏 App 关闭。")
	if !confirm(i18n.Tr(
		"After reboot and before opening or restoring tmux, with menu bar login disabled, temporarily pause the managed gtmux services, group storage, and restore them? [y/N] ",
		"请确认已重启、尚未打开或恢复 tmux，且已停用菜单栏登录启动。现在暂停 gtmux 后台服务、迁移并恢复服务？[y/N] ")) {
		if plan != nil && !pending {
			unlock, err := acquireRemoteServicesLock()
			if err != nil {
				return guidedStorageFailure(err)
			}
			defer unlock()
			if err := ops.finish(manager, plan); err != nil {
				return guidedStorageRecoveryFailure(err)
			}
		}
		return 0
	}
	unlock, err := acquireRemoteServicesLock()
	if err != nil {
		return guidedStorageFailure(err)
	}
	defer unlock()
	current, err := manager.Inspect()
	if err != nil {
		return guidedStorageFailure(err)
	}
	if !userservice.SameStorageLaunchdSnapshot(jobs, current) {
		return guidedStorageFailure(errors.New(i18n.Tr(
			"gtmux service settings changed while you were reviewing the plan. Rerun gtmux doctor --fix to review the new state",
			"确认期间 gtmux 服务设置发生变化。请重新运行 gtmux doctor --fix，核对新的状态")))
	}
	if err := processes(userservice.StorageLaunchdPIDs(current)); err != nil {
		return guidedStorageFailure(err)
	}
	if err := checkLegacyMenuBar(manager); err != nil {
		return guidedStorageFailure(err)
	}
	if plan == nil && len(current) > 0 {
		for _, job := range current {
			if job.Parked || job.Loaded && job.Override[0] == "disabled" {
				return guidedStorageFailure(fmt.Errorf("launchd profile %s needs review before pausing it", job.Label))
			}
		}
		plan = &storageServicePlan{Phase: "pause", Jobs: current}
	}
	var changeErr error
	if plan != nil {
		changeErr = ops.save(plan)
		if changeErr != nil {
			return guidedStorageRecoveryFailure(changeErr)
		}
		i18n.Say("Pausing managed gtmux services.", "正在暂停 gtmux 后台服务。")
		save := func() error { return ops.save(plan) }
		changeErr = manager.Stop(plan.Jobs, save)
		if changeErr == nil {
			changeErr = state.PrepareWorkflowServiceFence()
		}
		if changeErr == nil {
			changeErr = manager.Park(plan.Jobs, save)
		}
	}
	if changeErr == nil {
		changeErr = state.ChangeWorkflowLayout(true, writers)
	}
	if err := ops.finish(manager, plan); err != nil {
		return guidedStorageRecoveryFailure(errors.Join(changeErr, err))
	}
	if changeErr != nil {
		return guidedStorageFailure(changeErr)
	}
	status, err = state.ReadStorageLayout()
	if err != nil {
		return guidedStorageFailure(err)
	}
	renderSections([]dsection{{i18n.Tr("Storage", "数据目录"), []dcheck{workflowLayoutRow(status, nil)}}})
	i18n.Say("Storage is ready. You can open the menu bar app and run gtmux restore.",
		"数据目录已就绪。可以打开菜单栏 App，并运行 gtmux restore 恢复会话。")
	return 0
}

func checkLegacyMenuBar(manager userservice.StorageLaunchd) error {
	if err := manager.CheckMenuBar(); err != nil {
		return fmt.Errorf(i18n.Tr(
			"the legacy gtmux menu bar login item must be unloaded and disabled before migration: %w",
			"旧版安装器的 gtmux 菜单栏登录项须先卸载并停用，再迁移：%w"), err)
	}
	return nil
}

func guidedStorageRecoveryFailure(err error) int {
	i18n.Sae("Keep tmux closed and rerun gtmux doctor --fix to review the recorded maintenance.",
		"请保持 tmux 关闭，重新运行 gtmux doctor --fix，核对并继续已记录的维护步骤。")
	return guidedStorageFailure(err)
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
