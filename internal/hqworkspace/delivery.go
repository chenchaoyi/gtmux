package hqworkspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

// Delivery is the adapter's verified verdict. SafeHold means NOTHING was typed;
// a draft refusal after paste is uncertain, not eligible for automatic replay.
type Delivery struct {
	State    string
	Evidence string
	SafeHold bool
}

type DeliveryIO struct {
	Resolve func() (Conversation, string)       // identity and preflight hold code
	Send    func(Conversation, string) Delivery // rechecks before paste and Enter
}

func Payload(m Message, title string) string {
	context := "global HQ conversation"
	if m.MatterID != "" {
		context = "matter " + m.MatterID + " (" + title + ")"
	}
	return fmt.Sprintf("[gtmux workspace message %s; %s]\n%s", m.ID, context, m.Body)
}

// Pump processes at most one FIFO instruction. UI readers never call it implicitly.
// The app calls it after Submit; serve's existing fast tick handles safe holds later.
func (s *Store) Pump(io DeliveryIO, at int64) (bool, error) {
	// A process lease spans the actual input attempt, not just the claim write.
	// A slow surviving sender cannot be displaced by a timeout in another process.
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return false, err
	}
	lease, err := os.OpenFile(filepath.Join(s.dir, ".delivery-lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, err
	}
	defer lease.Close()
	if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return false, nil
		}
		return false, err
	}
	defer syscall.Flock(int(lease.Fd()), syscall.LOCK_UN)
	var effectiveMatter string
	var selected Message
	var title string
	target, hold := io.Resolve()
	err = s.transaction(func(d *Database) error {
		for id, m := range d.Messages {
			if m.Delivery != "submitting" {
				continue
			}
			if at-m.AttemptAt < 120 {
				return nil
			} // one sender owns HQ input at a time
			m.Delivery, m.Reason = "uncertain", "sender-disappeared"
			changed(d, at, "delivery-uncertain", m.MatterID, id, m.Reason)
			d.Messages[id] = m
		}
		pending := []Message{}
		for _, m := range d.Messages {
			if m.Delivery == "pending" {
				pending = append(pending, m)
			}
		}
		sort.Slice(pending, func(i, j int) bool { return pending[i].Seq < pending[j].Seq })
		if len(pending) == 0 {
			return nil
		}
		var m Message
		for _, candidate := range pending {
			effectiveMatter = CanonicalMatter(*d, candidate.MatterID)
			if candidate.MatterID != "" && (effectiveMatter == "" || d.Matters[effectiveMatter].State == "closed") {
				if candidate.Reason != "matter-closed" {
					candidate.Reason = "matter-closed"
					changed(d, at, "delivery-held", candidate.MatterID, candidate.ID, candidate.Reason)
					d.Messages[candidate.ID] = candidate
				}
				continue
			}
			m = candidate
			break
		}
		if m.ID == "" {
			return nil
		}
		if hold != "" || target.SessionID == "" {
			if hold == "" {
				hold = "hq-offline"
			}
			if m.Reason != hold {
				m.Reason = hold
				changed(d, at, "delivery-held", m.MatterID, m.ID, hold)
				d.Messages[m.ID] = m
			}
			return nil
		}
		m.Delivery, m.Reason, m.Target, m.AttemptAt = "submitting", "", target, at
		changed(d, at, "delivery-attempt", m.MatterID, m.ID, target.SessionID)
		d.Messages[m.ID] = m
		selected = m
		if m.MatterID != "" {
			title = d.Matters[effectiveMatter].Title
		}
		return nil
	})
	if err != nil || selected.ID == "" {
		return false, err
	}
	payload := selected
	payload.MatterID = effectiveMatter
	verdict := io.Send(target, Payload(payload, title))
	err = s.transaction(func(d *Database) error {
		m, ok := d.Messages[selected.ID]
		if !ok {
			return ErrNotFound
		}
		if m.Delivery != "submitting" || !m.Target.Same(target) || m.AttemptAt != at {
			return ErrConflict
		}
		m.Evidence = verdict.Evidence
		if len(m.Evidence) > 4096 {
			m.Evidence = m.Evidence[:4096]
		}
		switch verdict.State {
		case "landed":
			m.Delivery, m.DeliveredAt = "delivered", at
		case "queued":
			m.Delivery, m.DeliveredAt = "accepted-queue", at
		default:
			if verdict.SafeHold {
				m.Delivery, m.Reason, m.AttemptAt = "pending", verdict.State, 0
			} else {
				m.Delivery, m.Reason = "uncertain", verdict.State
			}
		}
		changed(d, at, "delivery-"+m.Delivery, m.MatterID, m.ID, m.Reason)
		d.Messages[m.ID] = m
		return nil
	})
	return true, err
}
