package transcript

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// CodexIdentity is persisted thread identity, not current client ownership.
type CodexIdentity struct {
	ThreadID       string
	HomeID         string
	CreationClient string
}

func canonicalHome(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}

// CodexHomeID is opaque on the wire. Server identity remains the caller's scope.
func CodexHomeID() string {
	sum := sha256.Sum256([]byte(canonicalHome(codexHome())))
	return hex.EncodeToString(sum[:16])
}

func DefaultCodexHome() bool {
	return canonicalHome(codexHome()) == canonicalHome(filepath.Join(state.Home(), ".codex"))
}

func CodexIdentityFor(id string) (CodexIdentity, bool) {
	return codexIdentityFor(directCodexPathResolver{}, id)
}

// CodexIdentityFor resolves identity using this gather's path scope.
func (s *CodexPathScope) CodexIdentityFor(id string) (CodexIdentity, bool) {
	return codexIdentityFor(s, id)
}

func codexIdentityFor(resolver codexPathResolver, id string) (CodexIdentity, bool) {
	paths := resolver.paths(id)
	if len(paths) == 0 {
		return CodexIdentity{}, false
	}
	meta := readCodexSessionMeta(paths[0])
	thread := meta.ID
	if thread == "" {
		thread = meta.SessionID
	} // legacy, never a modern tree-root alias
	if id == "" || thread != id {
		return CodexIdentity{}, false
	}
	client := ""
	switch meta.Originator {
	case "codex-tui":
		client = "terminal"
	case "codex_work_desktop", "Codex Desktop":
		client = "chatgpt_desktop"
	}
	return CodexIdentity{ThreadID: thread, HomeID: CodexHomeID(), CreationClient: client}, true
}
