package agentclient

import "testing"

func TestLifecycleCapabilityComesFromAdapter(t *testing.T) {
	if !ProvidesLifecycle("dst") || ProvidesLifecycle("unknown") {
		t.Fatal("declared capability lost")
	}
	old := adapters
	defer func() { adapters = old }()
	adapters = map[string]adapter{"synthetic": {lifecycle: true}, "dst": {}}
	if !ProvidesLifecycle("synthetic") || ProvidesLifecycle("dst") {
		t.Fatal("capability repeated a client-name list")
	}
}
