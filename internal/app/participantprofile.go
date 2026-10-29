package app

import (
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/agentprofile"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func participantProfile(pane, sid, home, at string) (any, error) {
	agent, client := "", ""
	if pane != "" {
		if tmux.Bin == "" || tmux.Display(pane, "#{pane_id}") == "" {
			return nil, sessionpolicy.ErrUnverified
		}
		loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
		r, ok := resume.LoadLive(loc)
		if a, bound := agentclient.ForPane(pane); bound {
			r, ok = resume.Record{Agent: a.Agent, SessionID: a.SessionID, HomeID: a.HomeID, TerminalClient: a.Client}, true
		} else if ok && r.Agent == "dsh" {
			return nil, sessionpolicy.ErrUnverified
		}
		if !ok {
			return nil, sessionpolicy.ErrUnverified
		}
		if home != "" && home != r.HomeID {
			return nil, sessionpolicy.ErrUnverified
		}
		agent, client, home = r.Agent, r.TerminalClient, r.HomeID
		if sid == "" {
			sid = r.SessionID
		} else if sid != r.SessionID {
			if hqpane.Find() != pane || at == "" {
				return nil, sessionpolicy.ErrUnverified
			}
			id := r.SessionID
			found := false
			for i := 0; i < 3; i++ {
				prev, a := events.HQSessionPredecessorLink(id, time.Now().Unix())
				if prev == "" {
					break
				}
				if prev == sid {
					agent = a
					if agent == "" {
						agent = transcript.AgentForSession(prev)
					}
					found = agent != ""
					break
				}
				id = prev
			}
			if !found {
				return nil, sessionpolicy.ErrUnverified
			}
			home, client = "", ""
		}
	} else {
		ref, err := sessionpolicy.Resolve(sid, home)
		if err != nil {
			return nil, err
		}
		agent, home = ref.Agent, ref.HomeID
	}
	p := agentprofile.New(agent, sid, at != "")
	from, until := int64(0), int64(0)
	if at != "" {
		turns, err := transcript.Load(agent, sid, maxTranscriptTurns)
		if err != nil {
			return nil, err
		}
		from, until, err = profileTurnInterval(turns, at)
		if err != nil {
			return nil, err
		}
	}
	var paths []string
	if agent == "codex" {
		paths = transcript.NewCodexPathScope().Paths(sid)
	} else {
		paths = []string{transcript.LogPath(agent, sid)}
	}
	if agent == "dsh" {
		s, e := dsh.Sessions.Resolve(sid)
		if e != nil || s.HomeID != home {
			return nil, sessionpolicy.ErrUnverified
		}
		paths = []string{s.Path}
	}
	agentprofile.ReadLogs(&p, paths, from, until)
	if at == "" {
		agentprofile.Current(&p, home)
		if client != "" {
			p.Facts = append(p.Facts, agentprofile.Fact{Key: "client", Value: client, Source: "verified_binding"})
		}
	}
	return p, nil
}

// A timestamp must identify exactly one loaded turn. Ambiguity must not attach
// another turn's configuration to the selected message.
func profileTurnInterval(turns []transcript.Turn, at string) (int64, int64, error) {
	tm, err := time.Parse(time.RFC3339Nano, at)
	if err != nil || tm.UnixMilli() <= 0 {
		return 0, 0, sessionpolicy.ErrUnverified
	}
	match := -1
	for i, t := range turns {
		if t.Time == at {
			if match >= 0 {
				return 0, 0, sessionpolicy.ErrUnverified
			}
			match = i
		}
	}
	if match < 0 {
		return 0, 0, sessionpolicy.ErrUnverified
	}
	until := int64(0)
	if match+1 < len(turns) {
		next, err := time.Parse(time.RFC3339Nano, turns[match+1].Time)
		if err != nil || next.UnixMilli() <= tm.UnixMilli() {
			return 0, 0, sessionpolicy.ErrUnverified
		}
		until = next.UnixMilli()
	}
	return tm.UnixMilli(), until, nil
}
