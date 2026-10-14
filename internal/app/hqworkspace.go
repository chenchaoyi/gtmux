package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// workspaceAction is shared by the local CLI and its native consumer. Message
// bodies travel over stdin, never shell interpolation or process arguments.
type workspaceAction struct {
	Op                   string                  `json:"op"`
	ID                   string                  `json:"id"`
	Matter               string                  `json:"matter,omitempty"`
	Into                 string                  `json:"into,omitempty"`
	Title                string                  `json:"title,omitempty"`
	Body                 string                  `json:"body,omitempty"`
	State                string                  `json:"state,omitempty"`
	Kind                 string                  `json:"kind,omitempty"`
	ReplyTo              string                  `json:"reply_to,omitempty"`
	Revision             uint64                  `json:"revision,omitempty"`
	Association          hqworkspace.Association `json:"association,omitempty"`
	AssociationID        string                  `json:"association_id,omitempty"`
	Associations         []string                `json:"associations,omitempty"`
	From                 int64                   `json:"from,omitempty"`
	Until                int64                   `json:"until,omitempty"`
	AcknowledgeUncertain bool                    `json:"acknowledge_uncertain,omitempty"`
}

func workspaceStore() *hqworkspace.Store { return hqworkspace.New(state.HQHome()) }

func workspaceHelp() int {
	fmt.Println(`gtmux hq workspace list --json
gtmux hq workspace show <matter-id|global|activity> [--before SEQ] [--earlier N] --json
gtmux hq workspace participant <matter-id> <association-id> --json
gtmux hq workspace act --stdin --json
gtmux hq workspace drain --json

The workspace keeps explicit goals, conversation membership and instruction receipts.
act reads ONE JSON object from stdin. Operations: create, send, report, update,
associate, unlink, move, interval, merge, split, cancel, retry, focus.
create needs id/title/body (one new goal and first instruction); send needs id/body
and optional matter; report needs id/kind/body and optional matter/reply_to/state.
report is accepted only from the current HQ pane. Association uses an exact agent,
home/session identity and from/until timestamps, never a title or recycled pane.
drain attempts only previously saved pending instructions; uncertain submissions
require explicit acknowledged retry. Reading never consumes HQ event watermark.`)
	return 0
}

func cmdHQWorkspace(args []string) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		return workspaceHelp()
	}
	verb := args[0]
	before := uint64(0)
	earlier := 0
	stdin := false
	rest := []string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
		case "--stdin":
			stdin = true
		case "--before", "--earlier":
			flag := args[i]
			i++
			if i >= len(args) {
				return 2
			}
			n, err := strconv.ParseUint(args[i], 10, 64)
			if err != nil {
				return 2
			}
			if flag == "--before" {
				before = n
			} else {
				if n > 20 {
					return 2
				}
				earlier = int(n)
			}
		default:
			if strings.HasPrefix(args[i], "--") {
				return 2
			}
			rest = append(rest, args[i])
		}
	}
	var result any
	var err error
	s := workspaceStore()
	switch verb {
	case "list":
		if len(rest) != 0 || stdin {
			return 2
		}
		result, err = workspaceSnapshot(s)
	case "show":
		if len(rest) != 1 || stdin {
			return 2
		}
		result, err = workspaceDetail(s, rest[0], before, earlier)
	case "participant":
		if len(rest) != 2 || stdin {
			return 2
		}
		result, err = workspaceParticipant(s, rest[0], rest[1])
	case "act":
		if !stdin || len(rest) != 0 {
			return 2
		}
		var action workspaceAction
		action, err = readWorkspaceAction(os.Stdin)
		if err == nil {
			result, err = applyWorkspaceAction(s, action, time.Now().Unix())
		}
	case "drain":
		if len(rest) != 0 || stdin {
			return 2
		}
		var attempted bool
		attempted, err = pumpWorkspace(s, time.Now().Unix())
		result = map[string]bool{"attempted": attempted}
	default:
		fmt.Fprintln(os.Stderr, "unknown HQ workspace command:", verb)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gtmux hq workspace:", err)
		return 1
	}
	b, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(string(b))
	return 0
}

func readWorkspaceAction(reader io.Reader) (workspaceAction, error) {
	var a workspaceAction
	b, err := io.ReadAll(io.LimitReader(reader, 65537))
	if err != nil {
		return a, err
	}
	if len(b) > 65536 {
		return a, errors.New("workspace request exceeds 64KiB")
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err = d.Decode(&a); err != nil {
		return a, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return a, errors.New("expected one workspace action")
	}
	return a, nil
}

func applyWorkspaceAction(s *hqworkspace.Store, a workspaceAction, now int64) (any, error) {
	var err error
	var result any = map[string]bool{"ok": true}
	switch a.Op {
	case "create":
		result, err = s.Start(a.ID, a.Title, a.ID+"-instruction", a.Body, now)
	case "send":
		result, err = s.Submit(a.ID, a.Matter, a.Body, now)
	case "report":
		if !workspaceIsHQCaller() {
			return nil, errors.New("only the verified current HQ pane may publish an HQ report")
		}
		result, err = s.Publish(a.ID, a.Matter, a.ReplyTo, a.Kind, a.Body, a.State, now)
	case "update":
		err = s.Update(a.Matter, a.Title, a.State, a.Body, a.Revision, now)
	case "associate":
		if !workspaceKnownConversation(a.Association.Conversation) {
			return nil, errors.New("conversation identity is not available on this Mac")
		}
		err = s.Associate(a.Matter, a.Association, a.Revision, now)
	case "unlink":
		err = s.Move(a.Matter, "", a.AssociationID, a.Revision, now)
	case "move":
		err = s.Move(a.Matter, a.Into, a.AssociationID, a.Revision, now)
	case "interval":
		err = s.Interval(a.Matter, a.AssociationID, a.From, a.Until, a.Revision, now)
	case "merge":
		err = s.Merge(a.Matter, a.Into, a.Revision, now)
	case "split":
		err = s.Split(a.Matter, a.ID, a.Title, a.Associations, a.Revision, now)
	case "cancel":
		err = s.Cancel(a.ID, now)
	case "retry":
		err = s.Retry(a.ID, a.AcknowledgeUncertain, now)
	case "focus":
		err = workspaceFocus(s, a.Matter, a.AssociationID)
	default:
		return nil, errors.New("unknown workspace operation")
	}
	if err != nil {
		diag.Did("act.hq-workspace", a.Matter, diag.Failed, a.Op, "id", a.ID, "association", a.AssociationID)
		return nil, err
	}
	diag.Did("act.hq-workspace", a.Matter, diag.OK, a.Op, "id", a.ID, "association", a.AssociationID)
	return result, nil
}
