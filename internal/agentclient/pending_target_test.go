package agentclient

import (
	"testing"
	"time"
)

func TestMSD006PendingTargetIdentity(t *testing.T) {
	for _, age := range []time.Duration{time.Second, 16 * time.Second} {
		for _, field := range []string{"session", "instance", "home", "binding", "pid", "server", "pane"} {
			t.Run(age.String()+"/"+field, func(t *testing.T) {
				_, a, inspect := attachmentFixture(t)
				r, err := Prepare(a, "synthetic", "fixed-request", false, inspect.Now)
				if err != nil {
					t.Fatal(err)
				}
				same, err := Prepare(a, "synthetic", "fixed-request", false, inspect.Now.Add(time.Millisecond))
				if err != nil || same.ID != r.ID {
					t.Fatal("same target pending lost identity", err)
				}
				switch field {
				case "session":
					a.SessionID += "-changed"
				case "instance":
					a.Instance = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
				case "home":
					a.HomeID += "-changed"
				case "binding":
					a.BindingEpoch++
				case "pid":
					a.PID++
				case "server":
					a.ServerEpoch += "-changed"
				case "pane":
					a.Pane += "changed"
				}
				// pane participates in the index key: a different pane has an independent ID.
				got, err := Prepare(a, "synthetic", "fixed-request", false, inspect.Now.Add(age))
				if field == "pane" {
					if err != nil || got.ID == r.ID {
						t.Fatal("different pane unexpectedly reused index", err)
					}
					return
				}
				if err == nil {
					t.Fatalf("pending %s crossed original identity at %s: %+v", field, age, got)
				}
			})
		}
	}
}
