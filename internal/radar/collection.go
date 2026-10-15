package radar

import (
	"sync"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Collection owns discovery for one operation, never a cached fleet or its facts.
// A sequence of ordered sensors can reuse it and still sample fresh mutable state.
type Collection struct {
	once  sync.Once
	paths *transcript.CodexPathScope
}

func NewCollection() *Collection { return &Collection{} }

func (c *Collection) codexPaths() *transcript.CodexPathScope {
	c.once.Do(func() { c.paths = transcript.NewCodexPathScope() })
	return c.paths
}

func (c *Collection) Agents() []Pane {
	panes, _ := gatherAgentsWithCodexScope(c.codexPaths())
	return panes
}
