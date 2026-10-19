// Package terminalkeys defines the shared terminal key vocabulary and tmux encoding.
package terminalkeys

//go:generate go run ./gen -out ../../mobileapp/src/api/terminalKeysCatalog.json

import (
	"errors"
	"fmt"
	"strings"
)

const Version = 1

var (
	ErrInvalid     = errors.New("invalid key event")
	ErrUnsupported = errors.New("unsupported key combination")
)

type Event struct {
	Key       string   `json:"key"`
	Modifiers []string `json:"modifiers"`
}

type Key struct {
	Key          string     `json:"key"`
	Label        string     `json:"label"`
	LabelZH      string     `json:"label_zh"`
	Group        string     `json:"group"`
	ModifierSets [][]string `json:"modifier_sets"`
}

var names = []struct{ key, tmux, label, zh string }{
	{"ArrowUp", "Up", "↑", "↑"}, {"ArrowDown", "Down", "↓", "↓"},
	{"ArrowLeft", "Left", "←", "←"}, {"ArrowRight", "Right", "→", "→"},
	{"Home", "Home", "Home", "行首"}, {"End", "End", "End", "行尾"},
	{"PageUp", "PPage", "Page Up", "上页"}, {"PageDown", "NPage", "Page Down", "下页"},
	{"Insert", "IC", "Insert", "插入"}, {"Delete", "DC", "Delete", "删除"},
	{"Backspace", "BSpace", "Backspace", "退格"}, {"Tab", "Tab", "Tab", "Tab"},
	{"Escape", "Escape", "Esc", "Esc"}, {"Enter", "Enter", "Return", "回车"},
	{"Space", "Space", "Space", "空格"},
}

// Encode accepts one bounded event, never a tmux command, key sequence or raw escape.
// Extended character modes are deliberately not assumed: ambiguous combinations fail.
func Encode(e Event) (string, error) {
	mask := 0
	for _, mod := range e.Modifiers {
		bit := 0
		switch mod {
		case "control":
			bit = 1
		case "alt":
			bit = 2
		case "shift":
			bit = 4
		default:
			return "", ErrInvalid
		}
		if mask&bit != 0 {
			return "", ErrInvalid
		}
		mask |= bit
	}
	key := e.Key
	if key == "" || len(key) > 16 {
		return "", ErrInvalid
	}
	base := ""
	for _, n := range names {
		if n.key == key {
			base = n.tmux
			break
		}
	}
	for i := 1; i <= 12; i++ {
		if key == fmt.Sprintf("F%d", i) {
			base = key
			break
		}
	}
	if base != "" {
		if key == "Tab" && mask == 4 {
			return "BTab", nil
		}
		if (key == "Tab" || key == "Enter" || key == "Escape" || key == "Backspace") && mask&5 != 0 {
			return "", ErrUnsupported
		}
		if key == "Space" && mask&4 != 0 {
			return "", ErrUnsupported
		}
		// Function/navigation keys have standard modified-key sequences in tmux.
	} else {
		if len(key) != 1 || key[0] < 33 || key[0] > 126 {
			return "", ErrInvalid
		}
		c := key[0]
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if mask&1 != 0 {
			if mask&4 != 0 || !(letter || strings.ContainsRune("@[\\]^_", rune(c))) {
				return "", ErrUnsupported
			}
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
		} else if mask&4 != 0 {
			if !letter {
				return "", ErrUnsupported
			}
			if c >= 'a' && c <= 'z' {
				c -= 'a' - 'A'
			}
			mask &^= 4
		}
		base = string(c)
	}
	prefix := ""
	if mask&1 != 0 {
		prefix += "C-"
	}
	if mask&2 != 0 {
		prefix += "M-"
	}
	if mask&4 != 0 {
		prefix += "S-"
	}
	return prefix + base, nil
}

func Catalog() []Key {
	var out []Key
	add := func(key, label, zh, group string) {
		sets := make([][]string, 0, 8)
		for mask := 0; mask < 8; mask++ {
			mods := make([]string, 0, 3)
			if mask&1 != 0 {
				mods = append(mods, "control")
			}
			if mask&2 != 0 {
				mods = append(mods, "alt")
			}
			if mask&4 != 0 {
				mods = append(mods, "shift")
			}
			if _, err := Encode(Event{key, mods}); err == nil {
				sets = append(sets, mods)
			}
		}
		out = append(out, Key{key, label, zh, group, sets})
	}
	for _, n := range names {
		add(n.key, n.label, n.zh, "navigation")
	}
	for i := 1; i <= 12; i++ {
		k := fmt.Sprintf("F%d", i)
		add(k, k, k, "function")
	}
	for _, c := range "abcdefghijklmnopqrstuvwxyz0123456789@[\\]^_/?.,;:-=+" {
		add(string(c), string(c), string(c), "characters")
	}
	return out
}

// Legacy preserves the audited v0 key contract. New combinations use Event instead.
var legacyNames = []string{"Enter", "C-c", "Escape", "Tab", "Up", "Down", "Left", "Right", "Space", "BSpace", "C-d", "C-z", "C-l"}

func Legacy(key string) bool {
	for _, name := range legacyNames {
		if name == key {
			return true
		}
	}
	return false
}

func LegacyKeys() map[string]bool {
	out := make(map[string]bool)
	for _, key := range legacyNames {
		out[key] = true
	}
	return out
}

func LegacyEvents() map[string]Event {
	out := map[string]Event{}
	for _, n := range names {
		if Legacy(n.tmux) {
			out[n.tmux] = Event{Key: n.key, Modifiers: []string{}}
		}
	}
	for _, c := range "cdzl" {
		out["C-"+string(c)] = Event{Key: string(c), Modifiers: []string{"control"}}
	}
	return out
}

type Context struct {
	Boot        string `json:"boot"`
	PaneBinding string `json:"pane_binding"`
	ExpiresAt   int64  `json:"expires_at"`
	Proof       string `json:"proof"`
}

type Action struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Label    string `json:"label"`
	LabelZH  string `json:"label_zh"`
	Count    int    `json:"count,omitempty"`
	Event    Event  `json:"event"`
}

// Target is a read-only observation. Binding changes when the terminal input route changes.
type Target struct {
	Binding string
	Actions []Action
}

type ActionRequest struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

// CharacterBytes encodes one validated printable ASCII shortcut using classic
// terminal bytes. This also avoids tmux interpreting a semicolon inside M-;.
// The caller cannot supply arbitrary byte sequences.
func CharacterBytes(e Event) ([]byte, bool) {
	if _, err := Encode(e); err != nil || len(e.Key) != 1 {
		return nil, false
	}
	c := e.Key[0]
	alt, control, shift := false, false, false
	for _, m := range e.Modifiers {
		alt = alt || m == "alt"
		control = control || m == "control"
		shift = shift || m == "shift"
	}
	if control {
		c &= 31
	} else if shift && c >= 'a' && c <= 'z' {
		c -= 32
	}
	b := []byte{c}
	if alt {
		b = append([]byte{27}, b...)
	}
	return b, true
}
