package hq

import (
	"github.com/chenchaoyi/gtmux/internal/humanize"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
)

// The knowledge base lives in internal/knowledge (hq-knowledge-engine, phase 1). What
// stays here is supervision: the verbs that need the supervisor's own state (the distill
// cadence for the capture banner, the miner for `knowledge mine`) and the doctor row that
// judges the export queue against the maintenance verdict vocabulary.

// CmdKnowledge dispatches `gtmux knowledge <verb>`; `mine` is the supervisor's (it drives
// internal/mine), everything else is the ledger's.
func CmdKnowledge(args []string) int {
	if len(args) > 0 && args[0] == "mine" {
		return knowledgeMine(args[1:])
	}
	return knowledge.CmdKnowledge(args)
}

// CmdCapture implements `gtmux capture`, with the drain-liveness banner composed here.
func CmdCapture(args []string) int { return knowledge.CmdCapture(args, captureListHeader) }

// captureListHeader is the one-line "is the drain alive?" banner above the queue. The
// queue depth alone can't tell you whether the loop is alive: an empty queue reads
// identically whether distill drained it yesterday or has never run at all — which is
// exactly how a 13-day distill outage stayed invisible.
func captureListHeader(now int64) string {
	d, _ := MaintenanceStatus(now)
	switch d.State {
	case MaintenanceNever:
		return i18n.Tr("last distill: never run", "上次蒸馏:从未运行")
	case MaintenanceSlipped:
		return i18n.Tr("last distill: "+humanize.AgeShort(d.AgeSec)+" ago — SLIPPED past its weekly cadence",
			"上次蒸馏:"+humanize.AgeShort(d.AgeSec)+"前 —— 已滑过每周节拍")
	default:
		return i18n.Tr("last distill: "+humanize.AgeShort(d.AgeSec)+" ago",
			"上次蒸馏:"+humanize.AgeShort(d.AgeSec)+"前")
	}
}

// promotionStaleSecs is the doctor's staleness floor for a pending promotion: past it,
// "nobody carried the brief" is a flagged condition, not a note — because an un-carried
// flag is exactly the charter-flags rot the exit ends.
const promotionStaleSecs = 14 * 24 * 60 * 60

// PromotionsRow is the export queue's health verdict (consumed by `gtmux doctor`).
type PromotionsRow struct {
	Pending   int   // open promotions
	OldestSec int64 // age of the oldest, 0 when none
	State     MaintenanceState
}

// PromotionsStatus reports the export queue at `now`, judged against the maintenance
// verdict vocabulary the rest of doctor's HQ rows use.
func PromotionsStatus(now int64) PromotionsRow {
	n, oldest := knowledge.PendingPromotionsSummary(now)
	row := PromotionsRow{Pending: n, OldestSec: oldest, State: MaintenanceOK}
	if n > 0 && oldest >= promotionStaleSecs {
		row.State = MaintenanceSlipped
	}
	return row
}
