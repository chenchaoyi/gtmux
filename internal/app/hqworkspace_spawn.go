package app

import (
	"errors"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func validateSpawnMatter(s *hqworkspace.Store, id string) error {
	if id == "" {
		return nil
	}
	d, err := s.Read()
	if err != nil {
		return err
	}
	m, ok := d.Matters[id]
	if !ok || m.MergedInto != "" || m.State == "closed" {
		return errors.New("matter unavailable or closed; create or select an active matter before spawning")
	}
	return nil
}

// Exact successful dispatch metadata is the association intent. A hook binding
// must match this dispatch's saved session or nonce before confirming membership.
// No title/cwd match can attach a worker, and failure leaves the intent visible.
func linkWorkspaceTask(s *hqworkspace.Store, task dispatch.Task, candidates []workspaceCandidate, now int64) (bool, error) {
	if task.MatterID == "" || task.MatterLinked || !task.Delivered && task.State != string(dispatch.StateQueued) {
		return false, nil
	}
	for _, c := range candidates {
		if c.Supervisor || c.Pane != task.Pane || c.Agent != task.Agent {
			continue
		}
		rec, ok := resume.Load(c.Loc)
		if !ok || rec.SessionID != c.SessionID {
			continue
		}

		// A delayed binding must not pick a new occupant of a recycled pane.
		if task.MatterBinding != "" {
			receipt, ok := native.BindingFor(c.SessionID, c.HomeID)
			if !ok || receipt.ID != task.MatterBinding || receipt.Pane != task.Pane || receipt.PreparedAt < task.CreatedAt {
				continue
			}
		} else if task.MatterSessionID == "" || c.SessionID != task.MatterSessionID || c.HomeID != task.MatterHomeID {
			continue
		}
		d, err := s.Read()
		if err != nil {
			return false, err
		}
		m, ok := d.Matters[hqworkspace.CanonicalMatter(d, task.MatterID)]
		if !ok {
			return false, errors.New("dispatch matter no longer active")
		}
		a := hqworkspace.Association{ID: "dispatch-" + task.ID, Conversation: c.Conversation, Role: "execution", From: task.CreatedAt}
		if err := s.Associate(m.ID, a, m.Revision, now); err != nil {
			return false, err
		}
		task.MatterLinked = true
		if err := dispatch.AddTask(task); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func reconcileWorkspaceTasks(s *hqworkspace.Store) error {
	pending := []dispatch.Task{}
	for _, t := range dispatch.ListTasks() {
		if t.MatterID != "" && !t.MatterLinked && (t.Delivered || t.State == string(dispatch.StateQueued)) {
			pending = append(pending, t)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	candidates := workspaceCandidates()
	for _, t := range pending {
		if _, err := linkWorkspaceTask(s, t, candidates, time.Now().Unix()); err != nil {
			return err
		}
	}
	return nil
}
