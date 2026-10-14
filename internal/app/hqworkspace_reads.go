package app

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/panefocus"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

type workspaceCandidate struct {
	hqworkspace.Conversation
	Status     string `json:"status"`
	Since      int64  `json:"since"`
	Supervisor bool   `json:"supervisor,omitempty"`
	Icon       string `json:"icon,omitempty"`
}

type workspaceList struct {
	ActivityRevision int64                `json:"activity_revision"`
	Schema           int                  `json:"schema"`
	SourceRevision   string               `json:"source_revision"`
	Revision         uint64               `json:"revision"`
	Matters          []hqworkspace.Matter `json:"matters"`
	Conversations    []workspaceCandidate `json:"conversations"`
	References       []workspaceCandidate `json:"references"`
	Supervisor       *workspaceCandidate  `json:"supervisor,omitempty"`
}

func workspaceCandidates() []workspaceCandidate {
	rows := []workspaceCandidate{}
	for _, p := range radar.GatherAgents() {
		c, ok := workspaceConversation(p)
		if !ok {
			continue
		}
		if c.Name == "" {
			c.Name = p.Loc
		}
		rows = append(rows, workspaceCandidate{Conversation: c, Status: p.Status, Since: p.Since, Supervisor: p.Role() == "supervisor", Icon: radar.IconFor(p.Agent, nil)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Loc < rows[j].Loc })
	return rows
}

func workspaceSnapshot(s *hqworkspace.Store) (workspaceList, error) {
	d, err := s.Read()
	if err != nil {
		return workspaceList{}, err
	}
	matters := hqworkspace.Matters(d)
	for i := range matters {
		matters[i].Assessment = ""
	} // Assessment is a selected-detail read, not sidebar payload.
	out := workspaceList{Schema: hqworkspace.Schema, Revision: d.Revision, Matters: matters, Conversations: []workspaceCandidate{}, References: []workspaceCandidate{}}
	for _, c := range workspaceCandidates() {
		if c.Supervisor {
			copy := c
			out.Supervisor = &copy
		} else {
			out.Conversations = append(out.Conversations, c)
		}
	}
	seen := map[string]bool{}
	for _, c := range out.Conversations {
		seen[c.Agent+"/"+c.HomeID+"/"+c.SessionID] = true
	}
	if out.Supervisor != nil {
		seen[out.Supervisor.Agent+"/"+out.Supervisor.HomeID+"/"+out.Supervisor.SessionID] = true
	}
	titles := map[string]map[string]string{}
	for _, r := range resume.AllLocated() {
		if r.SessionID == "" || r.Agent == "" || r.Loc == "" || r.Agent == "codex" && r.HomeID != "" && r.HomeID != transcript.CodexHomeID() {
			continue
		}
		home := r.HomeID
		if r.Agent == "codex" && home == "" {
			home = transcript.CodexHomeID()
		}
		key := r.Agent + "/" + home + "/" + r.SessionID
		if seen[key] {
			continue
		}
		seen[key] = true
		if transcript.LogPath(r.Agent, r.SessionID) == "" {
			continue
		}
		if titles[r.Agent] == nil {
			titles[r.Agent] = transcript.SessionTitles(r.Agent)
		}
		name := titles[r.Agent][r.SessionID]
		if name == "" {
			name = filepath.Base(r.Cwd) + " · " + r.Agent + " · " + r.SessionID[:min(len(r.SessionID), 8)]
		}
		out.References = append(out.References, workspaceCandidate{Conversation: hqworkspace.Conversation{Agent: r.Agent, HomeID: home, SessionID: r.SessionID, Name: name, Loc: r.Loc}, Status: "historical", Since: r.UpdatedAt, Icon: radar.IconFor(r.Agent, nil)})
		if len(out.References) >= 200 {
			break
		}
	}
	out.ActivityRevision = events.CurrentSeq()
	source := ""
	if out.Supervisor != nil {
		c := out.Supervisor
		source += "/" + c.Agent + "/" + c.HomeID + "/" + c.SessionID + "/" + c.Status + "/" + transcript.LogRevision(c.Agent, c.SessionID)
	}
	// Include selected-matter sources too, independent of caller selection.
	targets := map[string]bool{}
	for _, m := range d.Messages {
		if m.Target.SessionID != "" {
			key := m.Target.Agent + "/" + m.Target.SessionID
			targets[key] = true
		}
	}
	keys := []string{}
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		agent, sid, _ := strings.Cut(key, "/")
		source += "/" + key + "/" + transcript.LogRevision(agent, sid)
	}
	sum := sha256.Sum256([]byte(source))
	out.SourceRevision = fmt.Sprintf("%x", sum[:])
	return out, nil
}

type workspaceTurn struct {
	transcript.Turn
	Signal bool `json:"signal,omitempty"`
}

type workspaceMessage struct {
	hqworkspace.Message
	Reply *transcript.Turn `json:"reply,omitempty"`
}

type workspaceDetailReport struct {
	MatterTitles               map[string]string    `json:"matter_titles,omitempty"`
	Schema                     int                  `json:"schema"`
	Matter                     *hqworkspace.Matter  `json:"matter,omitempty"`
	Messages                   []workspaceMessage   `json:"messages"`
	Turns                      []workspaceTurn      `json:"turns"`
	MessageEarlierAvailable    bool                 `json:"message_earlier_available"`
	TranscriptEarlierAvailable bool                 `json:"transcript_earlier_available"`
	EarlierAvailable           bool                 `json:"earlier_available"`
	EarlierBefore              uint64               `json:"earlier_before,omitempty"`
	Changes                    []hqworkspace.Change `json:"changes"`
	Events                     []events.Record      `json:"events"`
	Warnings                   []string             `json:"warnings"`
}

func workspaceDetail(s *hqworkspace.Store, scope string, before uint64, earlier int) (workspaceDetailReport, error) {
	d, err := s.Read()
	if err != nil {
		return workspaceDetailReport{}, err
	}
	out := workspaceDetailReport{Schema: hqworkspace.Schema, Messages: []workspaceMessage{}, Turns: []workspaceTurn{}, Changes: []hqworkspace.Change{}, Events: []events.Record{}, Warnings: []string{}}
	if scope == "activity" {
		out.MatterTitles = map[string]string{}
		for id, m := range d.Matters {
			out.MatterTitles[id] = m.Title
		}
		out.Changes = d.Changes
		if len(out.Changes) > 200 {
			out.Changes = out.Changes[len(out.Changes)-200:]
			out.Warnings = append(out.Warnings, "activity-limited")
		}
		out.Events = workspaceActivity(time.Now().Unix())
		return out, nil
	}
	matter := scope
	if scope == "global" {
		matter = ""
	} else {
		m, ok := d.Matters[scope]
		if !ok || m.MergedInto != "" {
			return out, hqworkspace.ErrNotFound
		}
		out.Matter = &m
	}
	messages, more := hqworkspace.MessagePage(d, matter, before, 60)
	out.MessageEarlierAvailable = more
	out.EarlierAvailable = more
	if more && len(messages) > 0 {
		out.EarlierBefore = messages[0].Seq
	}
	loaded := map[string][]transcript.Turn{}
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Speaker != "user" || m.Target.SessionID == "" || m.Delivery == "pending" || m.Delivery == "cancelled" {
			continue
		}
		key := m.Target.Agent + "/" + m.Target.HomeID + "/" + m.Target.SessionID
		if _, exists := loaded[key]; exists {
			continue
		}
		if len(loaded) >= 8 {
			out.Warnings = append(out.Warnings, "reply-source-limit")
			break
		}
		turns, loadErr := workspaceLoad(m.Target, 100)
		if loadErr != nil {
			out.Warnings = append(out.Warnings, "reply-source-unavailable")
		}
		loaded[key] = turns
	}
	replies := map[string]*transcript.Turn{}
	replyBytes := 0
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Speaker != "user" || m.Target.SessionID == "" || m.Delivery == "pending" || m.Delivery == "cancelled" {
			continue
		}
		key := m.Target.Agent + "/" + m.Target.HomeID + "/" + m.Target.SessionID
		if t := workspaceReply(m, loaded[key]); t != nil {
			t = workspaceDistinctReply(t, m.ID, d.Messages)
			if t == nil {
				continue
			}
			b, _ := json.Marshal(t)
			if len(b) > 256<<10 || replyBytes+len(b) > 2<<20 {
				out.Warnings = append(out.Warnings, "large-reply-original-only")
				continue
			}
			replies[m.ID] = t
			replyBytes += len(b)
		}
	}
	for _, m := range messages {
		out.Messages = append(out.Messages, workspaceMessage{Message: m, Reply: replies[m.ID]})
	}

	if scope == "global" {
		for _, c := range workspaceCandidates() {
			if !c.Supervisor {
				continue
			}
			if transcript.LogPath(c.Agent, c.SessionID) == "" {
				out.Warnings = append(out.Warnings, "reply-source-unavailable")
				break
			}
			var raw []byte
			var meta server.TranscriptMeta
			raw, meta, err = transcriptForPane(c.Pane, earlier)
			if err != nil {
				return out, err
			}
			var turns []transcript.Turn
			if err = json.Unmarshal(raw, &turns); err != nil {
				return out, err
			}
			out.Turns = workspaceClassifyTurns(workspaceUntrackedTurns(turns, d), events.Read(7*24*3600, time.Now().Unix()))
			if meta.Dropped > 0 {
				out.Warnings = append(out.Warnings, "source-tail-limited")
			}
			out.TranscriptEarlierAvailable = meta.EarlierAvailable && earlier < 20
			if meta.EarlierAvailable && earlier >= 20 {
				out.Warnings = append(out.Warnings, "source-tail-limited")
			}
			out.EarlierAvailable = out.EarlierAvailable || out.TranscriptEarlierAvailable
			break
		}
	}
	boundWorkspaceTurns(&out)
	return out, nil
}

func workspaceLoad(c hqworkspace.Conversation, limit int) ([]transcript.Turn, error) {
	if c.Agent == "codex" && c.HomeID != "" && c.HomeID != transcript.CodexHomeID() {
		return nil, errors.New("conversation belongs to another agent home")
	}
	if transcript.LogPath(c.Agent, c.SessionID) == "" {
		return nil, errors.New("conversation source unavailable")
	}
	return transcript.Load(c.Agent, c.SessionID, limit)
}

func workspaceReply(m hqworkspace.Message, turns []transcript.Turn) *transcript.Turn {
	// Full bounded marker, not a title/prompt fragment. A quoted marker in assistant
	// output cannot create a match because only the input prompt is examined.
	prefix := "[gtmux workspace message " + m.ID + "; "
	for i := len(turns) - 1; i >= 0; i-- {
		_, body, hasBody := strings.Cut(turns[i].Prompt, "\n")
		if strings.HasPrefix(turns[i].Prompt, prefix) && hasBody && strings.TrimSpace(body) == strings.TrimSpace(m.Body) {
			t := turns[i]
			t.Prompt = m.Body
			return &t
		}
	}
	return nil
}

func workspaceUntrackedTurns(turns []transcript.Turn, d hqworkspace.Database) []transcript.Turn {
	out := []transcript.Turn{}
	for _, t := range turns {
		tracked := false
		for _, m := range d.Messages {
			if workspaceReply(m, []transcript.Turn{t}) != nil {
				tracked = true
				break
			}
		}
		if !tracked {
			out = append(out, t)
		}
	}
	return out
}

func workspaceKnownConversation(c hqworkspace.Conversation) bool {
	if c.SessionID == "" || c.Agent == "" {
		return false
	}
	if c.Agent == "codex" && c.HomeID != "" && c.HomeID != transcript.CodexHomeID() {
		return false
	}
	for _, r := range resume.All() {
		if r.Agent == c.Agent && r.SessionID == c.SessionID && (r.HomeID == c.HomeID || r.Agent == "codex" && r.HomeID == "" && c.HomeID == transcript.CodexHomeID()) {
			return true
		}
	}
	for _, candidate := range workspaceCandidates() {
		if candidate.Conversation.Same(c) && !candidate.Supervisor {
			return true
		}
	}
	return false
}

func workspaceParticipant(s *hqworkspace.Store, matter, id string) (workspaceDetailReport, error) {
	d, err := s.Read()
	if err != nil {
		return workspaceDetailReport{}, err
	}
	m, ok := d.Matters[matter]
	if !ok {
		return workspaceDetailReport{}, hqworkspace.ErrNotFound
	}
	for _, a := range m.Participants {
		if a.ID != id || a.RemovedAt != 0 {
			continue
		}
		turns, err := workspaceLoad(a.Conversation, 200)
		if err != nil {
			return workspaceDetailReport{}, err
		}
		out := workspaceDetailReport{Schema: hqworkspace.Schema, Matter: &m, Messages: []workspaceMessage{}, Turns: []workspaceTurn{}, Changes: []hqworkspace.Change{}, Events: []events.Record{}, Warnings: []string{}}
		for _, t := range turns {
			at, err := time.Parse(time.RFC3339Nano, t.Time)
			if err != nil {
				out.Warnings = append(out.Warnings, "undated-turn-omitted")
				continue
			}
			if at.Unix() < a.From || a.Until != 0 && at.Unix() > a.Until {
				continue
			}
			t.Agent = a.Conversation.Agent
			out.Turns = append(out.Turns, workspaceTurn{Turn: t})
		}
		if len(turns) == 200 {
			out.Warnings = append(out.Warnings, "participant-history-limited")
		}
		boundWorkspaceTurns(&out)
		return out, nil
	}
	return workspaceDetailReport{}, hqworkspace.ErrNotFound
}

func workspaceFocus(s *hqworkspace.Store, matter, id string) error {
	d, err := s.Read()
	if err != nil {
		return err
	}
	m, ok := d.Matters[matter]
	if !ok {
		return hqworkspace.ErrNotFound
	}
	for _, a := range m.Participants {
		if a.ID != id || a.RemovedAt != 0 {
			continue
		}
		matches := []string{}
		for _, c := range workspaceCandidates() {
			if c.Conversation.Same(a.Conversation) {
				matches = append(matches, c.Pane)
			}
		}
		if len(matches) != 1 {
			return errors.New("conversation has no unique live pane; its original record remains readable")
		}
		return panefocus.FocusPaneByID(matches[0])
	}
	return hqworkspace.ErrNotFound
}

func workspaceActivity(now int64) []events.Record {
	out := []events.Record{}
	all := events.Read(7*24*3600, now)
	identities := map[string]bool{}
	for _, c := range workspaceCandidates() {
		if c.Supervisor {
			identities[c.SessionID] = true
		}
	}
	for _, e := range all {
		if e.Event == events.AuditEventHQSession {
			identities[e.AgentSession] = true
			identities[e.PreviousAgentSession] = true
		}
	}
	delete(identities, "")
	for _, e := range all {
		if events.IsSupervisorAct(e) || identities[e.AgentSession] && e.Pane != "" {
			out = append(out, e)
		}
	}
	if len(out) > 200 {
		out = out[len(out)-200:]
	}
	return out
}

func getenvWorkspacePane() string { return os.Getenv("TMUX_PANE") }

// Only a journalled batch with the same payload is folded as a system signal.
// User prose that quotes the protocol, or an unconfirmed wake, remains visible.
func workspaceClassifyTurns(turns []transcript.Turn, records []events.Record) []workspaceTurn {
	out := []workspaceTurn{}
	for _, t := range turns {
		signal := false
		tag := hqwake.BatchID(t.Prompt)
		if tag != "" {
			for _, r := range records {
				if r.Event == events.AuditEventWakeDelivered && hqwake.BatchID(r.Summary) == tag && strings.Join(strings.Fields(r.Summary), " ") == strings.Join(strings.Fields(t.Prompt), " ") {
					signal = true
					break
				}
			}
		}
		out = append(out, workspaceTurn{Turn: t, Signal: signal})
	}
	return out
}

func boundWorkspaceTurns(out *workspaceDetailReport) {
	kept := []workspaceTurn{}
	size := 0
	for i := len(out.Turns) - 1; i >= 0; i-- {
		b, err := json.Marshal(out.Turns[i])
		if err != nil || len(b) > 512<<10 {
			out.Warnings = append(out.Warnings, "large-reply-original-only")
			continue
		}
		if size+len(b) > 2<<20 {
			out.Warnings = append(out.Warnings, "source-tail-limited")
			break
		}
		size += len(b)
		kept = append(kept, out.Turns[i])
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	out.Turns = kept
}

// A structured progress report must not suppress later unstructured commentary.
// Remove exact duplicate public text only; tool evidence remains available.
func workspaceDistinctReply(turn *transcript.Turn, message string, messages map[string]hqworkspace.Message) *transcript.Turn {
	published := map[string]bool{}
	for _, m := range messages {
		if m.Speaker == "hq" && m.ReplyTo == message {
			published[strings.TrimSpace(m.Body)] = true
		}
	}
	if len(published) == 0 {
		return turn
	}
	copy := *turn
	if len(turn.Segments) == 0 {
		if published[strings.TrimSpace(copy.Response)] {
			return nil
		}
		return &copy
	}
	copy.Segments = nil
	texts := []string{}
	for _, segment := range turn.Segments {
		if published[strings.TrimSpace(segment.Text)] {
			segment.Text = ""
		}
		if segment.Text != "" {
			texts = append(texts, segment.Text)
		}
		if segment.Text != "" || len(segment.Steps) > 0 {
			copy.Segments = append(copy.Segments, segment)
		}
	}
	copy.Response = strings.Join(texts, "\n\n")
	if len(copy.Segments) == 0 {
		return nil
	}
	return &copy
}
