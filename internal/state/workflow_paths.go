package state

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	workflowLayoutDir    = "layout-v3"
	workflowLayoutMarker = ".storage-layout-v3"
	workflowLeaseDir     = "locks/leases"
)

type workflowStore struct {
	destination string
	directory   bool
}

// These paths are private to the participating Go writers. Stable desktop
// paths, sockets and the terminal input spool retain their own contracts.
var workflowStores = map[string]workflowStore{
	"hq-nudges":                     {"layout-v3/hq/wake/delivery-queue", true},
	"hq-nudge-attempts":             {"layout-v3/hq/wake/submission-and-recovery/legacy-terminal-attempts", true},
	"hq-nudge-native-batches":       {"layout-v3/hq/wake/submission-and-recovery/native-batch-records", true},
	"hq-nudge-native-claims":        {"layout-v3/hq/wake/submission-and-recovery/native-queue-item-hardlinks", true},
	"hq-wake-accepted":              {"layout-v3/hq/wake/delivery-verification/accepted-awaiting-proof", true},
	"hqwake/accepted-archive.jsonl": {"layout-v3/hq/wake/delivery-verification/expired-proof-archive.jsonl", false},
	"hqwake":                        {"layout-v3/hq/wake/shared-coordination", true},
	".hq-migration-apply.lock":      {"locks/leases/.hq-migration-apply.lock", false},
}

var restoreWorkerLease = regexp.MustCompile(`^restore-worker-[0-9]+-[A-Za-z0-9_-]{16,64}$`)

func workflowGlobalLease(name string) bool {
	switch name {
	case "codex-frontend-binding", "conversation-messages", "dsh-account-usage",
		"notify-queue", "restore-closed-sessions", "restore-execution", "restore-jobs",
		"restore-native-attempts":
		return true
	}
	if port, ok := strings.CutPrefix(name, "serve-"); ok {
		value, err := strconv.ParseUint(port, 10, 16)
		return err == nil && strconv.FormatUint(value, 10) == port
	}
	return restoreWorkerLease.MatchString(name)
}

func workflowStoreFor(name string) (workflowStore, bool) {
	if store, ok := workflowStores[name]; ok {
		return store, true
	}
	lease, prefix := strings.CutPrefix(name, "lease-")
	lease, suffix := strings.CutSuffix(lease, ".lock")
	if prefix && suffix && workflowGlobalLease(lease) {
		return workflowStore{filepath.Join(workflowLeaseDir, name), false}, true
	}
	return workflowStore{}, false
}

// WorkflowPath resolves an explicitly registered workflow store. Participating
// commands hold the fixed storage lease and validate the layout before use.
func WorkflowPath(name string, descendants ...string) string {
	if store, ok := workflowStoreFor(name); ok {
		if purposeMarked(Dir()) {
			name = strings.TrimPrefix(store.destination, workflowLayoutDir+"/")
		} else if _, err := os.Lstat(filepath.Join(Dir(), workflowLayoutMarker)); !os.IsNotExist(err) {
			name = store.destination
		}
	}
	return filepath.Join(append([]string{Dir(), name}, descendants...)...)
}
