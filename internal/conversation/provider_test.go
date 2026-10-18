package conversation

import (
	"github.com/chenchaoyi/gtmux/internal/agents"
	"testing"
)

func TestManifestNativeReaderHasAdapter(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range agents.All() {
		if m.NativeReader == "" {
			continue
		}
		if _, ok := providers[m.NativeReader]; !ok {
			t.Errorf("%s promises unwired reader %s", m.Key, m.NativeReader)
		}
		if seen[m.NativeReader] {
			t.Errorf("duplicate reader %s", m.NativeReader)
		}
		seen[m.NativeReader] = true
	}
	for key := range providers {
		if !seen[key] {
			t.Errorf("unregistered reader %s", key)
		}
	}
}
