package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/userservice"
	"howett.net/plist"
)

// Direct clients can still hold the old queue or socket directory open.
// Inspect only their registered directories. Never print file names or arguments.
func checkPurposeOpenHandles() error {
	args := []string{"-nP", "-a", "-u", strconv.Itoa(os.Getuid()), "-F", "p"}
	flagCount := len(args)
	for _, rel := range []string{"terminal-input", "terminal-clients", "capture-bridge", "sessions/terminal-input", "sessions/clients/terminal-clients", "services/screen-capture"} {
		path := filepath.Join(state.Dir(), rel)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("unexpected direct-writer directory: %s", path)
		}
		args = append(args, "+d", path)
	}
	if len(args) == flagCount {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", args...)
	out, err := cmd.Output()
	if err == nil {
		for _, record := range strings.Fields(string(out)) {
			if value, ok := strings.CutPrefix(record, "p"); ok {
				if pid, err := strconv.Atoi(value); err == nil && pid > 0 {
					return fmt.Errorf(i18n.Tr("process %d still has the terminal input or capture store open; quit the client and rerun Doctor", "进程 %d 仍打开着终端输入或截屏目录。请退出对应客户端，再运行 Doctor。"), pid)
				}
			}
		}
	}
	var exit *exec.ExitError
	if ctx.Err() == nil && errors.As(err, &exit) && exit.ExitCode() == 1 && len(exit.Stderr) == 0 && len(out) == 0 {
		return nil
	}
	return errors.New(i18n.Tr("could not verify open terminal input and capture handles with lsof; check lsof and rerun Doctor", "lsof 未能核实终端输入和截屏目录是否仍在使用。请确认 lsof 可用，再运行 Doctor。"))
}

func initializeNewPurposeStorage() error {
	if _, err := os.Lstat(state.Dir()); !os.IsNotExist(err) {
		return err
	}
	if err := purposeDesktopCompatibility(); err != nil {
		return err
	}
	return state.InitializePurposeStorage()
}

func purposeDesktopCompatibility() error {
	inst := agentInstallers["dsh"]
	bridge := filepath.Join(filepath.Dir(inst.configPath()), "terminal-dst", "bridge.mjs")
	if data, err := readDSHArtifact(bridge); err == nil {
		if string(data) != dshTerminalPluginTemplate {
			if _, err := os.Lstat(inst.configPath()); os.IsNotExist(err) && !retainedV203Bridge(data) {
				return fmt.Errorf(i18n.Tr("unknown terminal-only DSH bridge at %s; verify its source and update it through its original installer", "单独保留的 DSH 桥接文件来源未能确认（%s）。请核对来源，通过原安装方式更新。"), bridge)
			}
			return errors.New(i18n.Tr("update the installed DSH bridge with gtmux install refresh-owned --agent dsh", "请先运行 gtmux install refresh-owned --agent dsh 更新已安装的 DSH 桥接插件"))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if runtime.GOOS != "darwin" {
		return nil
	}
	for _, bundle := range []string{filepath.Join(state.Home(), "Applications", "Gtmux.app"), "/Applications/Gtmux.app"} {
		if _, err := os.Lstat(bundle); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		path := filepath.Join(bundle, "Contents", "Info.plist")
		var meta struct {
			CFBundleIdentifier string
			GtmuxStorageLayout int
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err == nil && (!info.Mode().IsRegular() || info.Size() > 1<<20) {
			err = errors.New("invalid menu bar bundle metadata")
		}
		if err == nil {
			err = plist.NewDecoder(f).Decode(&meta)
		}
		_ = f.Close()
		if err != nil {
			return err
		}
		if meta.CFBundleIdentifier != "com.gtmux.menubar" || meta.GtmuxStorageLayout != 4 {
			return fmt.Errorf(i18n.Tr("update the installed menu bar before migration (%s)", "请先更新此菜单栏 App，再迁移（%s）"), bundle)
		}
	}
	return nil
}

func purposeAdvice(status state.StorageLayoutStatus) (string, error) {
	rows, err := state.InspectPurposePlacement(status)
	if err != nil {
		return "", err
	}
	if status.Workflow == "workflow-grouped" {
		if err := state.CheckPurposeSource(); err != nil {
			return "", err
		}
	}
	if len(rows) == 0 {
		return "", nil
	}
	for _, row := range rows {
		if row.Conflict {
			return "", fmt.Errorf(i18n.Tr("destination exists: %s; keep both copies", "目标位置已存在：%s。请保留两处原件"), row.Destination)
		}
	}
	intro := i18n.Tr("Session data, HQ, attachments and services can be grouped by purpose. ", "可按用途整理会话、HQ、附件和服务数据。")
	if err := purposeDesktopCompatibility(); err != nil {
		return intro + err.Error(), nil
	}
	if runtime.GOOS == "darwin" {
		manager := doctorStorageLaunchd()
		jobs, err := manager.Inspect()
		if err == nil {
			err = manager.CheckPurposeExecutables(jobs)
		}
		if err != nil {
			return intro + err.Error(), nil
		}
	}
	if err := checkStorageMigrationWindow(); err != nil {
		return intro + i18n.Tr("Deferred: ", "暂缓：") + err.Error(), nil
	}
	return intro + i18n.Tr("After reboot, before opening tmux, run gtmux doctor --fix in a standalone terminal.", "请在重启后、打开 tmux 前，从独立终端运行 gtmux doctor --fix。"), nil
}

// Preserve the established service plan through reboots. The layout command
// holds its own exclusive admission lock and verifies idle writers again.
func doctorPurposeFix(status state.StorageLayoutStatus, manager userservice.StorageLaunchd) int {
	if tmux.InTmux() {
		return guidedStorageFailure(errors.New(i18n.Tr("use a standalone terminal after reboot, before opening tmux", "请在重启后、打开 tmux 前，从独立终端操作")))
	}
	lease, err := storageMaintenanceLease()
	if err != nil {
		return guidedStorageFailure(err)
	}
	defer lease.Close()
	plan, err := readStorageServicePlan()
	if err != nil {
		return guidedStorageFailure(err)
	}
	if plan != nil && !plan.Purpose {
		return guidedStorageFailure(errors.New("finish the previous service maintenance first"))
	}
	ops := systemStorageServiceIO()
	if plan != nil && (plan.Phase != "pause" || status.Workflow == "categories-active" || status.Workflow == "return-category-services" || status.Workflow == "return-workflow-services") {
		unlock, err := acquireRemoteServicesLock()
		if err != nil {
			return guidedStorageFailure(err)
		}
		defer unlock()
		if err := ops.finish(manager, plan); err != nil {
			return guidedStorageRecoveryFailure(err)
		}
		return 0
	}
	if err := purposeDesktopCompatibility(); err != nil {
		return guidedStorageFailure(err)
	}
	var jobs []userservice.StorageLaunchdJob
	if runtime.GOOS == "darwin" {
		jobs, err = manager.Inspect()
		if err != nil {
			return guidedStorageFailure(err)
		}
		if err := manager.CheckPurposeExecutables(jobs); err != nil {
			return guidedStorageFailure(err)
		}
		if err := checkLegacyMenuBar(manager); err != nil {
			return guidedStorageFailure(err)
		}
	}
	if err := checkStorageProcesses(userservice.StorageLaunchdPIDs(jobs)); err != nil {
		return guidedStorageFailure(err)
	}
	if status.Workflow == "workflow-grouped" {
		if err := state.CheckPurposeSource(); err != nil {
			return guidedStorageFailure(err)
		}
	}
	if plan == nil {
		for _, job := range jobs {
			if job.Parked || job.Loaded && job.Override[0] == "disabled" {
				return guidedStorageFailure(fmt.Errorf("review service %s before migration", job.Label))
			}
		}
	}
	if status.Mode != "purpose" && status.Continuing == "" && status.Workflow != "group-workflow" {
		rows, e := state.InspectPurposePlacement(status)
		if e != nil {
			return guidedStorageFailure(e)
		}
		for _, row := range rows {
			if row.Conflict {
				return guidedStorageFailure(fmt.Errorf(i18n.Tr("occupied destination: %s", "目标位置已存在：%s"), row.Destination))
			}
		}
		i18n.Say(fmt.Sprintf("Group %d existing stores by purpose. Old attachment links will still open.", len(rows)), fmt.Sprintf("将按用途整理 %d 个现有存储项。历史对话中的附件链接仍可打开。", len(rows)))
	} else {
		i18n.Say("Continuing a previously recorded storage change.", "这次将继续上次已记录的迁移。")
	}
	forward := !strings.Contains(status.Workflow, "return") && status.Workflow != "pause-categories"
	if !confirmRisky(i18n.Tr("After reboot and before restoring tmux, pause managed services, complete the recorded storage change, and restore services? [y/N] ", "确认已重启且尚未打开 tmux。现在按记录整理目录，并暂停、恢复相关后台服务？[y/N] ")) {
		if plan != nil {
			unlock, e := acquireRemoteServicesLock()
			if e != nil {
				return guidedStorageFailure(e)
			}
			defer unlock()
			if status.Workflow == "move-to-categories" {
				if e := state.CancelPurposePreparation(); e != nil {
					return guidedStorageRecoveryFailure(e)
				}
			} else if status.Workflow != "workflow-grouped" {
				return guidedStorageRecoveryFailure(errors.New("the recorded move is paused; services are still held"))
			}
			if e := ops.finish(manager, plan); e != nil {
				return guidedStorageRecoveryFailure(e)
			}
		}
		return 0
	}
	unlock, err := acquireRemoteServicesLock()
	if err != nil {
		return guidedStorageFailure(err)
	}
	defer unlock()
	if runtime.GOOS == "darwin" {
		now, e := manager.Inspect()
		if e != nil {
			return guidedStorageFailure(e)
		}
		if !userservice.SameStorageLaunchdSnapshot(jobs, now) {
			return guidedStorageFailure(errors.New("service settings changed; review the new settings with gtmux doctor --fix"))
		}
		if e := manager.CheckPurposeExecutables(now); e != nil {
			return guidedStorageFailure(e)
		}
		if e := checkLegacyMenuBar(manager); e != nil {
			return guidedStorageFailure(e)
		}
	}
	if err := checkStorageProcesses(userservice.StorageLaunchdPIDs(jobs)); err != nil {
		return guidedStorageFailure(err)
	}
	if err := checkPurposeOpenHandles(); err != nil {
		return guidedStorageFailure(err)
	}
	if plan == nil && len(jobs) > 0 {
		plan = &storageServicePlan{Phase: "pause", Purpose: true, Jobs: jobs}
		if err := ops.save(plan); err != nil {
			return guidedStorageRecoveryFailure(err)
		}
	}
	if plan != nil {
		save := func() error { return ops.save(plan) }
		if err := manager.Stop(plan.Jobs, save); err != nil {
			return guidedStorageRecoveryFailure(err)
		}
	}
	if err := checkPurposeOpenHandles(); err != nil {
		return guidedStorageRecoveryFailure(err)
	}
	offline := checkEarlyStorageWriters
	if plan != nil {
		offline = func() error { return checkStorageProcesses(nil) }
	}
	if err := advancePurposePrerequisites(status, offline); err != nil {
		return guidedStorageRecoveryFailure(err)
	}
	if plan != nil {
		if err := checkStorageProcesses(nil); err != nil {
			return guidedStorageRecoveryFailure(err)
		}
		err = state.ChangePurposeLayoutWithServices(forward, checkEarlyPurposeWriters, func() error { return manager.Park(plan.Jobs, func() error { return ops.save(plan) }) })
	} else {
		err = state.ChangePurposeLayout(forward, checkEarlyPurposeWriters)
	}
	if err != nil {
		if current, e := state.ReadStorageLayout(); e == nil && current.Workflow == "workflow-grouped" && plan != nil {
			if restore := ops.finish(manager, plan); restore != nil {
				return guidedStorageRecoveryFailure(errors.Join(err, restore))
			}
		}
		return guidedStorageRecoveryFailure(err)
	}
	if err := ops.finish(manager, plan); err != nil {
		return guidedStorageRecoveryFailure(err)
	}
	i18n.Say("Storage verified. You can reopen gtmux.", "目录已核验，可以重新打开 gtmux。")
	return 0
}

func checkEarlyPurposeWriters() error {
	if err := checkEarlyStorageWriters(); err != nil {
		return err
	}
	return checkPurposeOpenHandles()
}

func advancePurposePrerequisites(status state.StorageLayoutStatus, offline func() error) error {
	if status.Workflow != "" && status.Workflow != "group-workflow" {
		return nil
	}
	if status.Continuing != "" && status.Continuing != "grouped" {
		return errors.New("finish the recorded rollback before reorganizing storage")
	}
	if status.Mode == "flat" || status.Continuing == "grouped" {
		if err := state.ChangeStorageLayout(true, offline); err != nil {
			return err
		}
	}
	return state.ChangeWorkflowLayout(true, offline)
}
