package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmdPanes implements `gtmux panes [--json]` — the pane-browser producer
// (tiered-pane-control): EVERY tmux pane, not just coding agents, so a sessions/panes
// browser can reach and type into any pane. `--json` is the machine contract; plain
// output is a session → window → pane tree. Read-only; unlike `agents`, it does not
// filter to agent panes. This is the superset; `gtmux agents` stays the agent radar.
func cmdPanes(args []string) int {
	asJSON, watched := false, false
	var watchVerb, watchTarget string
	rest := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-h", "--help":
			usage()
			return 0
		case "--json":
			asJSON = true
		case "--watched":
			watched = true
		case "watch", "unwatch":
			// `gtmux panes watch %N` / `gtmux panes unwatch %N` — opt a plain pane
			// onto (off) the radar (tiered-pane-control). Kept under `panes` (not a
			// top-level `watch`, which would collide with `agents --watch`).
			watchVerb = a
			if i+1 < len(args) {
				i++
				watchTarget = args[i]
			}
		default:
			rest = append(rest, a)
		}
	}
	if watchVerb != "" {
		return paneWatch(watchVerb, watchTarget)
	}
	if watched {
		return paneWatchList()
	}
	_ = rest
	if !tmux.ServerUp() {
		if asJSON {
			fmt.Println("[]")
			return 0
		}
		i18n.Say("No tmux server running.", "tmux server 未运行。")
		return 0
	}
	if asJSON {
		b, err := radar.PanesJSONBytes()
		if err != nil {
			i18n.Sae("panes: "+err.Error(), "panes："+err.Error())
			return 1
		}
		fmt.Println(string(b))
		return 0
	}
	printPaneTree(radar.GatherPanes())
	return 0
}

// printPaneTree renders the panes as a session → window → pane tree. Agent panes are
// marked so the tree still reads as "agents live here, plus everything else."
func printPaneTree(rows []radar.PaneRow) {
	if len(rows) == 0 {
		i18n.Say("No panes.", "没有 pane。")
		return
	}
	// Group by session (first-seen order), then window index (numeric), then pane.
	type key struct{ session, window string }
	sessOrder := []string{}
	seenSess := map[string]bool{}
	byWin := map[key][]radar.PaneRow{}
	winOrder := map[string][]string{}
	seenWin := map[key]bool{}
	for _, r := range rows {
		if !seenSess[r.Session] {
			seenSess[r.Session] = true
			sessOrder = append(sessOrder, r.Session)
		}
		k := key{r.Session, r.Window}
		if !seenWin[k] {
			seenWin[k] = true
			winOrder[r.Session] = append(winOrder[r.Session], r.Window)
		}
		byWin[k] = append(byWin[k], r)
	}
	for _, sess := range sessOrder {
		fmt.Printf("%s\n", sess)
		wins := winOrder[sess]
		sort.Slice(wins, func(i, j int) bool { return winLess(wins[i], wins[j]) })
		for _, w := range wins {
			panes := byWin[key{sess, w}]
			for _, r := range panes {
				mark := " " // plain pane
				label := r.Command
				if r.Tier == "agent" {
					mark = "▸" // an agent lives here
					if r.Agent != "" {
						label = r.Agent
					}
					if r.Title != "" {
						label += " · " + r.Title
					}
				}
				active := " "
				if r.Active {
					active = "*"
				}
				fmt.Printf("  %s %-5s %s %s  %s\n", mark, r.Loc[strings.Index(r.Loc, ":")+1:], active, r.PaneID, label)
			}
		}
	}
}

func winLess(a, b string) bool {
	ai, ae := strconv.Atoi(a)
	bi, be := strconv.Atoi(b)
	if ae == nil && be == nil {
		return ai < bi
	}
	return a < b
}

// paneWatch promotes (`watch`) or demotes (`unwatch`) a plain pane on the radar.
func paneWatch(verb, target string) int {
	if !paneIDRe.MatchString(target) {
		i18n.Sae("usage: gtmux panes "+verb+" <pane-id>", "用法：gtmux panes "+verb+" <pane-id>")
		return 2
	}
	if verb == "unwatch" {
		state.Remove(state.WatchedPath(target))
		i18n.Say("Unwatched "+target+".", "已取消关注 "+target+"。")
		return 0
	}
	if tmux.Bin == "" || tmux.Display(target, "#{pane_id}") == "" {
		i18n.Sae("Pane "+target+" no longer exists", "pane "+target+" 已不存在")
		return 1
	}
	if err := state.Touch(state.WatchedPath(target)); err != nil {
		i18n.Sae("panes watch: "+err.Error(), "panes watch："+err.Error())
		return 1
	}
	i18n.Say("Watching "+target+" — it rides along on the radar as a watched pane.",
		"已关注 "+target+" —— 它会作为「关注的 pane」出现在雷达上。")
	return 0
}

func paneWatchList() int {
	watched := state.WatchedPanes()
	if len(watched) == 0 {
		i18n.Say("No watched panes.", "没有关注中的 pane。")
		return 0
	}
	sort.Strings(watched)
	for _, p := range watched {
		fmt.Println(p)
	}
	return 0
}
