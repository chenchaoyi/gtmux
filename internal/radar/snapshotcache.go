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
func (c *snapshotCache) read() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.scope != nil {
		key := c.scope()
		if key != c.scopeKey {
			c.body = nil
			c.scopeKey = key
		}
	}
	if c.body != nil && c.now().Sub(c.at) < snapshotTTL {
		return bytes.Clone(c.body), nil
	}
	body, err := c.load()
	if err != nil {
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
