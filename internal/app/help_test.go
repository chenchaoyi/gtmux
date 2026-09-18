package app

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// The old help was one 129-line wall printed for every command. These pin the three
// things that made it cost real turns: a command you can run that the help never
// names, a screen too wide to read, and a flag whose accepted values existed only
// inside the error you got for guessing wrong.

// internal plumbing and back-compat aliases — the same set check-design.sh keeps out
// of the CLAUDE.md command list, for the same reason: you never type them.
var notYourCommands = map[string]bool{
	"tunnel-client": true, "save-tab-order": true, "options": true,
	"oneshot-run": true, "server-mode": true, "install-hooks": true,
	"uninstall-hooks": true, "uninstall-app": true,
	"help": true, "version": true,
}

func dispatchedCommands(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatalf("read app.go: %v", err)
	}
	s := string(src)
	i := strings.Index(s, "\tswitch sub {")
	if i < 0 {
		t.Fatal("app.go no longer has the `switch sub {` dispatch this test reads")
	}
	body := s[i:]
	if j := strings.Index(body, "\n\t}\n"); j > 0 {
		body = body[:j]
	}
	var out []string
	for _, m := range regexp.MustCompile(`case "([a-z][a-z0-9-]*)"`).FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	if len(out) < 20 {
		t.Fatalf("only found %d dispatched commands; the switch shape changed", len(out))
	}
	return out
}

func TestEveryCommandYouCanRunIsInTheHelp(t *testing.T) {
	for _, name := range dispatchedCommands(t) {
		if notYourCommands[name] {
			continue
		}
		if findCommand(name) == nil {
			t.Errorf("`gtmux %s` runs but the help table never names it — add it to helpCommands", name)
		}
	}
}

func TestTheHelpFitsAnEightyColumnTerminal(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		screens := map[string]string{"(the screen)": usageText()}
		for _, c := range helpCommands {
			screens[c.Name] = commandHelpText(c.Name)
		}
		for name, text := range screens {
			for _, line := range strings.Split(text, "\n") {
				if w := i18n.DispWidth(line); w > 80 {
					t.Errorf("%s/%s: a %d-column line wraps on an 80-column terminal:\n%s", name, lang, w, line)
				}
			}
		}
	}
	i18n.SetLang("en")
}

func TestAFlagSaysWhatItTakesBeforeYouGuessWrong(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	i18n.SetLang("en")
	got := commandHelpText("knowledge")
	for _, kind := range []string{"facts", "howto", "pitfalls", "judgment", "decisions"} {
		if !strings.Contains(got, kind) {
			t.Errorf("knowledge --help does not name the --kind value %q", kind)
		}
	}
	if !strings.Contains(got, "300 bytes") {
		t.Error("knowledge --help does not say --why is refused above 300 bytes")
	}
	if !strings.Contains(got, "--confirmed") {
		t.Error("knowledge --help does not say --sensitive must come with --confirmed")
	}
}

func TestOneCommandsHelpIsOnlyThatCommand(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	i18n.SetLang("en")
	got := commandHelpText("focus")
	if !strings.Contains(got, "gtmux focus") {
		t.Fatalf("focus --help does not lead with the command:\n%s", got)
	}
	if strings.Contains(got, "SET UP AND UPDATE") || strings.Contains(got, "gtmux tunnel") {
		t.Errorf("focus --help is printing the whole screen again:\n%s", got)
	}
	if n := len(strings.Split(strings.TrimSpace(got), "\n")); n > 12 {
		t.Errorf("focus --help is %d lines; it used to be the 129-line wall", n)
	}
}

func TestTheScreenSaysWhatARunWillTouch(t *testing.T) {
	i18n.SetLang("en")
	agents := findCommand("agents")
	if agents.Writes {
		t.Error("agents is marked as writing; it reads")
	}
	if got := modeOf(agents, false); got != "reads only" {
		t.Errorf("agents reports mode %q, want %q", got, "reads only")
	}
	if got := modeOf(findCommand("focus"), false); got != "moves your terminal" {
		t.Errorf("focus reports mode %q", got)
	}
}

func TestTheJSONCarriesWhatAnAgentNeeds(t *testing.T) {
	var doc helpJSON
	// Build the same value usageJSON encodes, without capturing stdout.
	for _, g := range helpGroups {
		doc.Groups = append(doc.Groups, groupJSON{ID: g.ID, Title: g.EN, TitleZH: g.ZH, ReadsOnly: g.ReadsOnly})
	}
	for _, c := range helpCommands {
		cj := commandJSON{Name: c.Name, Group: c.Group, Summary: c.EN, SummaryZH: c.ZH, Writes: c.Writes}
		for _, f := range c.Flags {
			cj.Flags = append(cj.Flags, flagJSON{Name: f.Name, Summary: f.EN, Values: f.Values, MaxBytes: f.MaxBytes, Requires: f.Requires})
		}
		doc.Cmds = append(doc.Cmds, cj)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, c := range doc.Cmds {
		if c.Summary == "" || c.SummaryZH == "" {
			t.Errorf("%s ships only one language half", c.Name)
		}
		if c.Summary == c.SummaryZH {
			t.Errorf("%s has the same line in both languages — one half was never written", c.Name)
		}
		if c.Group == "" {
			t.Errorf("%s belongs to no group", c.Name)
		}
	}
	var kind flagJSON
	for _, c := range doc.Cmds {
		if c.Name != "knowledge" {
			continue
		}
		for _, f := range c.Flags {
			if strings.HasPrefix(f.Name, "--kind") {
				kind = f
			}
		}
	}
	if len(kind.Values) != 5 {
		t.Errorf("--kind carries %d values in the JSON, want the 5 it accepts", len(kind.Values))
	}
}
