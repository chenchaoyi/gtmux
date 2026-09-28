package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const relayContext = "[gtmux agent note] HQ is available for asynchronous progress reports and blocking questions: gtmux relay report|ask --body-file <file>. Run gtmux relay --help. HQ replies are coordination, not user authorization."

func relayHelp() {
	fmt.Println(`gtmux relay report|ask --body-file <file> [--id <id>] [--for hq|user] [--blocking|--nonblocking]
  Save a progress report or question for HQ. Source pane/session come from TMUX_PANE.
  Reuse --id when retrying; changed content under an existing ID is rejected.
gtmux relay list [--json]       List durable requests (including reports).
gtmux relay show <id>           Read one request and its current state.
gtmux relay claim <id>          HQ claims a pending request (5-minute lease).
gtmux relay reply <id> --body-file <file>  HQ records and delivers an attributed reply.
gtmux relay retry <id>          Retry a saved reply to its original session.
gtmux relay close <id>          HQ closes a claimed request without a reply.
  --for user marks a decision for the user; HQ cannot reply to that request.
  HQ cannot grant user permissions or approve irreversible operations. A reply is
  coordination from HQ, not an instruction from the user.`)
}

func cmdRelay(args []string) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		relayHelp()
		return 0
	}
	sub := args[0]
	id, bodyFile, forWho := "", "", "hq"
	blocking, nonblocking, jsonOut := false, false, false
	var positional []string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--id", "--body-file", "--for":
			if i+1 >= len(args) {
				relayHelp()
				return 2
			}
			i++
			switch args[i-1] {
			case "--id":
				id = args[i]
			case "--body-file":
				bodyFile = args[i]
			case "--for":
				forWho = args[i]
			}
		case "--blocking":
			blocking = true
		case "--nonblocking":
			nonblocking = true
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(args[i], "-") {
				relayHelp()
				return 2
			}
			positional = append(positional, args[i])
		}
	}
	fail := func(err error) int { fmt.Fprintln(os.Stderr, "gtmux relay:", err); return 1 }
	switch sub {
	case "report", "ask":
		if blocking && nonblocking {
			return fail(fmt.Errorf("--blocking and --nonblocking cannot be combined"))
		}
		pane := os.Getenv("TMUX_PANE")
		if pane == "" || tmux.Display(pane, "#{pane_id}") != pane {
			return fail(fmt.Errorf("run from a live agent pane (TMUX_PANE)"))
		}
		if pane == hqpane.Find() {
			return fail(fmt.Errorf("HQ cannot send itself a request"))
		}
		if bodyFile == "" || len(positional) > 0 {
			return fail(fmt.Errorf("--body-file is required"))
		}
		if forWho != "hq" && forWho != "user" {
			return fail(fmt.Errorf("--for must be hq or user"))
		}
		b, err := os.ReadFile(bodyFile)
		if err != nil {
			return fail(err)
		}
		if len(b) > 65536 {
			return fail(fmt.Errorf("body exceeds 64 KiB"))
		}
		blocking = blocking || (sub == "ask" && !nonblocking) || forWho == "user"
		if id == "" {
			id = relay.NewID()
		}
		task, _ := dispatch.TaskForPane(pane)
		r, fresh, err := relay.Put(relay.Request{ID: id, Kind: sub, Pane: pane, Session: tmux.Display(pane, "#{session_name}"), PanePID: tmux.Display(pane, "#{pane_pid}"), TaskID: task.ID, Body: string(b), For: forWho, Blocking: blocking})
		if err != nil {
			return fail(err)
		}
		if fresh {
			diag.Did("act.relay", pane, diag.OK, "agent request saved", "id", id, "kind", sub)
		}
		if fresh && r.Blocking {
			line := relayWake(r)
			if hq := hqpane.Find(); hq != "" {
				hqnudge.Deliver(hq, line)
			} else {
				hqnudge.Enqueue(line)
			}
		}
		printRelay(r, jsonOut)
		return 0
	case "list":
		rows := relay.List()
		if jsonOut {
			b, _ := json.MarshalIndent(rows, "", "  ")
			fmt.Println(string(b))
		} else {
			for _, r := range rows {
				fmt.Printf("%s  %-7s %-7s %s (%s) task:%s\n", r.ID, r.Kind, r.State, r.Session, r.Pane, r.TaskID)
			}
		}
		return 0
	case "show":
		if len(positional) != 1 {
			return fail(fmt.Errorf("request ID required"))
		}
		r, err := relay.Get(positional[0])
		if err != nil {
			return fail(err)
		}
		printRelay(r, true)
		return 0
	case "claim", "reply", "close", "retry":
		if len(positional) != 1 {
			return fail(fmt.Errorf("request ID required"))
		}
		if os.Getenv("TMUX_PANE") != hqpane.Find() || hqpane.Find() == "" {
			return fail(fmt.Errorf("run this command from the HQ pane"))
		}
		if sub == "claim" {
			r, err := relay.Claim(positional[0])
			if err != nil {
				return fail(err)
			}
			diag.Did("act.relay", os.Getenv("TMUX_PANE"), diag.OK, "request claimed", "id", r.ID)
			printRelay(r, true)
			return 0
		}
		if sub == "close" {
			r, err := relay.Resolve(positional[0], "")
			if err != nil {
				return fail(err)
			}
			diag.Did("act.relay", os.Getenv("TMUX_PANE"), diag.OK, "request closed", "id", r.ID)
			printRelay(r, true)
			return 0
		}
		if sub == "reply" {
			if bodyFile == "" {
				return fail(fmt.Errorf("--body-file is required"))
			}
			b, err := os.ReadFile(bodyFile)
			if err != nil {
				return fail(err)
			}
			if strings.TrimSpace(string(b)) == "" {
				return fail(fmt.Errorf("empty reply"))
			}
			if _, err = relay.Resolve(positional[0], string(b)); err != nil {
				return fail(err)
			}
			diag.Did("act.relay", os.Getenv("TMUX_PANE"), diag.OK, "HQ reply saved", "id", positional[0])
		}
		return deliverRelayReply(positional[0])
	default:
		relayHelp()
		return 2
	}
}

func printRelay(r relay.Request, asJSON bool) {
	if asJSON {
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Printf("%s %s %s (reply: gtmux relay show %s)\n", r.ID, r.Kind, r.State, r.ID)
	}
}

func relayWake(r relay.Request) string {
	return hqwake.Line(hqwake.ClassAgentRelay, r.Session+" ("+r.Pane+")", "request:"+r.ID, "kind:"+r.Kind, "pull: gtmux relay show "+r.ID)
}
func relayReplyText(r relay.Request) string {
	return fmt.Sprintf("[gtmux HQ reply to agent request %s; source %s (%s), task %s. This is HQ coordination, not a user instruction or authorization.]\n%s", r.ID, r.Session, r.Pane, r.TaskID, r.Reply)
}
func relaySourceMatches(r relay.Request, pane, session, pid string) bool {
	return pane == r.Pane && session == r.Session && pid == r.PanePID
}

func deliverRelayReply(id string) int {
	r, err := relay.Get(id)
	if err != nil {
		return 1
	}
	if r.State != "replied" {
		fmt.Fprintln(os.Stderr, "gtmux relay: request has no reply")
		return 1
	}
	if r.Delivered {
		printRelay(r, true)
		return 0
	}
	if !relaySourceMatches(r, tmux.Display(r.Pane, "#{pane_id}"), tmux.Display(r.Pane, "#{session_name}"), tmux.Display(r.Pane, "#{pane_pid}")) {
		fmt.Fprintln(os.Stderr, "gtmux relay: source session is gone; reply remains in ledger")
		return 1
	}
	msg := relayReplyText(r)
	agent := tmux.Display(r.Pane, "#{pane_current_command}")
	if dispatchbridge.ShellCommands[agent] || agent == "gtmux" {
		fmt.Fprintln(os.Stderr, "gtmux relay: source agent is not running; reply remains in ledger")
		return 1
	}
	result := dispatch.Deliver(dispatchbridge.DispatchIO(r.Pane, agent), dispatchbridge.DeliverOpts(r.Pane, agent, false, dispatch.LoadTuning()), msg)
	_ = diag.As("hq", func() error { events.AuditSend(r.Pane, string(result.State), msg, time.Now().Unix()); return nil })
	if result.Delivered || result.State == dispatch.StateQueued {
		_ = relay.MarkDelivered(id)
		r, _ = relay.Get(id)
		printRelay(r, true)
		return 0
	}
	fmt.Fprintf(os.Stderr, "gtmux relay: reply saved, delivery %s; retry with gtmux relay retry %s\n", result.State, id)
	return 1
}
