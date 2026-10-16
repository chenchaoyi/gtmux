package app

import (
	"fmt"
	"time"

	"github.com/chenchaoyi/gtmux/internal/humanize"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func autoSaveCheck(statusRight, path string, receipt backstopReceipt, socket, pid string, now time.Time) dcheck {
	label := i18n.Tr("resurrect autosave", "resurrect 自动保存")
	switch n := continuumTriggerCount(statusRight); {
	case n > 1:
		return dcheck{stRec, label, fmt.Sprintf(i18n.Tr("%d save triggers", "%d 个保存触发器"), n),
			i18n.Tr("Keep one absolute-path continuum save trigger in status-right. Duplicate triggers can launch concurrent saves; doctor --fix leaves them unchanged for manual review.",
				"请在 status-right 中只保留一个使用绝对路径的 continuum 保存触发器。重复配置可能导致并发保存，doctor --fix 不会自动删除，需手动核对。")}
	case n == 0:
		return dcheck{stRec, label, i18n.Tr("trigger missing", "触发器缺失"),
			i18n.Tr("Run gtmux doctor --fix to review adding the save trigger without replacing existing status text.",
				"请运行 gtmux doctor --fix，确认后补充保存触发器；现有状态栏内容会保留。")}
	}
	snapshot, valid := fingerprintSave(path)
	if !valid {
		return dcheck{stRec, label, i18n.Tr("no readable session backup", "没有可读取的会话备份"),
			i18n.Tr("The trigger is configured, but no valid saved layout was found. Check the resurrect save directory and permissions, and keep gtmux serve running for fallback saves.",
				"已配置保存触发器，但未找到有效的会话布局备份。请检查 resurrect 保存目录和写入权限，并保持 gtmux serve 运行以提供备用保存。")}
	}
	age := now.Sub(time.Unix(0, snapshot.Modified))
	snapshotAge := humanize.AgeShort(int64(age.Seconds()))
	if receipt.matches(path, socket, pid, now) {
		checkAge := humanize.AgeShort(now.Unix() - receipt.At)
		return dcheck{stOK, label,
			fmt.Sprintf(i18n.Tr("fallback save completed %s ago", "备用保存成功 · %s 前"), checkAge),
			fmt.Sprintf(i18n.Tr("The save script completed successfully for this tmux server. The layout last changed %s ago; resurrect retains the existing snapshot when the layout is unchanged. This does not verify pane-content archives or restore results.",
				"此 tmux 服务的保存脚本已成功执行。布局快照最后更新于 %s 前；布局未变化时，resurrect 会保留原快照。此项未验证终端内容归档及实际恢复结果。"), snapshotAge)}
	}
	if age < 0 || age >= backstopArmedStaleAfter {
		return dcheck{stRec, label,
			fmt.Sprintf(i18n.Tr("snapshot unchanged for %s; save not confirmed", "快照 %s 未变化，近期保存未确认"), snapshotAge),
			i18n.Tr("An unchanged snapshot does not prove saving stopped. No recent successful save is recorded for this server and layout. Check continuum's status-right trigger and logs; keep gtmux serve running for fallback saves. Agent activity alone does not verify a completed save.",
				"快照未变化不代表停止保存；目前没有此服务和快照近期保存成功的回执。请检查 continuum 的 status-right 触发器及日志，并保持 gtmux serve 运行以提供备用保存。Agent 持续工作不能证明保存已完成。")}
	}
	return dcheck{stOK, label, i18n.Tr("installed", "已安装"),
		fmt.Sprintf(i18n.Tr("The save trigger is configured; a valid layout snapshot was updated %s ago.", "已配置保存触发器，有效的布局快照更新于 %s 前。"), snapshotAge)}
}
