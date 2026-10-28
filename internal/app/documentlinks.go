package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/artifact"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// The request names a link, not permission to read an arbitrary source path.
func resolveConversationDocument(reference, session, home string) (artifact.File, error) {
	existing, existingErr := artifact.Resolve(reference, session, home)
	if strings.HasPrefix(reference, "gtmux-file:") {
		return existing, existingErr
	}
	if !sessionpolicy.ValidRequest(session, home) {
		return artifact.File{}, artifact.ErrUnavailable
	}
	agent, cwd, ok := documentConversation(session, home)
	if !ok {
		return existing, existingErr
	}
	target, ok := artifact.LocalPath(reference, cwd)
	if !ok {
		return existing, existingErr
	}
	turns, err := transcript.Load(agent, session, 0)
	if err != nil {
		return existing, existingErr
	}
	linked := func(source string) bool {
		for _, link := range artifact.Links(source) {
			path, ok := artifact.LocalPath(link, cwd)
			if ok && path == target {
				return true
			}
		}
		return false
	}
	for _, turn := range turns {
		match := linked(turn.Prompt) || linked(turn.Response)
		for _, segment := range turn.Segments {
			if linked(segment.Text) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		file, err := artifact.Linked(target, session, home)
		if err == nil {
			return file, nil
		}
		if errors.Is(err, artifact.ErrRevoked) {
			return artifact.File{}, err
		}
		// An existing snapshot remains usable after its temporary source disappears.
		// Other failures must not silently show an older revision as the current file.
		if existingErr == nil && errors.Is(err, os.ErrNotExist) {
			return existing, nil
		}
		return artifact.File{}, err
	}
	return existing, existingErr
}

func documentConversation(session, home string) (agent, cwd string, ok bool) {
	if ref, err := sessionpolicy.Resolve(session, home); err == nil {
		agent = ref.Agent
		if agent == "codex" {
			if identity, found := transcript.CodexIdentityFor(session); found {
				cwd = identity.Cwd
			}
		}
		if agent == "dsh" {
			if value, err := dsh.Sessions.Resolve(session); err == nil {
				cwd = value.Cwd
			}
		}
		return agent, cwd, true
	}
	if home != "" {
		return "", "", false
	}
	agent = transcript.AgentForSession(session)
	if agent == "" || agent == "codex" {
		return "", "", false
	}
	// Non-native readers still verify identity in their provider-owned log layout.
	for _, record := range resume.All() {
		if record.SessionID == session && record.Agent == agent && record.HomeID == home && filepath.IsAbs(record.Cwd) {
			if cwd != "" && cwd != record.Cwd {
				return agent, "", true
			}
			cwd = record.Cwd
		}
	}
	return agent, cwd, true
}
