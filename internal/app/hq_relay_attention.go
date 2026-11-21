package app

import (
	"errors"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

const workspaceRelayAttentionCursor = "relay-attention-start"
const workspaceRelayAttentionHead = "workspace relay follow-up"

var workspaceReserveRelayAttention = hqnudge.ReserveRelayAttention
var workspaceGetAttentionRelay = relay.Get
var workspaceRelayAttentionNotice = hq.WatchRelayAttention

// Transport metadata established at relay entry, never report prose.
func workspaceManagedRelayAttention(r relay.Request) bool {
	return r.Kind == "report" && r.For == "hq" && !r.Blocking && r.Agent == "codex" &&
		r.HomeID != "" && r.ThreadID != "" && r.TaskID != "" && r.Pane != "" && r.PanePID != ""
}

func readPendingAttention(q hqworkspace.Reconciliation, now int64) (relay.Request, bool, error) {
	id := strings.TrimPrefix(q.Observation.Ref, "relay:")
	r, err := workspaceGetAttentionRelay(id)
	if err != nil {
		return relay.Request{}, false, err
	}
	source := hqworkspace.Conversation{Agent: r.Agent, HomeID: r.HomeID, SessionID: r.ThreadID}
	if r.ID != id || !workspaceManagedRelayAttention(r) || r.CreatedAt != q.Observation.OccurredAt ||
		!source.Same(q.Observation.Conversation) {
		return relay.Request{}, false, errors.New("relay attention source identity has changed")
	}
	return r, r.State == "pending" || r.State == "claimed" && now-r.ClaimedAt > 300, nil
}

func serviceWorkspaceRelayAttention(s *hqworkspace.Store, d hqworkspace.Database, now int64) {
	var notices []hq.RelayAttention
	checkedSlot := false
	for _, due := range hqworkspace.RelayAttentionQueue(d, now) {
		q := due.Reconciliation
		late := now-q.Observation.OccurredAt >= int64(2*time.Minute/time.Second)
		eligible := !checkedSlot && q.RelayAttentionAt == 0 && (due.SourceEnded || late)
		if !eligible && (!late || len(notices) >= 8) {
			continue
		}
		r, pending, verifyErr := readPendingAttention(q, now)
		if verifyErr != nil {
			diag.ReportStoreFailure("hq-relay-attention", verifyErr)
			if late && len(notices) < 8 {
				notices = append(notices, hq.RelayAttention{ID: strings.TrimPrefix(q.Observation.Ref, "relay:"),
					Since: q.Observation.OccurredAt, ReservedAt: q.RelayAttentionAt, Unverified: true})
			}
			continue
		}
		if !pending {
			continue
		}
		if eligible {
			checkedSlot = true
			_, err := workspaceReserveRelayAttention(workspaceRelayAttentionLine(r.ID), func() (bool, error) {
				recorded, err := s.ReserveRelayAttention(r.ID, now)
				if recorded && err == nil {
					q.RelayAttentionAt = now
				}
				return recorded, err
			})
			diag.ReportStoreFailure("hq-relay-attention", err)
		}
		if late && len(notices) < 8 {
			notices = append(notices, hq.RelayAttention{ID: r.ID, Since: r.CreatedAt, ReservedAt: q.RelayAttentionAt})
		}
	}
	workspaceRelayAttentionNotice(now, notices)
}

func workspaceRelayAttentionLine(id string) string {
	return hqwake.Line(hqwake.ClassTick, workspaceRelayAttentionHead, "relay:"+id, "pull: gtmux relay show "+id)
}

func workspaceAttentionLineID(line string) (string, bool) {
	fields := strings.Split(line, hqwake.FieldSep())
	if len(fields) != 3 || !strings.HasSuffix(strings.TrimSpace(fields[0]), " "+workspaceRelayAttentionHead) {
		return "", false
	}
	id := strings.TrimPrefix(strings.TrimSpace(fields[1]), "relay:")
	if line != workspaceRelayAttentionLine(id) {
		return "", true // recognized reserved class with invalid metadata
	}
	return id, true
}

func revalidateWorkspaceRelayAttention(s *hqworkspace.Store, now int64, id, line string) (bool, string) {
	d, err := s.Read()
	if err != nil {
		diag.ReportStoreFailure("hq-relay-attention", err)
		return false, ""
	}
	q, ok := hqworkspace.RelayAttentionPending(d, id, now)
	if !ok || q.RelayAttentionAt == 0 {
		return false, ""
	}
	if _, pending, err := readPendingAttention(q, now); !pending || err != nil {
		diag.ReportStoreFailure("hq-relay-attention", err)
		return false, ""
	}
	return true, line
}
