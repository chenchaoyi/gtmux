package app

import (
	"errors"
	"fmt"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func rowInputSpool() dcheck {
	usage, err := agentclient.InspectInputSpool()
	return inputSpoolDoctorRow(usage, err)
}

func inputSpoolDoctorRow(u agentclient.InputSpoolUsage, err error) dcheck {
	label := i18n.Tr("terminal input queue", "终端发送队列")
	if errors.Is(err, agentclient.ErrInputFull) {
		value := fmt.Sprintf(i18n.Tr("at least %d entries · scan limit %d", "至少 %d 个目录项 · 扫描上限 %d"), u.Entries, u.ScanLimit)
		return dcheck{stMiss, label, value, i18n.Tr(
			"New input is blocked. Preserve pending input and replay records and follow Input store maintenance.",
			"新请求已被拦截。请保留未决输入和防重记录，按输入队列维护文档核查。")}
	}
	if err != nil {
		return dcheck{stMiss, label, i18n.Tr("could not read the active queue", "无法读取活动队列"), i18n.Tr(
			"Verify the queue layout and permissions.",
			"请核对队列布局和读取权限。")}
	}
	value := fmt.Sprintf(i18n.Tr("%d/%d slots · %d entries", "%d/%d 槽 · %d 个目录项"), u.UsedSlots, u.SlotLimit, u.Entries)
	if u.UsedSlots+u.Reservation > u.SlotLimit {
		return dcheck{stMiss, label, value, i18n.Tr(
			"New input is blocked by settlement reservations. Preserve pending input and replay records and follow Input store maintenance.",
			"结算预留使新请求无法入队。请保留未决输入和防重记录，按输入队列维护文档核查。")}
	}
	if u.UsedSlots*5 >= u.SlotLimit*4 {
		return dcheck{stRec, label, value, i18n.Tr(
			"Review unresolved requests and verified settlements before the queue fills.",
			"请在队列占满前核对未决请求和已确认的结算。")}
	}
	return dcheck{stOK, label, value, i18n.Tr(
		"Slot usage includes space reserved for claims and receipts.",
		"槽位占用包含领取和回执的预留空间。")}
}
