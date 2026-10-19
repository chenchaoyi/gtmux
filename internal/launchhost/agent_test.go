package launchhost

import (
	"strings"
	"testing"
)

func TestAgentSnapshotDoesNotRequireHostAndRevalidatesChain(t *testing.T) {
	for _, kind := range []string{"valid", "no host", "birth", "parent", "command", "hook changed", "wrong agent", "cycle", "depth"} {
		t.Run(kind, func(t *testing.T) {
			m := fixture()
			reads := map[int]int{}
			if kind == "no host" {
				delete(m, 12)
			}
			if kind == "wrong agent" {
				p := m[11]
				p.Command = "/bin/unrelated"
				m[11] = p
			}
			if kind == "cycle" {
				p := m[10]
				p.Parent = 10
				m[10] = p
			}
			if kind == "depth" {
				for n := 10; n < 20; n++ {
					m[n] = process{n, n + 1, "born", "sh"}
				}
			}
			i := inspector{read: func(pid int) (process, bool) {
				p, ok := m[pid]
				reads[pid]++
				if pid == 11 && reads[pid] > 1 {
					switch kind {
					case "birth":
						p.Start = "new"
					case "parent":
						p.Parent = 99
					case "command":
						p.Command = "other"
					}
				}
				if kind == "hook changed" && pid == 10 && reads[pid] > 1 {
					p.Parent = 99
				}
				return p, ok
			}}
			got := i.agent(10, []string{"codex"}, 100)
			if kind == "valid" || kind == "no host" {
				if got != (AgentProcess{11, "codex", "born", 100}) {
					t.Fatal(got)
				}
			} else if got != (AgentProcess{}) {
				t.Fatal("claimed changed ancestry", got)
			}
			if len(reads) > maxDepth {
				t.Fatal("walk exceeded depth", reads)
			}
		})
	}
}
func TestAgentHandoffRejectsInvalidOrStaleEvidence(t *testing.T) {
	p := AgentProcess{11, "codex", "born", 100}
	raw := EncodeAgent(p)
	if DecodeAgent(raw, 100) != p {
		t.Fatal("handoff lost process")
	}
	for _, bad := range []string{"", strings.Repeat("x", 513), strings.Replace(raw, "codex", "../codex", 1), strings.Replace(raw, `"pid":11`, `"pid":1`, 1)} {
		if DecodeAgent(bad, 100) != (AgentProcess{}) {
			t.Fatal("accepted invalid evidence")
		}
	}
	for _, now := range []int64{161, 97} {
		if DecodeAgent(raw, now) != (AgentProcess{}) {
			t.Fatal("accepted stale/future evidence")
		}
	}
}
