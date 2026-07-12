package resource

import (
	"fmt"
	"regexp"
	"strings"
)

// agentVersionRe matches a bare semver command like "2.1.207" — how Claude Code
// reports its process command.
var agentVersionRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// attribute walks the ps snapshot to (a) sum each live pane's process-tree RSS +
// CPU, and (b) find reclaim candidates — heavy processes under NO live pane and
// not whitelisted (the general rule), with a curated pattern set raising the
// confidence + reclaim hint.
func attribute(procs []proc, panePIDs map[string]int, cfg config) (map[string]AgentUse, []Orphan) {
	byPID := make(map[int]proc, len(procs))
	children := make(map[int][]int, len(procs))
	for _, p := range procs {
		byPID[p.pid] = p
		children[p.ppid] = append(children[p.ppid], p.pid)
	}

	// owned[pid] = true when pid is in SOME live pane's subtree (so it's not an
	// orphan). Also sum per-pane RSS/CPU over the subtree.
	owned := make(map[int]bool, len(procs))
	agents := make(map[string]AgentUse)
	for pane, root := range panePIDs {
		if root <= 0 {
			continue
		}
		var rssKB int
		var cpu float64
		walk(root, children, byPID, func(p proc) {
			owned[p.pid] = true
			rssKB += p.rssKB
			cpu += p.cpu
		})
		if rssKB > 0 || cpu > 0 {
			agents[pane] = AgentUse{RSSMB: rssKB / 1024, CPU: round1(cpu)}
		}
	}

	// Reclaim candidates. Two lanes:
	//  - CURATED patterns (simulator / dev-server / tmux) surface even at low
	//    per-process RSS — they're KNOWN leftovers. Simulator processes are MANY
	//    tiny helpers, so they aggregate into ONE named entry (total RSS + the
	//    shutdown hint) rather than flooding the list.
	//  - the GENERIC heuristic (heavy + unowned + not whitelisted + not an agent)
	//    needs the RSS floor to avoid noise.
	var orphans []Orphan
	var simRSS, simCPU, simCount, simPID int
	for _, p := range procs {
		if owned[p.pid] || !validComm(p.comm) || isWhitelisted(p.comm) || isAgentProcess(p.comm) {
			continue
		}
		mb := p.rssKB / 1024
		kind, hint := classifyReclaim(p.comm)
		switch {
		case kind == "simulator":
			simRSS += mb
			simCPU += int(p.cpu*10 + 0.5)
			simCount++
			if simPID == 0 {
				simPID = p.pid
			}
		case kind != "": // dev-server / tmux — surface individually (port/sock matters)
			orphans = append(orphans, Orphan{PID: p.pid, RSSMB: mb, CPU: round1(p.cpu), Comm: baseComm(p.comm), Kind: kind, Hint: hint})
		case mb >= cfg.OrphanRSSMB: // generic heavy orphan
			orphans = append(orphans, Orphan{PID: p.pid, RSSMB: mb, CPU: round1(p.cpu), Comm: baseComm(p.comm), Hint: hint})
		}
	}
	if simCount > 0 {
		orphans = append(orphans, Orphan{
			PID: simPID, RSSMB: simRSS, CPU: round1(float64(simCPU) / 10),
			Comm: fmt.Sprintf("iOS Simulator runtime (%d procs)", simCount), Kind: "simulator",
			Hint: "leftover iOS Simulator runtime — `xcrun simctl shutdown all`",
		})
	}
	sortOrphansByRSS(orphans)
	return agents, orphans
}

// isAgentProcess reports whether comm looks like a coding-agent process (Claude
// reports its version as its command, e.g. "2.1.207"; others by name). We never
// suggest reclaiming an agent — one outside a pane is likely a native session,
// not garbage.
func isAgentProcess(comm string) bool {
	c := strings.ToLower(baseComm(comm))
	if agentVersionRe.MatchString(c) {
		return true
	}
	for _, a := range []string{"claude", "codex", "cursor", "gemini", "aider", "opencode", "crush", "amp"} {
		if c == a {
			return true
		}
	}
	return false
}

// walk applies fn to root and its whole subtree (guards against cycles via the
// finite process set — each pid visited once).
func walk(root int, children map[int][]int, byPID map[int]proc, fn func(proc)) {
	seen := map[int]bool{}
	var rec func(int)
	rec = func(pid int) {
		if seen[pid] {
			return
		}
		seen[pid] = true
		if p, ok := byPID[pid]; ok {
			fn(p)
		}
		for _, c := range children[pid] {
			rec(c)
		}
	}
	rec(root)
}

// isWhitelisted excludes processes that are legitimately long-lived and NOT
// something a session leaked: gtmux/tmux itself, the OS, the login shell, the
// coding-agent app bundles, browsers, editors. Only genuinely reclaimable
// leftovers should surface.
func isWhitelisted(comm string) bool {
	c := strings.ToLower(comm)
	for _, w := range []string{
		"/sbin/launchd", "kernel_task", "windowserver", "loginwindow", "gtmux",
		"/usr/sbin/", "/usr/libexec/", "/system/library/", "com.apple.",
		"/applications/", "cloudflared", "clashx", "clash", "ssh-agent",
	} {
		if strings.Contains(c, w) {
			return true
		}
	}
	return false
}

// classifyReclaim tags a known leftover kind + a reclaim hint (curated set;
// raises confidence over the bare heuristic). "" kind = generic orphan.
func classifyReclaim(comm string) (kind, hint string) {
	c := strings.ToLower(comm)
	switch {
	case strings.Contains(c, "coresimulator") || strings.Contains(c, "simulator") || strings.Contains(c, "launchd_sim"):
		return "simulator", "leftover iOS Simulator runtime — `xcrun simctl shutdown all` / kill the pid"
	case strings.Contains(c, "vite") || strings.Contains(c, "webpack") || strings.Contains(c, "next-server") ||
		(strings.Contains(c, "node") && strings.Contains(c, "dev")):
		return "dev-server", "a dev server left running — check its port and kill the pid"
	case strings.Contains(c, "tmux: server") || strings.Contains(c, "tmux server"):
		return "tmux", "a stray tmux server — `tmux -L <sock> kill-server` or kill the pid"
	case strings.Contains(c, "metro"):
		return "dev-server", "a Metro bundler left running — kill the pid"
	default:
		return "", "not owned by any live agent — verify before killing the pid"
	}
}

// baseComm shortens a full command path to its basename for display.
// validComm sanity-filters a process command before it can become a reclaim
// candidate: reject empty, and reject a leading '-' — that marks a LOGIN SHELL
// ("-zsh", "-bash", "-/bin/bash") or junk argv[0] (e.g. a process whose comm ps
// reports as "-PPID"), none of which is a meaningful thing to suggest killing.
func validComm(comm string) bool {
	c := strings.TrimSpace(comm)
	return c != "" && !strings.HasPrefix(c, "-")
}

func baseComm(comm string) string {
	comm = strings.TrimSpace(comm)
	if i := strings.LastIndexByte(strings.Fields(comm)[0], '/'); i >= 0 {
		return comm[i+1:]
	}
	return comm
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }

// sortOrphansByRSS orders reclaim candidates heaviest-first (simple insertion —
// the list is short).
func sortOrphansByRSS(o []Orphan) {
	for i := 1; i < len(o); i++ {
		for j := i; j > 0 && o[j].RSSMB > o[j-1].RSSMB; j-- {
			o[j], o[j-1] = o[j-1], o[j]
		}
	}
}
