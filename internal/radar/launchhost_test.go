package radar

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
)

func TestNativeHostProjectionDoesNotExposePrivateIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	e := launchhost.Evidence{Name: "sample-client", Kind: "process", ObservedAt: now - 30, AgentPID: 123, AgentStart: "private-agent-birth", HostPID: 124, HostStart: "private-host-birth"}
	if err := native.Save(native.Record{Agent: "claude", SessionID: "sample", State: "working", UpdatedAt: now, PID: os.Getpid(), LaunchHost: e}); err != nil {
		t.Fatal(err)
	}
	rows := nativePanes(nil, nil, now)
	if len(rows) != 1 || rows[0].LaunchHostDisplay() == nil {
		t.Fatal(rows)
	}
	rows[0].currentClient = "unconfirmed"
	b, err := agentsJSONBytes(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"agent_pid", "host_pid", "private-agent-birth", "private-host-birth"} {
		if strings.Contains(string(b), private) {
			t.Fatal("private process evidence leaked", private)
		}
	}
	var values []map[string]json.RawMessage
	if err := json.Unmarshal(b, &values); err != nil {
		t.Fatal(err)
	}
	var h map[string]any
	if err := json.Unmarshal(values[0]["launch_host"], &h); err != nil {
		t.Fatal(err)
	}
	if len(h) != 3 || h["name"] != "sample-client" || h["kind"] != "process" || h["observed_at"] != float64(now-30) {
		t.Fatal(h)
	}
	if string(values[0]["source"]) != `"native"` || string(values[0]["current_client"]) != `"unconfirmed"` || string(values[0]["adoptable"]) == "true" {
		t.Fatal(string(b))
	}
}
