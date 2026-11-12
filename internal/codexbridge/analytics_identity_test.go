package codexbridge

import (
	"fmt"
	"strings"
	"testing"
)

func TestSID002AnalyticsLabelDoesNotEraseRootIdentity(t *testing.T) {
	for _, label := range []string{"cli", "tui", "user", "command_center", "future-client-label"} {
		t.Run(label, func(t *testing.T) {
			o, bound := readyObserver(t)
			o.request([]byte(fmt.Sprintf(`{"id":1,"method":"thread/resume","params":{"threadSource":%q}}`, label)))
			o.response(rootResponse(1, "visible-root"))
			if strings.Join(*bound, ",") != "visible-root" {
				t.Fatal("analytics label erased own root identity", *bound)
			}
		})
	}
}

func TestSID002AnalyticsCannotPromoteChildOrGlobalTraffic(t *testing.T) {
	for _, response := range []string{
		`{"id":1,"result":{"thread":{"id":"child","source":"cli","parentThreadId":"parent"}}}`,
		`{"id":1,"result":{"thread":{"id":"child","source":{"subAgent":{}}}}}`,
		`{"id":1,"result":{"thread":{"id":"helper","source":"vscode","ephemeral":true}}}`,
		`{"id":1,"error":{"code":-32603}}`,
	} {
		o, bound := readyObserver(t)
		o.request([]byte(`{"id":1,"method":"thread/start","params":{"threadSource":"tui"}}`))
		o.response([]byte(response))
		if len(*bound) != 0 {
			t.Fatal("non-root response acquired identity", *bound)
		}
	}
	o, bound := readyObserver(t)
	o.request([]byte(`{"id":1,"method":"thread/read","params":{"threadId":"other","threadSource":"tui"}}`))
	o.response(rootResponse(1, "other"))
	o.response([]byte(`{"method":"thread/started","params":{"thread":{"id":"other","source":"cli"}}}`))
	if len(*bound) != 0 {
		t.Fatal("read/global traffic acquired identity", *bound)
	}
}
