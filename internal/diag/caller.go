package diag

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Who started a command. A trail that says only "gtmux sent text to %7" cannot answer the
// question people bring to it, which is whether they did that, HQ did, or an agent in
// another pane did. The answer comes from what the process can see, never from a guess:
//
//   - GTMUX_ACTOR, set by a gtmux component that runs the CLI on someone's behalf (the menu
//     bar sets "menubar"), and only from a closed set, so a stray value cannot invent an
//     actor;
//   - "hq" when the command runs at or inside HQ's home, the same rule the journal uses to
//     decide that a read is HQ's;
//   - "agent:%N" when it runs from a tmux pane whose agent is mid-turn, which the hook
//     records as the pane's turn marker. A person cannot type a shell command into a pane
//     while its agent holds the turn, so the command came from the agent;
//   - "user" otherwise: someone typed it.
//
// A long-running component is not a command: serve, the hook and the tunnel client call
// SetProcess with their component and "system" when they start, so what they do on their own is not credited to
// whoever happens to share the process's pane or directory. serve names the device behind
// a request with As.

// ActorEnv is the variable a gtmux component sets on a CLI it runs for someone else.
const ActorEnv = "GTMUX_ACTOR"

// delegatedActors are the values ActorEnv may carry.
var delegatedActors = map[string]bool{"menubar": true, "update": true, "system": true}

var (
	actorMu      sync.RWMutex
	processComp  string // SetProcess
	processActor string // SetProcess
	scopedActor  string // As, while it runs
	scopedG      uint64 // the goroutine As runs fn on
	asMu         sync.Mutex
)

// SetProcess names the component this process is and the actor for what it does on its
// own: Did writes under that component, and Caller returns that actor.
func SetProcess(component, actor string) {
	actorMu.Lock()
	processComp, processActor = component, actor
	actorMu.Unlock()
}

// As runs fn with Caller returning actor, so a verb serve performs for a device, whose
// record is written deep inside the verb, names that device. Only the goroutine running
// fn sees it: serve's ticks run beside its handlers, and one of them writing a record
// while a phone's verb runs must not be credited to the phone. Calls are serialized.
func As(actor string, fn func() error) error {
	asMu.Lock()
	defer asMu.Unlock()
	actorMu.Lock()
	scopedActor, scopedG = actor, goid()
	actorMu.Unlock()
	defer func() {
		actorMu.Lock()
		scopedActor, scopedG = "", 0
		actorMu.Unlock()
	}()
	return fn()
}

// goid is the current goroutine's id, read from its stack header ("goroutine 18 [").
func goid() uint64 {
	var buf [64]byte
	b := buf[:runtime.Stack(buf[:], false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	if i := bytes.IndexByte(b, ' '); i > 0 {
		b = b[:i]
	}
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

// Caller returns the actor for what this process is doing now.
func Caller() string {
	actorMu.RLock()
	scoped, g, proc := scopedActor, scopedG, processActor
	actorMu.RUnlock()
	if scoped != "" && g == goid() {
		return scoped
	}
	if proc != "" {
		return proc
	}
	if a := strings.TrimSpace(os.Getenv(ActorEnv)); delegatedActors[a] {
		return a
	}
	if atOrInside(cwd(), state.HQHome()) {
		return "hq"
	}
	if pane := os.Getenv("TMUX_PANE"); state.IsPaneID(pane) && state.Exists(state.ActivePath(pane)) {
		return "agent:" + pane
	}
	return "user"
}

func cwd() string {
	d, err := os.Getwd()
	if err != nil {
		return ""
	}
	return d
}

func atOrInside(dir, home string) bool {
	if dir == "" || home == "" {
		return false
	}
	dir, home = filepath.Clean(dir), filepath.Clean(home)
	return dir == home || strings.HasPrefix(dir, home+string(os.PathSeparator))
}

// Did records an action taken in this process, by the actor Caller names: under the
// component SetProcess named, or "cli" for a command. It is for code that runs in more
// than one kind of process, like the journal's audit acts; code that knows its actor and
// component calls Act directly.
func Did(event, target, outcome, msg string, kv ...any) {
	actorMu.RLock()
	comp := processComp
	actorMu.RUnlock()
	if comp == "" {
		comp = "cli"
	}
	For(comp).Act(event, Caller(), target, outcome, msg, kv...)
}

// DidRC records a command's act from its exit code and passes the code through: 0 is ok,
// 1 failed. A usage error (2) changed nothing and is not an act.
func DidRC(event, target string, rc int, msg string, kv ...any) int {
	switch rc {
	case 0:
		Did(event, target, OK, msg, kv...)
	case 2:
	default:
		Did(event, target, Failed, msg, append(kv, "exit", rc)...)
	}
	return rc
}

// Outcome maps an error to an action outcome: nil is ok, anything else failed.
func Outcome(err error) string {
	if err != nil {
		return Failed
	}
	return OK
}

// Sum is the short hash an entry carries in place of a payload: enough to match a send to
// its receipt in the journal, nothing that reads back as the text.
func Sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:4])
}
