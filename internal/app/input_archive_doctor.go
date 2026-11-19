package app

import (
	"strconv"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func rowInputArchive() dcheck { return inputArchiveDoctorRow(agentclient.InspectInputArchive()) }

func inputArchiveDoctorRow(h agentclient.InputArchiveHealth) dcheck {
	label := i18n.Tr("input replay archive", "终端发送防重档案")
	switch {
	case h.NeedsRepair:
		return dcheck{stMiss, label, i18n.Tr("archive could not be verified", "无法验证防重档案"),
			i18n.Tr("Preserve the queue and recover the matching catalogue and pages from a verified backup. Affected IDs stay blocked. See Input store maintenance.",
				"保留队列，从已核验的备份恢复配套清单和数据页。受影响的 ID 继续拦截。见输入队列维护文档。")}
	case h.AdmissionReserved:
		return dcheck{stMiss, label, i18n.Tr("only settlement space remains", "仅剩结算空间"),
			i18n.Tr("New stable send IDs are paused. Preserve existing replay keys and plan expanded archive capacity before reopening intake.",
				"暂停接纳新的稳定发送 ID。保留已有防重键并安排归档扩容，再恢复接入。")}
	case h.RecoveryPending:
		return dcheck{stRec, label, i18n.Tr("retirement recovery pending", "回收对账待完成"),
			i18n.Tr("Keep the checkpoint for the next admission to reconcile. If it persists, follow Input store maintenance.",
				"保留检查点，等待下一次入队对账。如持续存在，请按输入队列维护文档核对。")}
	}
	used := 0
	if h.LimitBytes > 0 {
		used = h.UsedBytes * 100 / h.LimitBytes
	}
	value := strconv.Itoa(used) + "%"
	if used >= 80 {
		return dcheck{stRec, label, value,
			i18n.Tr("Back up the archive with its catalogue and plan capacity before the settlement reserve is reached.",
				"备份档案及配套清单，并在接近结算预留空间前安排容量。")}
	}
	return dcheck{stOK, label, value, i18n.Tr("Committed archive data is readable.", "已提交的防重档案可读取。")}
}
