package radar

import (
	"bytes"
	"slices"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const snapshotTTL = 1500 * time.Millisecond
const snapshotByteCap = 4 << 20

type snapshotCache struct {
	scope    func() string
	scopeKey string
	mu       sync.Mutex
	body     []byte
	panes    []Pane
	at       time.Time
	load     func() ([]byte, error)
	now      func() time.Time
}

var sharedSnapshot = &snapshotCache{load: AgentsJSONBytes, now: time.Now, scope: state.Home}

// CachedAgentsJSONBytes is the serve's one producer snapshot, shared by SSE and
// authenticated HTTP readers. Standalone CLI reads keep their fresh path.
func CachedAgentsJSONBytes() ([]byte, error) { return sharedSnapshot.read() }

// CachedAgents returns the recent Pane snapshot (within snapshotTTL) or refreshes
// it on miss, so multi-device push completion rechecks share one recent scan.
func CachedAgents() []Pane { return sharedSnapshot.readPanes() }

// RefreshAgentsSnapshot keeps the SSE producer fresh on every existing tick and
// publishes that same observation for REST readers. It never reuses the REST TTL
// for notifications, nor lets an older concurrent REST load overwrite this scan.
func RefreshAgentsSnapshot() []Pane {
	var panes []Pane
	_, _ = sharedSnapshot.refreshPanes(func() ([]Pane, []byte, error) {
		panes = GatherAgents()
		body, err := agentsJSONBytes(panes)
		return panes, body, err
	})
	return panes
}

func (c *snapshotCache) resetScope() {
	if c.scope != nil {
		key := c.scope()
		if key != c.scopeKey {
			c.body = nil
			c.panes = nil
			c.scopeKey = key
		}
	}
}

func (c *snapshotCache) read() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetScope()
	if c.body != nil && c.now().Sub(c.at) < snapshotTTL {
		return bytes.Clone(c.body), nil
	}
	return c.loadSnapshot(c.load)
}

func (c *snapshotCache) readPanes() []Pane {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetScope()
	if c.panes != nil && c.now().Sub(c.at) < snapshotTTL {
		return slices.Clone(c.panes)
	}
	panes := GatherAgents()
	if body, err := agentsJSONBytes(panes); err == nil && len(body) <= snapshotByteCap {
		c.body = bytes.Clone(body)
	} else {
		c.body = nil
	}
	c.panes = slices.Clone(panes)
	c.at = c.now()
	return panes
}

func (c *snapshotCache) refresh(load func() ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetScope()
	c.panes = nil
	return c.loadSnapshot(load)
}

func (c *snapshotCache) refreshPanes(load func() ([]Pane, []byte, error)) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetScope()
	panes, body, err := load()
	if err != nil {
		c.body = nil
		c.panes = slices.Clone(panes)
		c.at = c.now()
		return nil, err
	}
	if len(body) <= snapshotByteCap {
		c.body = bytes.Clone(body)
	} else {
		c.body = nil
	}
	c.panes = slices.Clone(panes)
	c.at = c.now()
	return body, nil
}

func (c *snapshotCache) loadSnapshot(load func() ([]byte, error)) ([]byte, error) {
	body, err := load()
	if err != nil {
		c.body = nil
		return nil, err
	}
	if len(body) <= snapshotByteCap {
		c.body = bytes.Clone(body)
		c.at = c.now()
	} else {
		c.body = nil
	}
	return body, nil
}
