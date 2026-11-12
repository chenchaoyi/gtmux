package knowledge

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func memoryJSON(value any, err error) int {
	if err != nil {
		i18n.Sae("gtmux knowledge: "+err.Error(), "gtmux knowledge: "+err.Error())
		return 1
	}
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 1
	}
	return 0
}

func readMemoryJSON(path string, target any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() > 512<<10 {
		return fmt.Errorf("request exceeds 512 KiB")
	}
	d := json.NewDecoder(io.LimitReader(f, (512<<10)+1))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("request must contain exactly one JSON value")
	}
	return nil
}

func knowledgeMemoryUsage(args []string) int {
	if len(args) == 1 && args[0] == "summary" {
		r, err := SummarizeUsage(time.Now().Add(-30 * 24 * time.Hour).Unix())
		return memoryJSON(r, err)
	}
	if len(args) == 2 && args[0] == "record" {
		var r UsageReceipt
		if err := readMemoryJSON(args[1], &r); err != nil {
			return memoryJSON(nil, err)
		}
		out, err := RecordUsage(r)
		return memoryJSON(out, err)
	}
	return memoryJSON(nil, fmt.Errorf("usage record <receipt.json> | summary (last 30 days; reported stages only)"))
}

func knowledgeDream(args []string) int {
	if len(args) == 3 && args[0] == "extend" {
		return knowledgeMutation(func() error {
			var q DreamAllowance
			if err := readMemoryJSON(args[2], &q); err != nil {
				return err
			}
			r, err := ExtendDream(args[1], q)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(r)
		})
	}
	if len(args) == 3 && args[0] == "defer" {
		return knowledgeMutation(func() error {
			var q struct {
				ItemID string `json:"item_id"`
				Reason string `json:"reason"`
			}
			if err := readMemoryJSON(args[2], &q); err != nil {
				return err
			}
			r, err := DeferDream(args[1], q.ItemID, q.Reason)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(r)
		})
	}
	if len(args) == 1 && args[0] == "list" {
		r, err := DreamRuns()
		return memoryJSON(r, err)
	}
	if len(args) == 2 {
		switch args[0] {
		case "start":
			return knowledgeMutation(func() error {
				var q DreamStart
				if err := readMemoryJSON(args[1], &q); err != nil {
					return err
				}
				r, err := StartDream(q)
				if err != nil {
					return err
				}
				return json.NewEncoder(os.Stdout).Encode(r)
			})
		case "status":
			r, err := DreamStatus(args[1])
			return memoryJSON(r, err)
		case "history":
			r, err := DreamHistory(args[1])
			return memoryJSON(r, err)
		case "proposal":
			r, err := DreamProposal(args[1])
			return memoryJSON(r, err)
		}
	}
	if len(args) != 2 || args[0] != "plan" {
		return memoryJSON(nil, fmt.Errorf("dream plan <request.json> | start <run.json> | list | status|history|proposal <run-id> | run <run-id> <HQ-choice.json> | defer <run-id> <reason.json> | extend <run-id> <allowance.json>"))
	}
	var q DreamRequest
	if err := readMemoryJSON(args[1], &q); err != nil {
		return memoryJSON(nil, err)
	}
	p, err := PlanDream(q)
	return memoryJSON(p, err)
}

type toolVersionsFlag map[string]string

func (v toolVersionsFlag) String() string { return "name=version" }
func (v toolVersionsFlag) Set(s string) error {
	name, version, ok := strings.Cut(s, "=")
	if !ok || name == "" || version == "" {
		return fmt.Errorf("tool must be name=version")
	}
	v[name] = version
	return nil
}

func knowledgeConsult(args []string) int {
	var q ConsultRequest
	tools := toolVersionsFlag{}
	f := flag.NewFlagSet("consult", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&q.Query, "query", "", "task query")
	f.StringVar(&q.Action, "action", "", "action")
	f.StringVar(&q.Project, "repo", "", "project path")
	f.StringVar(&q.Role, "role", "worker", "worker or hq")
	f.IntVar(&q.TokenBudget, "budget", 1200, "estimated tokens")
	f.StringVar(&q.Purpose, "purpose", "task", "task, audit, evaluation")
	f.BoolVar(&q.IncludeLegacy, "include-legacy", false, "include unscoped notes for local HQ investigation")
	f.Var(tools, "tool", "exact tool version")
	f.Bool("json", true, "structured result")
	if err := f.Parse(args); err != nil {
		return memoryJSON(nil, err)
	}
	if f.NArg() == 1 && q.Query == "" {
		q.Query = f.Arg(0)
	} else if f.NArg() != 0 {
		return memoryJSON(nil, fmt.Errorf("consult takes --query text [--action action] [--repo path] [--tool name=version] [--budget 1200]"))
	}
	if q.Query == "" && q.Action == "" {
		return memoryJSON(nil, fmt.Errorf("consult needs a query or action"))
	}
	q.Tools = tools
	r, err := Consult(q)
	return memoryJSON(r, err)
}

func knowledgeReview(args []string) int {
	if len(args) != 2 {
		return memoryJSON(nil, fmt.Errorf("review export|diff <vault-directory> | check|apply <proposal.json>"))
	}
	switch args[0] {
	case "export":
		r, err := ExportVault(args[1])
		return memoryJSON(r, err)
	case "diff":
		r, err := DiffVault(args[1])
		if err != nil {
			return memoryJSON(nil, err)
		}
		p := *r.Proposal
		p.Conflicts, p.Missing, p.Drafts = r.Conflicts, r.Missing, r.Drafts
		p.ID = proposalDigest(p)
		return memoryJSON(p, nil)
	case "check":
		p, err := readReview(args[1])
		if err != nil {
			return memoryJSON(nil, err)
		}
		r, err := ApplyReview(p, true)
		return memoryJSON(r, err)
	case "apply":
		return knowledgeMutation(func() error {
			p, err := readReview(args[1])
			if err != nil {
				return err
			}
			r, err := ApplyReview(p, false)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(r)
		})
	default:
		return memoryJSON(nil, fmt.Errorf("unknown review operation %q", args[0]))
	}
}
