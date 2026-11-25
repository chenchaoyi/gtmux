package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
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
	if _, _, pending := status.PendingChange(); err == nil && !pending {
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

func applyDoctorStorageChange(workflow, grouped bool, writers func() error, confirmReady func(string) bool) int {
	if err := writers(); err != nil {
		i18n.Sae("Storage maintenance is blocked: "+err.Error(),
			"数据目录维护被阻止："+err.Error())
		return 1
	}
	if workflow {
		stores, err := state.InspectWorkflowPaths()
		if err != nil {
			i18n.Sae("Storage preview failed: "+err.Error(), "数据目录预览失败："+err.Error())
			return 1
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
	return workflowLayoutRow(status, err)
}

func rowWakeStorage() dcheck {
	usage, err := hqnudge.InspectStorage()
	label := i18n.Tr("HQ wake delivery", "HQ 唤醒投递")
	if err != nil {
		return dcheck{stMiss, label, i18n.Tr("inspection incomplete", "盘点未完成"), err.Error()}
	}
	value := fmt.Sprintf(i18n.Tr("%d queued · %d claimed · %d unknown · %d awaiting proof",
		"%d 待投递 · %d 已领取 · %d 结果未知 · %d 待送达核实"),
		usage.Queued, usage.Claimed, usage.Uncertain, usage.AwaitingProof)
	if usage.Unclassified > 0 {
		return dcheck{stRec, label, value,
			fmt.Sprintf(i18n.Tr("%d unclassified entries kept in %s. Verify their owner and delivery evidence.",
				"%d 个未分类条目保留在 %s。请核对归属和投递证据。"), usage.Unclassified, state.WorkflowPath("hq-nudges"))}
	}
	if usage.Uncertain > 0 {
		return dcheck{stRec, label, value,
			i18n.Tr("Unknown outcomes remain on hold. Check delivery evidence by batch before retrying.",
				"结果未知的原件继续保留。重试前先按批次核对送达证据。")}
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
