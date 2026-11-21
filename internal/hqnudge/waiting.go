package hqnudge

import (
	"encoding/json"
	"errors"
	stdio "io"
	"os"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/ansi"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Older drains only admit plain .txt files. Keep structured entries out of
// those drains so provenance is never forwarded as prompt text.
const waitingWakeSuffix = ".hqwait"

type waitingServer struct {
	Socket string
	PID    string
	Start  int64
}

type waitingTerminal struct {
	Pane    string
	PanePID string
	Session string // tmux session name, distinct from the conversation ID
	Loc     string
	Dead    bool
	InMode  bool
}

type waitingConversation struct {
	Agent, Home, Session, Instance string
}

// WaitingSource is the verified origin of an ordinary waiting notification.
// A caller can carry a zero value when it cannot establish that origin.
type WaitingSource struct {
	Server waitingServer
	Origin waitingTerminal
	Owner  waitingConversation
	Task   string
}

type waitingWake struct {
	Line   string        `json:"line"`
	Source WaitingSource `json:"source"`
	Kind   string        `json:"kind"`
	Round  int64         `json:"round"` // waiting marker write, Unix nanoseconds
	Turn   string        `json:"turn,omitempty"`
}

const waitingPaneFormat = "#{socket_path}\t#{pid}\t#{start_time}\t#{pane_id}\t#{pane_pid}\t#{session_name}\t#{session_name}:#{window_index}.#{pane_index}\t#{pane_dead}\t#{pane_in_mode}"

var waitingTmux = tmux.Run
var waitingOwner = liveWaitingConversation
var waitingTask = dispatch.TaskForPane
var waitingCapture = tmux.CaptureFullColor
var waitingTurn = transcript.CodexLastTurnEvent

func readWaitingTerminal(row string) (waitingServer, waitingTerminal, bool) {
	f := strings.Split(row, "\t")
	if len(f) != 9 {
		return waitingServer{}, waitingTerminal{}, false
	}
	start, _ := strconv.ParseInt(f[2], 10, 64)
	s := waitingServer{Socket: f[0], PID: f[1], Start: start}
	p := waitingTerminal{Pane: f[3], PanePID: f[4], Session: f[5], Loc: f[6], Dead: f[7] == "1", InMode: f[8] == "1"}
	return s, p, s.Socket != "" && positiveNumber(s.PID) && s.Start > 0 &&
		strings.HasPrefix(p.Pane, "%") && positiveNumberOrZero(p.Pane[1:]) && positiveNumber(p.PanePID) &&
		p.Session != "" && p.Loc != "" && (f[7] == "0" || f[7] == "1") && (f[8] == "0" || f[8] == "1")
}

func positiveNumber(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n > 0
}

func positiveNumberOrZero(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}

func readWaitingPane(pane string) (waitingServer, waitingTerminal, bool) {
	row, err := waitingTmux("display-message", "-p", "-t", pane, waitingPaneFormat)
	s, p, ok := readWaitingTerminal(row)
	return s, p, err == nil && ok && p.Pane == pane
}

func liveWaitingConversation(p waitingTerminal) waitingConversation {
	if f, ok := resume.CodexFrontendForPane(p.Pane); ok && strconv.Itoa(f.PanePID) == p.PanePID && f.Loc == p.Loc {
		return waitingConversation{"codex", f.HomeID, f.SessionID, f.Instance}
	}
	if a, ok := agentclient.ForPane(p.Pane); ok {
		return waitingConversation{a.Agent, a.HomeID, a.SessionID, a.Instance}
	}
	return waitingConversation{}
}

func (c waitingConversation) inHome() bool {
	switch c.Agent {
	case "codex":
		return c.Home != "" && c.Home == transcript.CodexHomeID()
	case "dsh":
		return c.Home != "" && c.Home == dsh.HomeID()
	}
	return false
}

func WaitingSourceForPane(pane string) WaitingSource {
	s, p, ok := readWaitingPane(pane)
	if !ok || p.Dead {
		return WaitingSource{}
	}
	t, ok := waitingTask(pane)
	if !ok || t.ID == "" || t.Pane != pane || t.Session != p.Session || t.CreatedAt < s.Start {
		return WaitingSource{}
	}
	c := waitingOwner(p)
	if t.Agent != c.Agent || c.Session == "" || c.Instance == "" || !c.inHome() ||
		(t.MatterSessionID != "" && t.MatterSessionID != c.Session) ||
		(t.MatterHomeID != "" && t.MatterHomeID != c.Home) {
		return WaitingSource{}
	}
	return WaitingSource{Server: s, Origin: p, Owner: c, Task: t.ID}
}

// DeliverWaiting records the marker's episode while the exact source still
// owns it. Hook callers also supply the observed session and Home. A missing
// proof leaves the notification on the original plain-text route.
func DeliverWaiting(target, line string, src WaitingSource, kind, agent, home, session, turn string) {
	path := state.WaitingPath(src.Origin.Pane)
	round, ok := waitingRound(path, kind)
	if src.Task != "" && ok && round > 0 &&
		(agent == "" || (agent == src.Owner.Agent && home == src.Owner.Home && session == src.Owner.Session)) &&
		WaitingSourceForPane(src.Origin.Pane) == src {
		b, err := json.Marshal(waitingWake{Line: line, Source: src, Kind: kind, Round: round, Turn: turn})
		if err == nil {
			enqueueWithSuffix(prod, line, string(b), waitingWakeSuffix)
			if target != "" {
				withHQInput(func() { drain(prod, target) })
			}
			return
		}
	}
	if target == "" {
		Enqueue(line)
	} else {
		Deliver(target, line)
	}
}

// A missing marker only allows a later positive observation to be considered.
// A changed, unreadable or concurrently replaced marker cannot prove a clear.
func waitingRound(path, kind string) (int64, bool) {
	f, err := state.OpenPrivateRead(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, true
	}
	if err != nil {
		return 0, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > 256 {
		return 0, false
	}
	line, err := stdio.ReadAll(stdio.LimitReader(f, 257))
	current, statErr := os.Stat(path)
	if err != nil || statErr != nil || len(line) > 256 || !os.SameFile(info, current) || !current.ModTime().Equal(info.ModTime()) ||
		strings.TrimSpace(string(line)) != kind {
		return 0, false
	}
	return info.ModTime().UnixNano(), true
}

type waitingInventory struct {
	server waitingServer
	panes  map[string]bool
}

// An affirmative roster from the original server can prove that a pane ID has
// retired there. A missing server or a partial/foreign roster cannot.
func readWaitingInventory() waitingInventory {
	out, err := waitingTmux("list-panes", "-a", "-F", waitingPaneFormat)
	if err != nil || out == "" {
		return waitingInventory{}
	}
	v := waitingInventory{panes: map[string]bool{}}
	for _, row := range strings.Split(out, "\n") {
		s, p, ok := readWaitingTerminal(row)
		if !ok || (v.server != (waitingServer{}) && v.server != s) {
			return waitingInventory{}
		}
		v.server = s
		v.panes[p.Pane] = true
	}
	return v
}

func revalidateWaiting(w waitingWake, roster func() waitingInventory) string {
	src := w.Source
	if src.Task == "" || src.Owner.Session == "" || src.Owner.Instance == "" || !src.Owner.inHome() || w.Round <= 0 ||
		src.Server.Socket == "" || src.Server.PID == "" || src.Server.Start <= 0 ||
		src.Origin.Pane == "" || src.Origin.PanePID == "" || src.Origin.Loc == "" || w.Kind == "" {
		return ""
	}
	if inventory := roster(); inventory.server == src.Server && !inventory.panes[src.Origin.Pane] {
		return events.DropWaitSourceRetired
	}
	s, pane, ok := readWaitingPane(src.Origin.Pane)
	if !ok || s != src.Server || pane.PanePID != src.Origin.PanePID || pane.Loc != src.Origin.Loc {
		return ""
	}
	if pane.Dead {
		return events.DropWaitSourceRetired
	}
	path := state.WaitingPath(pane.Pane)
	round, known := waitingRound(path, w.Kind)
	if !known || (round != 0 && round != w.Round) || pane.InMode || waitingOwner(pane) != src.Owner {
		return ""
	}
	if !observedWaitClear(w) {
		return ""
	}
	s2, p2, known := readWaitingPane(pane.Pane)
	lastRound, markerKnown := waitingRound(path, w.Kind)
	if known && s2 == s && p2 == pane && markerKnown && lastRound == round && waitingOwner(p2) == src.Owner {
		return events.DropWaitResolved
	}
	return ""
}

func observedWaitClear(w waitingWake) bool {
	if w.Source.Owner.Agent == "codex" && w.Turn != "" {
		t := waitingTurn(w.Source.Owner.Session)
		if !t.Ambiguous && t.TurnID == w.Turn && t.At.UnixNano() > w.Round &&
			(t.Kind == "task_complete" || t.Kind == "turn_aborted") {
			return true
		}
	}
	if w.Kind != "draft" && w.Kind != "startup" && !(w.Kind == "permission" && w.Source.Owner.Agent == "codex") {
		return false
	}
	frame := waitingCapture(w.Source.Origin.Pane)
	plain := ansi.Strip(frame)
	draft, recognized := dispatch.DraftOfColored(frame)
	agent := w.Source.Owner.Agent
	return recognized && strings.TrimSpace(draft) == "" &&
		prompt.IsComposerReady(plain, agent) && !prompt.IsStartupPicker(frame, agent) && prompt.WaitingOptions(plain) == nil
}

func readWakeLine(path string, data []byte) (waitingWake, error) {
	if !strings.HasSuffix(claimBase(path), waitingWakeSuffix) {
		return waitingWake{Line: strings.TrimSpace(string(data))}, nil
	}
	var w waitingWake
	err := json.Unmarshal(data, &w)
	return w, err
}
