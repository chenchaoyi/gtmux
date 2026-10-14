package app

import (
	"sync/atomic"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// Verified delivery can wait for a receipt. Keep it off the feed's fast tick so
// lifecycle detection, notification routing and HTTP/SSE service keep progressing.
type workspaceRunner struct{ running atomic.Bool }

func (r *workspaceRunner) start(work func()) {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	go func() { defer r.running.Store(false); work() }()
}

var workspaceBackground workspaceRunner

func scheduleWorkspaceWork() {
	if workspaceBackground.running.Load() {
		return
	}
	s := workspaceStore()
	d, err := s.Read()
	if err != nil {
		diag.ReportStoreFailure("hq-workspace", err)
		return
	}
	pending := false
	for _, m := range d.Messages {
		if m.Delivery == "pending" || m.Delivery == "submitting" {
			pending = true
			break
		}
	}
	if !pending {
		for _, t := range dispatch.ListTasks() {
			if t.MatterID != "" && !t.MatterLinked && (t.Delivered || t.State == string(dispatch.StateQueued)) {
				pending = true
				break
			}
		}
	}
	if !pending {
		return
	}
	workspaceBackground.start(func() {
		if _, err := pumpWorkspace(s, time.Now().Unix()); err != nil {
			diag.ReportStoreFailure("hq-workspace", err)
		}
		if err := reconcileWorkspaceTasks(s); err != nil {
			diag.ReportStoreFailure("hq-workspace-membership", err)
		}
	})
}
