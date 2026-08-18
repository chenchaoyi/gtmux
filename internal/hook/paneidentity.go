package hook

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// paneFromAncestry answers "which tmux pane does this hook belong to?" when
// $TMUX_PANE is absent.
//
// $TMUX_PANE used to be enough, because the agent ran in the pane. It no longer
// is: Claude Code hands a session to a BACKGROUND HOST — `claude daemon run` →
// `--bg-pty-host` → the conversation process — and that process inherits neither
// a tty nor the tmux env. Measured on 2026-08-18, pane %13:
//
//	78152  the session process      (no TMUX_PANE, no tty)
//	77301    --bg-pty-host
//	77257      claude daemon run --origin transient --spawned-by {pid:34175}
//	34175        claude            tty ttys014   ← the pane's interactive client
//	19982          bash            ← tmux reports this as %13's pane_pid
//
// The env is gone but the ANCESTRY still leads home, so we ask the process tree
// instead of the environment. The pane went blind for 5h15m before this existed:
// every event was filed as a native (non-tmux) session, so the pane's state, its
// resume binding and its event stream all froze while looking perfectly calm.
//
// Two signals, strongest first:
//
//   - an ancestor pid IS a pane's pane_pid — exact, no interpretation;
//   - an ancestor's tty IS a pane's pane_tty — tmux allocates one pty per pane, so
//     a match is unique; a terminal running a NON-tmux agent has the terminal's own
//     tty, which is never a pane_tty.
//
// It fails open by construction: it runs only when $TMUX_PANE is absent, walks a
// bounded number of ancestors, treats every error as "not identified", and
// refuses to guess when more than one pane matches. Falling through means the
// existing native-session path, which stays correct for agents that really are
// outside tmux.
func paneFromAncestry() string {
	if tmux.Bin == "" {
		return ""
	}
	chain := ancestry(os.Getpid(), maxAncestorWalk)
	if len(chain) == 0 {
		return ""
	}
	return resolvePane(chain, panesForIdentity())
}

// resolvePane is the decision itself, kept free of ps/tmux so the rules are
// testable as rules. Two signals, strongest first:
//
//   - an ancestor pid IS a pane's pane_pid — exact, no interpretation;
//   - an ancestor's tty IS a pane's pane_tty — tmux allocates one pty per pane, so
//     a match is unique; a terminal running a NON-tmux agent has the terminal's own
//     tty, which is never a pane_tty.
//
// Ambiguity is never resolved by picking one: two panes matching the same signal
// returns "", which sends the caller down the native path it would have taken
// anyway. Guessing a pane would write one agent's events onto another's row.
func resolvePane(chain []ancestor, panes []identityPane) string {
	if len(chain) == 0 || len(panes) == 0 {
		return ""
	}
	for _, a := range chain {
		var hit string
		for _, p := range panes {
			if p.pid == 0 || p.pid != a.pid {
				continue
			}
			if hit != "" && hit != p.id {
				return "" // two panes claim the same pid — refuse to guess
			}
			hit = p.id
		}
		if hit != "" {
			return hit
		}
	}
	for _, a := range chain {
		if normalizeTTY(a.tty) == "" {
			continue
		}
		var hit string
		for _, p := range panes {
			if !sameTTY(p.tty, a.tty) {
				continue
			}
			if hit != "" && hit != p.id {
				return ""
			}
			hit = p.id
		}
		if hit != "" {
			return hit
		}
	}
	return ""
}

// maxAncestorWalk bounds the climb. The measured chain is 4 deep; 8 leaves room
// for another layer of hosting without turning a hook into a process crawler.
const maxAncestorWalk = 8

type ancestor struct {
	pid int
	tty string
}

// ancestry returns the process and its ancestors, closest first, INCLUDING the
// starting pid (a hook is a child of the agent, so the agent is already one hop
// up, but the caller should not have to know that).
func ancestry(pid, max int) []ancestor {
	var out []ancestor
	for i := 0; i < max && pid > 1; i++ {
		ppid, tty := psPidTTY(pid)
		out = append(out, ancestor{pid: pid, tty: tty})
		if ppid <= 1 {
			break
		}
		pid = ppid
	}
	return out
}

// psPidTTY reads one pid's parent and controlling tty. A process with no tty
// (every background host in this story) reports "??" or "-", which we normalize
// to "" so it can never match a pane.
func psPidTTY(pid int) (ppid int, tty string) {
	out, err := exec.Command("ps", "-o", "ppid=,tty=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, ""
	}
	f := strings.Fields(strings.TrimSpace(string(out)))
	if len(f) == 0 {
		return 0, ""
	}
	ppid, _ = strconv.Atoi(f[0])
	if len(f) > 1 {
		tty = normalizeTTY(f[1])
	}
	return ppid, tty
}

type identityPane struct {
	id  string
	pid int
	tty string
}

// panesForIdentity lists every pane with the two fields we can match on. One
// tmux call, not one per pane.
func panesForIdentity() []identityPane {
	out, err := tmux.Run("list-panes", "-a", "-F", "#{pane_id} #{pane_pid} #{pane_tty}")
	if err != nil || out == "" {
		return nil
	}
	var panes []identityPane
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		p := identityPane{id: f[0]}
		p.pid, _ = strconv.Atoi(f[1])
		if len(f) > 2 {
			p.tty = normalizeTTY(f[2])
		}
		panes = append(panes, p)
	}
	return panes
}

// normalizeTTY reduces the several spellings of the same terminal to one:
// `ps` prints "ttys014" (or "??"/"-" for none) where tmux prints "/dev/ttys014".
func normalizeTTY(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "??" || s == "-" || s == "?" {
		return ""
	}
	return strings.TrimPrefix(s, "/dev/")
}

func sameTTY(a, b string) bool {
	a, b = normalizeTTY(a), normalizeTTY(b)
	return a != "" && a == b
}
