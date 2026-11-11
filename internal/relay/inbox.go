package relay

import (
	"context"
	"sort"
	"time"
)

// Notice is transport receipt for a short inbox hint, never for the reply body
// or proof that an agent read or acted on it.
type Notice struct {
	TurnID string `json:"turn_id"`
	State  string `json:"state"`
	At     int64  `json:"at"`
}

// Source is an executing conversation, not a reusable pane number alone.
type Source struct{ Agent, HomeID, ThreadID, Pane, Session, PanePID string }

func SourceMatches(r Request, s Source) bool {
	if r.Agent == "codex" || s.Agent == "codex" {
		return r.Agent == "codex" && s.Agent == "codex" && s.HomeID != "" && s.ThreadID != "" && r.HomeID == s.HomeID && r.ThreadID == s.ThreadID
	}
	return s.Pane != "" && s.PanePID != "" && s.Session != "" && r.Pane == s.Pane && r.PanePID == s.PanePID && r.Session == s.Session
}

const NoticeMaxAge = 15 * time.Minute

func NoticeEligible(r Request, now int64) bool {
	return r.State == "replied" && r.For == "hq" && r.ReplyChannel == "inbox" && r.Agent == "codex" && r.HomeID != "" && r.ThreadID != "" && r.ReplyAt > 0 && r.ReplyAt <= now && now-r.ReplyAt <= int64(NoticeMaxAge/time.Second) && r.FetchedAt == 0 && r.Notice == nil && !r.Delivered && r.ReplyAttempt == ""
}

// BeginNotice provides durable at-most-once coalescing across concurrent HQ
// replies. An unconfirmed write also occupies this exact active turn's notice.
func BeginNotice(id, turnID string) error {
	return BeginNoticeContext(context.Background(), id, turnID)
}
func BeginNoticeContext(ctx context.Context, id, turnID string) error {
	if turnID == "" || len(turnID) > 256 {
		return ErrState
	}
	return transactionContext(ctx, func() error {
		r, err := read(id)
		if err != nil {
			return err
		}
		if !NoticeEligible(r, time.Now().Unix()) {
			return ErrState
		}
		for _, prior := range List() {
			if prior.HomeID == r.HomeID && prior.ThreadID == r.ThreadID && prior.Notice != nil && prior.Notice.TurnID == turnID {
				return ErrState
			}
		}
		r.Notice = &Notice{TurnID: turnID, State: "submitting", At: time.Now().Unix()}
		return write(r)
	})
}
func FinishNotice(id string, accepted, attempted bool) error {
	return FinishNoticeContext(context.Background(), id, accepted, attempted)
}
func FinishNoticeContext(ctx context.Context, id string, accepted, attempted bool) error {
	return transactionContext(ctx, func() error {
		r, err := read(id)
		if err != nil {
			return err
		}
		if r.Notice == nil || r.Notice.State != "submitting" {
			return ErrState
		}
		if !attempted {
			r.Notice = nil
		} else if accepted {
			r.Notice.State = "accepted"
		} else {
			r.Notice.State = "unconfirmed"
		}
		return write(r)
	})
}
func Fetch(id string, s Source) (Request, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return FetchContext(ctx, id, s)
}
func FetchContext(ctx context.Context, id string, s Source) (Request, error) {
	var out Request
	err := transactionContext(ctx, func() error {
		r, err := read(id)
		if err != nil {
			return err
		}
		if !SourceMatches(r, s) {
			return ErrState
		}
		if r.State == "replied" && r.ReplyChannel == "inbox" && r.FetchedAt == 0 {
			r.FetchedAt = time.Now().Unix()
			if err = write(r); err != nil {
				return err
			}
		}
		out = r
		return nil
	})
	return out, err
}

// Inbox is newest-first and bounded. Legacy queued/delivered rows are available
// through explicit --all/show, without re-injecting or rewriting them.
func Inbox(s Source, all bool, limit int) []Request {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows := List()
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i].ReplyAt, rows[j].ReplyAt
		if a == 0 {
			a = rows[i].CreatedAt
		}
		if b == 0 {
			b = rows[j].CreatedAt
		}
		if a == b {
			return rows[i].ID < rows[j].ID
		}
		return a < b
	})
	out := make([]Request, 0, min(limit, len(rows)))
	for i := len(rows) - 1; i >= 0 && len(out) < limit; i-- {
		r := rows[i]
		if !SourceMatches(r, s) || r.State != "replied" || (!all && (r.ReplyChannel != "inbox" || r.FetchedAt != 0)) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Wait reads one source-owned request. It has no transport/input side effects;
// a timeout leaves the reply in the inbox for the next explicit pull.
func Wait(ctx context.Context, id string, s Source) (Request, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		r, err := Get(id)
		if err != nil {
			return Request{}, err
		}
		if !SourceMatches(r, s) {
			return Request{}, ErrState
		}
		if r.State == "replied" {
			return FetchContext(ctx, id, s)
		}
		if r.State == "closed" {
			return r, nil
		}
		select {
		case <-ctx.Done():
			return r, ctx.Err()
		case <-ticker.C:
		}
	}
}
