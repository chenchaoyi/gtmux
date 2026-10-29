package codexbridge

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func readyObserver(t *testing.T) (*observer, *[]string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	var bound []string
	o := &observer{home: transcript.CodexHomeID(), bind: func(id, cwd string) { bound = append(bound, id) }}
	o.request([]byte(`{"id":0,"method":"initialize"}`))
	h, _ := json.Marshal(home)
	o.response([]byte(`{"id":0,"result":{"codexHome":` + string(h) + `}}`))
	return o, &bound
}

func rootResponse(id int, thread string) []byte {
	return []byte(fmt.Sprintf(`{"id":%d,"result":{"thread":{"id":%q,"ephemeral":false,"source":"vscode"}}}`, id, thread))
}

func TestObserverBindsOnlyItsOwnSuccessfulRootOperations(t *testing.T) {
	o, bound := readyObserver(t)
	for i, method := range []string{"thread/start", "thread/resume", "thread/fork"} {
		o.request([]byte(fmt.Sprintf(`{"id":%d,"method":%q,"params":{}}`, i+1, method)))
		o.response(rootResponse(i+1, fmt.Sprintf("root-%d", i+1)))
	}
	if strings.Join(*bound, ",") != "root-1,root-2,root-3" {
		t.Fatalf("root bindings = %v", *bound)
	}
	// A response/notification seen on this socket is not evidence unless this
	// socket issued the corresponding root operation.
	o.response(rootResponse(44, "foreign"))
	o.response([]byte(`{"method":"thread/started","params":{"thread":{"id":"foreign"}}}`))
	if len(*bound) != 3 {
		t.Fatal("global/foreign traffic claimed a pane", *bound)
	}
}

func TestObserverExcludesHelpersSubagentsAndFailures(t *testing.T) {
	for name, exchange := range map[string][2]string{
		"title helper":       {`{"id":1,"method":"thread/start","params":{"ephemeral":true,"threadSource":"thread_title"}}`, string(rootResponse(1, "helper"))},
		"ephemeral response": {`{"id":1,"method":"thread/start"}`, `{"id":1,"result":{"thread":{"id":"helper","ephemeral":true,"source":"vscode"}}}`},
		"subagent":           {`{"id":1,"method":"thread/resume"}`, `{"id":1,"result":{"thread":{"id":"child","source":{"subAgent":{"other":"x"}}}}}`},
		"exec":               {`{"id":1,"method":"thread/resume"}`, `{"id":1,"result":{"thread":{"id":"exec","source":"exec"}}}`},
		"error":              {`{"id":1,"method":"thread/start"}`, `{"id":1,"error":{"code":-32603}}`},
		"missing provenance": {`{"id":1,"method":"thread/start"}`, `{"id":1,"result":{"thread":{"id":"unknown"}}}`},
	} {
		t.Run(name, func(t *testing.T) {
			o, bound := readyObserver(t)
			o.request([]byte(exchange[0]))
			o.response([]byte(exchange[1]))
			if len(*bound) > 0 {
				t.Fatalf("unowned thread bound: %v", *bound)
			}
		})
	}
}

func TestObserverIgnoresLateRootAfterNewerSelection(t *testing.T) {
	o, bound := readyObserver(t)
	o.request([]byte(`{"id":1,"method":"thread/start"}`))
	o.request([]byte(`{"id":2,"method":"thread/start"}`))
	o.response(rootResponse(2, "new"))
	o.response(rootResponse(1, "old"))
	if strings.Join(*bound, ",") != "new" {
		t.Fatal("late operation stole foreground identity", *bound)
	}
}

func TestObserverCatalogTrafficCannotConsumeIdentityCapacity(t *testing.T) {
	o, bound := readyObserver(t)
	for i := 1; i < 1000; i++ {
		o.request([]byte(fmt.Sprintf(`{"id":%d,"method":"plugin/list"}`, i)))
	}
	if len(o.pending) != 0 {
		t.Fatal("catalog requests retained", len(o.pending))
	}
	o.response([]byte(`{"id":1,"result":{"largeCatalog":"` + strings.Repeat("x", observationLimit) + `"}}`))
	o.request([]byte(`{"id":1001,"method":"thread/start"}`))
	o.response(rootResponse(1001, "real"))
	if strings.Join(*bound, ",") != "real" {
		t.Fatal("catalog prevented a later binding", *bound)
	}
}

func TestObserverRequiresMatchingServerHome(t *testing.T) {
	o, bound := readyObserver(t)
	o.request([]byte(`{"id":5,"method":"initialize"}`))
	o.response([]byte(`{"id":5,"result":{"codexHome":"/synthetic/other-home"}}`))
	o.request([]byte(`{"id":6,"method":"thread/start"}`))
	o.response(rootResponse(6, "foreign"))
	if len(*bound) != 0 {
		t.Fatal("different server home claimed a pane", *bound)
	}
}

func TestObserverResumesLargeHistoryWithoutRetainingCatalogs(t *testing.T) {
	o, bound := readyObserver(t)
	o.request([]byte(`{"id":3,"method":"thread/resume"}`))
	response := `{"result":{"thread":{"id":"long-session","ephemeral":false,"source":"cli","turns":[{"text":"` + strings.Repeat("x", 2<<20) + `"}]}},"id":3}`
	o.response([]byte(response))
	if strings.Join(*bound, ",") != "long-session" || len(o.pending) != 0 {
		t.Fatal("long history prevented exact resume binding", *bound, len(o.pending))
	}
}

func TestObserverKeepsSuccessfulRootWorkspace(t *testing.T) {
	o, _ := readyObserver(t)
	var cwd string
	o.bind = func(_ string, dir string) { cwd = dir }
	o.request([]byte(`{"id":2,"method":"thread/resume"}`))
	o.response([]byte(`{"id":2,"result":{"thread":{"id":"saved-root","cwd":"/saved/project","source":"cli"}}}`))
	if cwd != "/saved/project" {
		t.Fatal("root workspace lost", cwd)
	}
}
