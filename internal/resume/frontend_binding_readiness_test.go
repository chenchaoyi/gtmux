package resume

import (
	"errors"
	"os"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

func TestCBA001UnavailableProofIsRetryableButNeverPublishes(t *testing.T) {
	for _, missing := range []string{"bridge", "backend"} {
		t.Run(missing, func(t *testing.T) {
			f, _ := frontendFixture(t)
			original := frontendProcess
			frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
				if missing == "bridge" && pid == os.Getpid() || missing == "backend" && pid == f.BackendPID {
					return launchhost.AgentProcess{}, false
				}
				return original(pid)
			}
			if _, e := BindCodexFrontend(f, "root"); !bindingRetryable(e) {
				t.Fatal(e)
			}
			if _, ok := LoadLive(f.Loc); ok {
				t.Fatal("unknown process granted authority")
			}
			frontendProcess = original
			if _, e := BindCodexFrontend(f, "root"); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func bindingRetryable(err error) bool {
	var e *FrontendBindingError
	return errors.As(err, &e) && e.Retryable
}
func TestCBA001ObservedBirthHomeAndTargetChangesStayHard(t *testing.T) {
	for _, mode := range []string{"bridge", "backend", "home", "target"} {
		t.Run(mode, func(t *testing.T) {
			f, c := frontendFixture(t)
			original := frontendProcess
			switch mode {
			case "bridge", "backend":
				frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
					p, ok := original(pid)
					if mode == "bridge" && pid == f.BridgePID || mode == "backend" && pid == f.BackendPID {
						p.Start = "new-birth"
					}
					return p, ok
				}
			case "home":
				f.HomeID = "different"
			case "target":
				c.Start = "reused-client"
			}
			if _, e := BindCodexFrontend(f, "root"); e == nil || bindingRetryable(e) {
				t.Fatal("observed conflict became retryable", mode, e)
			}
			if _, ok := LoadLive(f.Loc); ok {
				t.Fatal("conflict published")
			}
		})
	}
}
func TestCBA001MissingTerminalProofNeverBecomesInputAuthority(t *testing.T) {
	f, _ := frontendFixture(t)
	original := frontendClients
	frontendClients = func([]codexterminal.Pane) map[string]codexterminal.Client { return nil }
	if _, e := BindCodexFrontend(f, "root"); !bindingRetryable(e) {
		t.Fatal(e)
	}
	if _, ok := LoadLive(f.Loc); ok {
		t.Fatal("unknown terminal published")
	}
	frontendClients = original
	if _, e := BindCodexFrontend(f, "root"); e != nil {
		t.Fatal(e)
	}
}
