package app

import (
	"fmt"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func rowLegacyProfileLocks() dcheck {
	label := i18n.Tr("legacy profile locks", "旧版参与者资料锁")
	count, err := state.LegacyProfileLockCount()
	if err != nil {
		return dcheck{stRec, label, i18n.Tr("could not count files", "无法统计文件"), err.Error()}
	}
	if count == 0 {
		return dcheck{stOK, label, "0", ""}
	}
	return dcheck{stRec, label, fmt.Sprint(count), i18n.Tr(
		"After upgrading or stopping every hook and process that can still run the old writer, run `gtmux doctor --prune-profile-locks`. Profile data is kept.",
		"升级或停用所有仍可能运行旧版写入逻辑的 hook 和进程后，运行 `gtmux doctor --prune-profile-locks`。资料快照会保留。")}
}

func doctorPruneProfileLocks(confirmRetired func(string) bool) int {
	count, err := state.LegacyProfileLockCount()
	if err != nil {
		i18n.Sae("Could not count legacy profile locks: "+err.Error(), "无法统计旧版参与者资料锁："+err.Error())
		return 1
	}
	if count == 0 {
		i18n.Say("No legacy profile locks to remove.", "没有待清理的旧版参与者资料锁。")
		return 0
	}
	fmt.Println(i18n.Tr(
		fmt.Sprintf("Found %d empty legacy profile lock files. Cleanup keeps profile data and current shard locks.", count),
		fmt.Sprintf("发现 %d 个空的旧版参与者资料锁文件。清理会保留资料快照和新版分组锁。", count)))
	if !confirmRetired(i18n.Tr(
		"Have you upgraded or stopped all legacy writers, including in-flight hooks? Confirm to remove these files [y/N] ",
		"是否已升级或停用所有旧版写入方，并等待执行中的旧版 hook 退出？确认后清理这些文件 [y/N] ")) {
		i18n.Say("Cleanup skipped. Confirm writer retirement in an interactive terminal to proceed.", "已跳过清理。请在交互终端确认旧版写入方已停用后再执行。")
		return 0
	}
	removed, busy, err := state.PruneLegacyProfileLocks()
	fmt.Println(i18n.Tr(fmt.Sprintf("Removed %d legacy profile locks.", removed), fmt.Sprintf("已清理 %d 个旧版参与者资料锁。", removed)))
	if busy > 0 {
		fmt.Println(i18n.Tr(
			fmt.Sprintf("Kept %d held locks. Retire the remaining legacy writers and retry.", busy),
			fmt.Sprintf("保留了 %d 个仍被持有的锁。停用剩余旧版写入方后重试。", busy)))
	}
	if err != nil {
		i18n.Sae("Legacy profile lock cleanup stopped: "+err.Error(), "旧版参与者资料锁清理中止："+err.Error())
	}
	if busy > 0 || err != nil {
		return 1
	}
	return 0
}
