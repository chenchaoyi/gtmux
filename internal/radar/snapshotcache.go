package radar

import (
	"bytes"
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
	at       time.Time
	load     func() ([]byte, error)
	now      func() time.Time
}

var sharedSnapshot = &snapshotCache{load: AgentsJSONBytes, now: time.Now, scope: state.Home}

// CachedAgentsJSONBytes is the serve's one producer snapshot, shared by SSE and
// authenticated HTTP readers. Standalone CLI reads keep their fresh path.
func CachedAgentsJSONBytes() ([]byte, error) { return sharedSnapshot.read() }

// RefreshAgentsSnapshot keeps the SSE producer fresh on every existing tick and
// publishes that same observation for REST readers. It never reuses the REST TTL
// for notifications, nor lets an older concurrent REST load overwrite this scan.
func RefreshAgentsSnapshot() []Pane {
	var panes []Pane
	_, _ = sharedSnapshot.refresh(func() ([]byte, error) {
		panes = GatherAgents()
		return agentsJSONBytes(panes)
	})
	return panes
}

func (c *snapshotCache) resetScope() {
	if c.scope != nil {
		key := c.scope()
		if key != c.scopeKey {
			c.body = nil
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

func (c *snapshotCache) refresh(load func() ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetScope()
	return c.loadSnapshot(load)
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
