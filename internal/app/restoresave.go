package app

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

// Reading a tmux-resurrect save as EVIDENCE of what was running in each pane at the
// moment the snapshot was taken.
//
// This exists because restore used to answer the wrong question. A resume record is
// written by the agent's hooks and NEVER pruned, so it says "an agent ran in this
// pane at some point" — a permanent property of a locator. Restore treated that as
// "an agent was running here", and so relaunched a conversation into every pane that
// had ever hosted one, including panes that had been plain interactive shells for
// days. Observed 2026-08-04: a `日常更新:0.0` pane doing routine binary upgrades came
// back with `claude --resume 369b78a0…` typed into it, sitting at the trust gate;
// across the fleet 10 live agent panes restored as 16, and one of the six phantoms
// carried 33.7M tokens of a four-day-old conversation — enough to trip gtmux's own
// usage·warn burn alarm. It repeats every reboot because each injection re-creates
// the very record (and, on the next autosave, the very pane_full_command) that
// justifies the next one.
//
// The save itself holds the missing evidence. Each `pane` line records that pane's
// `pane_current_command` and `pane_full_command` at save time — the process is long
// dead when restore runs, so this file is the ONLY witness to what was live.
//
// # The empty-title field shift (do not "simplify" the parser)
//
// tmux-resurrect's save.sh re-reads its own raw dump with `while IFS=$d read …`.
// The delimiter is a TAB, and a tab is IFS *whitespace*, so bash collapses a run of
// them into one separator: a pane with an EMPTY title loses its field entirely and
// every later field shifts left by one. resurrect then re-emits the shifted values,
// so those lines carry the pane's pid where the command belongs and a useless full
// command (it was computed from the wrong pid). This is not theoretical — four of
// the six phantom sessions in the incident above were on shifted lines, including
// the reported one. Reading the fixed column would have called `bash` a live agent
// and left the bug exactly where it was. The dir field's `:` prefix is what tells
// the two layouts apart.
type savedPane struct {
	Session string
	Window  string
	Pane    string
	Loc     string // "session:window.pane" — the same locator resume records use
	Dir     string // pane_current_path at save time
	Cmd     string // pane_current_command ("bash", or for Claude Code its VERSION)
	Full    string // pane_full_command ("" when resurrect recorded none)
	Shifted bool   // this line lost its title field (see above); Full is unusable
}

// savedLayout is a parsed resurrect save: the panes in file order (the plan renders
// them that way) plus a locator index (restore looks panes up by locator).
type savedLayout struct {
	Panes []savedPane
	ByLoc map[string]savedPane
	Ref   time.Time // the save's mtime — "when this evidence was true"
}

// paneEvidence is what a save says about one pane at save time.
type paneEvidence int

const (
	evidenceMissing paneEvidence = iota // no line for this pane in the save
	evidenceShell                       // an interactive shell — nothing was running
	evidenceOther                       // a program that is not an agent (vim, ssh…)
	evidenceAgent                       // an agent process was running
	evidenceUnclear                     // something ran, but the save can't name it
)

func (e paneEvidence) String() string {
	switch e {
	case evidenceShell:
		return "shell"
	case evidenceOther:
		return "non-agent"
	case evidenceAgent:
		return "agent"
	case evidenceUnclear:
		return "unclear"
	}
	return "missing"
}

// allowsResume reports whether restore may relaunch a conversation into this pane.
//
// The two denials that matter are `shell` (the pane was idle — the bug this fixes)
// and `missing` (the pane isn't in the save at all, so restore did not create it and
// has no business typing into it). `unclear` ALLOWS deliberately: the save's full
// command is missing on every shifted line, and a live Claude Code pane's
// `pane_current_command` is its version string ("2.1.220"), which no table can
// recognize. Denying those would trade this bug for its mirror image — a live
// conversation silently NOT coming back, which has bitten this user twice — so
// ambiguity resolves in favour of restoring, and the log says which panes took that
// branch.
func (e paneEvidence) allowsResume() bool { return e == evidenceAgent || e == evidenceUnclear }

// evidence classifies what this saved pane was doing.
func (sp savedPane) evidence() paneEvidence {
	if sp.Full != "" {
		// Agent first, shell second: an agent installed as a shebang script runs as
		// `/bin/sh /usr/local/bin/<agent> …`, and reading only argv[0] would file it
		// as an idle shell and lose the conversation.
		if key, _ := resume.FromCommand(sp.Full); key != "" {
			return evidenceAgent
		}
		if isShellCommand(argv0(sp.Full)) {
			return evidenceShell
		}
		return evidenceOther // we know what ran, and it wasn't an agent
	}
	// No full command recorded: the pane had no child process (an idle shell), or
	// this is a shifted line. Fall back to pane_current_command.
	if sp.Cmd == "" {
		return evidenceUnclear
	}
	if isShellCommand(sp.Cmd) {
		return evidenceShell
	}
	if key, _ := resume.FromCommand(sp.Cmd); key != "" {
		return evidenceAgent
	}
	return evidenceUnclear
}

// savedSessionID is the conversation id this pane's own command line was resuming
// ("" when the save recorded none). It is the fallback source when a pane that
// demonstrably hosted an agent has no resume record — the record store missing an
// entry must not cost the user a live conversation.
func (sp savedPane) savedSessionID() (agent, id string) {
	if sp.Full == "" {
		return "", ""
	}
	return resume.FromCommand(sp.Full)
}

// argv0 is a command line's leading token ("" for an empty line).
func argv0(cmdline string) string {
	if i := strings.IndexAny(cmdline, " \t"); i >= 0 {
		return filepath.Base(cmdline[:i])
	}
	return filepath.Base(cmdline)
}

// loadSavedLayout parses a resurrect save. An empty layout (missing/unreadable save,
// or no parseable pane lines) is the caller's signal that there is NO evidence to
// gate on — callers must then behave as they did before this file existed rather
// than deny every resume.
func loadSavedLayout(path string) savedLayout {
	out := savedLayout{ByLoc: map[string]savedPane{}}
	if path == "" {
		return out
	}
	if fi, err := os.Stat(path); err == nil {
		out.Ref = fi.ModTime()
	}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		sp, ok := parseSavedPaneLine(sc.Text())
		if !ok {
			continue
		}
		out.Panes = append(out.Panes, sp)
		if _, dup := out.ByLoc[sp.Loc]; !dup {
			out.ByLoc[sp.Loc] = sp
		}
	}
	return out
}

// parseSavedPaneLine parses one `pane` line of a resurrect save, handling BOTH field
// layouts (see the savedPane doc for why the shifted one exists).
//
// Normal:  pane │ session │ win │ win_active │ :win_flags │ pane │ title │ :dir │ active │ cmd │ … │ :full
// Shifted: pane │ session │ win │ win_active │ :win_flags │ pane │ :dir  │ active │ cmd │ pid │ :garbage
//
// The discriminator is the `:` that resurrect's format literally prefixes onto the
// directory: it lands at index 7 normally and at index 6 when the title collapsed.
// Checking index 7 FIRST matters — a pane title can itself start with `:` (bash
// title-setting prologues produce exactly that), and only the shifted layout has a
// bare `0`/`1` (pane_active) sitting at index 7.
func parseSavedPaneLine(line string) (savedPane, bool) {
	if !strings.HasPrefix(line, "pane\t") {
		return savedPane{}, false
	}
	f := strings.Split(line, "\t")
	if len(f) < 9 || f[1] == "" {
		return savedPane{}, false
	}
	sp := savedPane{Session: f[1], Window: f[2], Pane: f[5]}
	sp.Loc = sp.Session + ":" + sp.Window + "." + sp.Pane
	switch {
	case strings.HasPrefix(f[7], ":") && len(f) >= 10:
		sp.Dir = unescapeSavedDir(f[7])
		sp.Cmd = f[9]
		// The full command is the LAST field (resurrect appends `:${full_command}`
		// after dropping pid/history_size). Older/other writers pad differently, so
		// take it by position from the end, not a fixed index.
		if last := f[len(f)-1]; len(f) >= 11 && strings.HasPrefix(last, ":") {
			sp.Full = strings.TrimPrefix(last, ":")
		}
	case strings.HasPrefix(f[6], ":"):
		sp.Shifted = true
		sp.Dir = unescapeSavedDir(f[6])
		sp.Cmd = f[8]
		// Full is deliberately left empty: on a shifted line resurrect computed it
		// from the wrong pid, so whatever is there describes some other process.
	default:
		return savedPane{}, false // a layout we don't recognize — claim nothing
	}
	return sp, true
}

// unescapeSavedDir strips the format's `:` prefix and un-escapes the spaces
// resurrect escapes when it writes the directory.
func unescapeSavedDir(field string) string {
	return strings.ReplaceAll(strings.TrimPrefix(field, ":"), `\ `, " ")
}
