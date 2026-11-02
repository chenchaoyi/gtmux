package app

import (
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotSettingsPreserveOtherConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p := configPath()
	_ = os.MkdirAll(filepath.Dir(p), 0700)
	_ = os.WriteFile(p, []byte(`{"other":"synthetic-private-field"}`), 0600)
	if n := cmdConfig([]string{"snapshots", "off", "--keep", "5", "--max-mb", "32"}); n != 0 {
		t.Fatalf("config returned %d", n)
	}
	c, e := hq.SnapshotConfiguration()
	if e != nil || c.Enabled || c.Keep != 5 || c.MaxMB != 32 {
		t.Fatalf("policy %+v %v", c, e)
	}
	b, _ := os.ReadFile(p)
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	if v["other"] != "synthetic-private-field" {
		t.Fatal("other config lost")
	}
	before := string(b)
	if cmdConfig([]string{"snapshots", "on", "--keep", "0"}) == 0 {
		t.Fatal("invalid policy accepted")
	}
	b, _ = os.ReadFile(p)
	if string(b) != before {
		t.Fatal("invalid update changed config")
	}
	_ = os.WriteFile(p, []byte(`{broken`), 0600)
	if cmdConfig([]string{"snapshots", "off"}) == 0 {
		t.Fatal("malformed config overwritten")
	}
	b, _ = os.ReadFile(p)
	if string(b) != `{broken` {
		t.Fatal("malformed config lost")
	}
}
