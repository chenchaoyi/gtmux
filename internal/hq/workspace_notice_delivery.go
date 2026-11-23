package hq

import (
	"fmt"
	"maps"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

const workspaceNoticePendingPrefix = "pending/"

// A pending key accompanies the legacy timestamp until the queue result is
// recorded. Old ledgers have timestamp keys only.
func reserveWorkspaceNotice(records map[string]int64, key string, at int64) {
	records[key] = at
	records[workspaceNoticePendingPrefix+key] = at
}

type workspaceNoticeRepair struct {
	key        string
	reservedAt int64
	value      int64
	keep       bool
}

func (r workspaceNoticeRepair) apply(records map[string]int64) {
	delete(records, workspaceNoticePendingPrefix+r.key)
	if r.keep {
		records[r.key] = r.value
	} else {
		delete(records, r.key)
	}
}

// The workspace reconciliation loop is the sole active owner. It retains only
// the most recent known write failure and cooldowns for current blockers.
var workspaceNoticeDelivery struct {
	path       string
	repair     *workspaceNoticeRepair
	blockers   map[string]int64
	unknownLog map[string]int64
}

func reconcileWorkspaceNoticeLedger(path string, records map[string]int64, alerts map[string]wakeNotice, readErr error) (map[string]int64, error) {
	delivery := &workspaceNoticeDelivery
	if delivery.path != path {
		delivery.path = path
		delivery.repair = nil
		delivery.blockers = make(map[string]int64)
		delivery.unknownLog = make(map[string]int64)
	}
	for key := range delivery.blockers {
		if _, active := alerts[key]; !active {
			delete(delivery.blockers, key)
		}
	}
	for key := range delivery.unknownLog {
		if _, active := alerts[key]; !active {
			delete(delivery.unknownLog, key)
		}
	}
	if r := delivery.repair; r != nil {
		if _, active := alerts[r.key]; !active {
			delivery.repair = nil
		} else if readErr == nil {
			current, exists := records[r.key]
			at, pending := records[workspaceNoticePendingPrefix+r.key]
			if pending && (at != r.reservedAt || !exists || current != r.reservedAt) ||
				!pending && (exists != r.keep || exists && current != r.value) {
				return records, fmt.Errorf("workspace notice changed while repairing %q", r.key)
			}
			repaired := make(map[string]int64, len(records))
			maps.Copy(repaired, records)
			r.apply(repaired)
			if err := saveWakeNotices(path, repaired); err != nil {
				return records, err
			}
			records = repaired
			delivery.repair = nil
		}
	}
	if readErr == nil {
		for marker, at := range records {
			key, pending := strings.CutPrefix(marker, workspaceNoticePendingPrefix)
			if !pending || !alerts[key].workspaceDecision || delivery.unknownLog[key] == at {
				continue
			}
			delivery.unknownLog[key] = at
			diag.For("hq").Act("act.wake.notice.recovery", "system", "workspace", diag.Refused,
				"HQ desktop submission is unknown; check the saved audit before retrying",
				"reason", key, "attempt_at", at)
		}
	}
	return records, readErr
}
