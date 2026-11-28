package radar

import (
	"slices"
	"sync"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Collection owns discovery and one agent snapshot for a single operation (such
// as one slow-tick sweep), while Digest/Usage still sample fresh transcript content.
type Collection struct {
	once       sync.Once
	paths      *transcript.CodexPathScope
	agentsOnce sync.Once
	panes      []Pane
}

func NewCollection() *Collection { return &Collection{} }

func (c *Collection) codexPaths() *transcript.CodexPathScope {
	c.once.Do(func() { c.paths = transcript.NewCodexPathScope() })
	return c.paths
}

func (c *Collection) Agents() []Pane {
	c.agentsOnce.Do(func() {
		c.panes, _ = gatherAgentsWithCodexScope(c.codexPaths())
	})
	return slices.Clone(c.panes)
}
