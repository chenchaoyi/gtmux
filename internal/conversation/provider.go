// Package conversation resolves verified external histories by agent and data home.
// Reading public history is separate from current client ownership and consent.
package conversation

import (
	"errors"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

var ErrUnverified = errors.New("conversation identity is unavailable or ambiguous")

type Ref struct{ Agent, ID, HomeID, CreationClient string }

// Providers own artifact identity; manifests declare which capability is wired.
// A future external reader adds one adapter here, without changing API or policy.
type provider struct {
	home    func() string
	resolve func(string) (Ref, bool)
}

var providers = map[string]provider{
	"codex": {transcript.CodexHomeID, func(id string) (Ref, bool) {
		v, ok := transcript.CodexIdentityFor(id)
		return Ref{"codex", id, v.HomeID, v.CreationClient}, ok
	}},
	"dsh": {dsh.HomeID, func(id string) (Ref, bool) {
		v, err := dsh.Sessions.Resolve(id)
		return Ref{"dsh", id, v.HomeID, ""}, err == nil && v.Origin != "subagent" && v.HasMessages
	}},
}

// Resolve retains ID-only reads only while their identity remains unique across
// configured providers. An opaque home selects an adapter, never a caller path.
func Resolve(id, home string) (Ref, error) {
	if id == "" || len(id) > 256 || strings.ContainsAny(id, "*/?[]\\\x00") {
		return Ref{}, ErrUnverified
	}
	var found []Ref
	for _, m := range agents.All() {
		p, ok := providers[m.NativeReader]
		if !ok || home != "" && home != p.home() {
			continue
		}
		if v, ok := p.resolve(id); ok {
			found = append(found, v)
		}
	}
	if len(found) != 1 {
		return Ref{}, ErrUnverified
	}
	return found[0], nil
}
func ValidHome(home string) bool {
	if home == "" {
		return true
	}
	for _, m := range agents.All() {
		if p, ok := providers[m.NativeReader]; ok && home == p.home() {
			return true
		}
	}
	return false
}
func HomeID(agent string) string {
	if m, ok := agents.For(agent); ok {
		if p, ok := providers[m.NativeReader]; ok {
			return p.home()
		}
	}
	return ""
}
func Supports(agent string) bool {
	m, ok := agents.For(agent)
	if !ok {
		return false
	}
	_, ok = providers[m.NativeReader]
	return ok
}
func Verified(agent, id, home string) bool {
	r, err := Resolve(id, home)
	return err == nil && r.Agent == agent
}
