// Package agentprofile projects observed participant facts, never control authority.
package agentprofile

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

type Fact struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
	At     int64  `json:"observed_at,omitempty"`
}

type Process struct {
	Name      string   `json:"name"`
	Path      string   `json:"path,omitempty"`
	Arguments []string `json:"arguments,omitempty"`
}

type Profile struct {
	Agent       string    `json:"agent"`
	Name        string    `json:"name"`
	SessionID   string    `json:"session_id"`
	Historical  bool      `json:"historical"`
	Facts       []Fact    `json:"facts"`
	Chain       []Process `json:"execution_chain,omitempty"`
	ChainAt     int64     `json:"chain_observed_at,omitempty"`
	Partial     bool      `json:"partial,omitempty"`
	Unavailable bool      `json:"unavailable,omitempty"`
}

// Only domain readers live here; identity and display names come from the registry.
var readers = map[string]bool{"claude": true, "codex": true, "dsh": true, "kimi": true, "opencode": true, "cursor": true, "gemini": true}

func New(agent, session string, historical bool) Profile {
	p := Profile{Agent: agent, Name: agent, SessionID: session, Historical: historical, Facts: []Fact{}, Partial: !readers[agent]}
	if a, ok := agents.For(agent); ok {
		p.Agent, p.Name = a.Key, a.Label
	}
	return p
}

var identifier = regexp.MustCompile(`^[\pL\pN][\pL\pN._/@:+ ()-]{0,159}$`)

func clean(s string) string {
	s = strings.TrimSpace(s)
	if !identifier.MatchString(s) || strings.Contains(s, "://") {
		return ""
	}
	for _, c := range s {
		if unicode.IsControl(c) || unicode.In(c, unicode.Cf) {
			return ""
		}
	}
	return s
}

func fact(key, value, source string, at int64) Fact { return Fact{key, clean(value), source, at} }

func (p *Profile) put(f Fact) {
	for i, old := range p.Facts {
		if old.Key == f.Key {
			if f.Value == "" {
				p.Facts = append(p.Facts[:i], p.Facts[i+1:]...)
			} else {
				p.Facts[i] = f
			}
			return
		}
	}
	if f.Value != "" {
		p.Facts = append(p.Facts, f)
	}
}

func stamp(s string) int64 {
	t, _ := time.Parse(time.RFC3339Nano, s)
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

type point struct {
	At    int64
	Facts []Fact
	End   bool
}

func apply(p *Profile, points []point, from, until int64) {
	// A reply's terminal boundary wins over a later idle selection in the same session.
	if from > 0 {
		for _, x := range points {
			if x.End && x.At >= from && (until == 0 || x.At < until) {
				until = x.At + 1
				break
			}
		}
	}
	for _, x := range points {
		if from > 0 && (x.At == 0 || until > 0 && x.At >= until) {
			continue
		}
		for _, f := range x.Facts {
			p.put(f)
		}
	}
}
