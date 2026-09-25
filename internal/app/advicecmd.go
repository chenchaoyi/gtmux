package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/advice"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/humanize"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// `gtmux advice` — what HQ proposed, and what became of it (hq-counsel).
//
// The other ledgers record what the supervisor knows, dispatched and saw. This one records
// what it ADVISED, which is the only one that can answer whether its judgement is worth
// anything and whether it is still speaking up. Writing is gated to the HQ home for the
// same reason knowledge mutations are: it is the supervisor's record of its own work.
// Reading works anywhere, so the commander can check the tally from wherever they are.

// CmdAdvice implements `gtmux advice`.
func CmdAdvice(args []string) int {
	var (
		what, why, about string
		id, outcome      string
		words, since     string
		jsonOut, tally   bool
		openOnly         bool
		limit            = 10
	)
	take := func(i *int, flag string) (string, bool) {
		if *i+1 >= len(args) {
			i18n.Sae("gtmux advice: "+flag+" needs a value", "gtmux advice: "+flag+" 后面要跟内容")
			return "", false
		}
		*i++
		return args[*i], true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--why":
			v, ok := take(&i, a)
			if !ok {
				return 2
			}
			why = v
		case strings.HasPrefix(a, "--why="):
			why = strings.TrimPrefix(a, "--why=")
		case a == "--about":
			v, ok := take(&i, a)
			if !ok {
				return 2
			}
			about = v
		case strings.HasPrefix(a, "--about="):
			about = strings.TrimPrefix(a, "--about=")
		case a == "--taken":
			outcome = advice.Taken
		case a == "--declined":
			outcome = advice.Declined
		case a == "--moot":
			outcome = advice.Moot
		case a == "--words":
			v, ok := take(&i, a)
			if !ok {
				return 2
			}
			words = v
		case strings.HasPrefix(a, "--words="):
			words = strings.TrimPrefix(a, "--words=")
		case a == "--since":
			v, ok := take(&i, a)
			if !ok {
				return 2
			}
			since = v
		case strings.HasPrefix(a, "--since="):
			since = strings.TrimPrefix(a, "--since=")
		case a == "--limit":
			v, ok := take(&i, a)
			if !ok {
				return 2
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				i18n.Sae("gtmux advice: --limit wants a positive number", "gtmux advice: --limit 要一个正整数")
				return 2
			}
			limit = n
		case a == "--json":
			jsonOut = true
		case a == "--tally":
			tally = true
		case a == "--open":
			openOnly = true
		case a == "--list":
			// listing is the default when nothing else is asked for
		case a == "-h" || a == "--help":
			return adviceUsage()
		case strings.HasPrefix(a, "-"):
			i18n.Sae("gtmux advice: no flag called '"+a+"'", "gtmux advice: 没有 '"+a+"' 这个参数")
			return 2
		default:
			// A bare word that names existing advice is an id to mark; anything else is
			// the advice itself.
			if id == "" && what == "" && looksLikeAdviceID(a) {
				id = a
			} else if what == "" {
				what = a
			} else {
				what += " " + a
			}
		}
	}

	live, err := advice.List()
	if err != nil {
		i18n.Sae("gtmux advice: "+err.Error(), "gtmux advice: "+err.Error())
		return 1
	}

	switch {
	case outcome != "":
		if id == "" {
			i18n.Sae("gtmux advice: which one? (gtmux advice a3 --taken)",
				"gtmux advice: 标哪一条？（gtmux advice a3 --taken）")
			return 2
		}
		if !adviceGate() {
			return 1
		}
		if err := advice.Mark(id, outcome, words, advice.Now()); err != nil {
			diag.Did("act.advice", id, diag.Failed, "an outcome was not recorded", "error", err)
			i18n.Sae("gtmux advice: "+err.Error(), "gtmux advice: "+err.Error())
			return 1
		}
		diag.Did("act.advice", id, diag.OK, "recorded what became of a piece of advice", "outcome", outcome)
		i18n.Say("✓ "+id+" "+outcome, "✓ "+id+" "+adviceOutcomeZH(outcome))
		return 0
	case what != "":
		if !adviceGate() {
			return 1
		}
		newID, err := advice.Give(what, why, about, advice.Now())
		if err != nil {
			diag.Did("act.advice", "", diag.Failed, "a piece of advice was not recorded", "error", err)
			i18n.Sae("gtmux advice: "+err.Error(), "gtmux advice: "+err.Error())
			return 1
		}
		diag.Did("act.advice", newID, diag.OK, "recorded a piece of advice", "about", about)
		i18n.Say("✓ recorded as "+newID+" — mark it later with `gtmux advice "+newID+" --taken|--declined|--moot`",
			"✓ 记为 "+newID+" —— 后面用 `gtmux advice "+newID+" --taken|--declined|--moot` 标结果")
		return 0
	case tally:
		return adviceTally(live, since, jsonOut)
	default:
		return adviceList(live, limit, openOnly, jsonOut)
	}
}

// looksLikeAdviceID matches the short ids the ledger mints (a1, a2, …).
func looksLikeAdviceID(s string) bool {
	if len(s) < 2 || s[0] != 'a' {
		return false
	}
	for _, r := range s[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// adviceGate refuses a write from anywhere but the HQ home, and says why.
func adviceGate() bool {
	if advice.FromHQHome() {
		return true
	}
	i18n.Sae("gtmux advice: only the HQ session records advice (run it from "+state.HQHome()+")",
		"gtmux advice: 只有 HQ 会话能记建议（请在 "+state.HQHome()+" 下运行）")
	return false
}

func adviceOutcomeZH(o string) string {
	switch o {
	case advice.Taken:
		return "采纳"
	case advice.Declined:
		return "没采纳"
	default:
		return "已被事情本身解决"
	}
}

func adviceTally(live []advice.Advice, since string, jsonOut bool) int {
	floor, err := advice.ParseSince(since, advice.Now())
	if err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 2
	}
	t := advice.Count(live, floor)
	if jsonOut {
		b, _ := json.Marshal(t)
		fmt.Println(string(b))
		return 0
	}
	window := i18n.Tr("all time", "全部")
	if since != "" && since != "all" {
		window = since
	}
	i18n.Say(fmt.Sprintf("advice over %s: %d given · %d taken · %d declined · %d open · %d overtaken",
		window, t.Given, t.Taken, t.Declined, t.Open, t.Moot),
		fmt.Sprintf("%s的建议：提了 %d 条 · 采纳 %d · 没采纳 %d · 还没结果 %d · 被事情本身解决 %d",
			window, t.Given, t.Taken, t.Declined, t.Open, t.Moot))
	if rate, ok := t.TakenRate(); ok {
		i18n.Say(fmt.Sprintf("  of the %d that got an answer, %.0f%% were taken", t.Taken+t.Declined, rate*100),
			fmt.Sprintf("  有结果的 %d 条里，%.0f%% 被采纳", t.Taken+t.Declined, rate*100))
	}
	if t.Given == 0 {
		i18n.Say("  nothing recorded in this window", "  这段时间里没有记录")
	}
	return 0
}

func adviceList(live []advice.Advice, limit int, openOnly, jsonOut bool) int {
	rows := advice.Recent(live, limit, openOnly)
	if jsonOut {
		if rows == nil {
			rows = []advice.Advice{}
		}
		b, _ := json.Marshal(rows)
		fmt.Println(string(b))
		return 0
	}
	if len(rows) == 0 {
		if openOnly {
			i18n.Say("no advice is waiting on an outcome", "没有还没标结果的建议")
		} else {
			i18n.Say("no advice recorded yet", "还没记过建议")
		}
		return 0
	}
	now := advice.Now()
	for _, a := range rows {
		mark := i18n.Tr("open", "待定")
		if !a.Open() {
			mark = adviceOutcomeLabel(a.Outcome)
		}
		fmt.Printf("  %-4s %-10s %-8s %s\n", a.ID, humanize.AgeShort(now-a.At), mark, a.What)
		if a.Why != "" {
			fmt.Printf("       %s\n", a.Why)
		}
		if a.Words != "" {
			fmt.Printf("       「%s」\n", a.Words)
		}
	}
	return 0
}

func adviceOutcomeLabel(o string) string {
	switch o {
	case advice.Taken:
		return i18n.Tr("taken", "采纳")
	case advice.Declined:
		return i18n.Tr("declined", "没采纳")
	default:
		return i18n.Tr("overtaken", "已解决")
	}
}

func adviceUsage() int {
	i18n.Say(`usage: gtmux advice
  advice "<what you advised>" [--why "<the reasoning>"] [--about "<what it concerns>"]
  advice <id> --taken | --declined [--words "<their answer>"] | --moot
  advice [--open] [--limit N] [--json]      # what was advised, newest first
  advice --tally [--since 30d|12h|all] [--json]

  The supervisor's record of its own counsel. The other ledgers hold what HQ knows,
  dispatched and saw; this one holds what it PROPOSED, which is the only one that can
  answer whether its judgement is any good and whether it is still speaking up at all.
  Recording runs from the HQ home; reading works anywhere. The commander files nothing.`,
		`用法：gtmux advice
  advice "<你建议了什么>" [--why "<依据>"] [--about "<关于什么>"]
  advice <id> --taken | --declined [--words "<他的原话>"] | --moot
  advice [--open] [--limit N] [--json]      # 提过的建议，最新的在前
  advice --tally [--since 30d|12h|all] [--json]

  HQ 给自己记的参谋台账。别的台账记的是它知道什么、派了什么活、看到了什么；
  这一本记的是它提过什么建议。只有这一本能回答：它的判断到底值不值钱，
  以及它是不是还在开口。记录要在 HQ 家目录里跑，查在哪儿都行。司令什么都不用记。`)
	return 0
}
