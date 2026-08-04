package agents

// The accessors below project the manifests into the exact shapes the individual
// subsystems consume today. Each replaces one hand-kept, agent-keyed list. They
// are pinned to the legacy values by registry_test.go's golden snapshots, so a
// subsystem can switch from its private map to the accessor with no behavior
// change.

// HookEquippedKeys returns every key (canonical + aliases) whose events feed the
// receipt/ready stream — the driver's hook-equipped set.
func HookEquippedKeys() []string {
	var out []string
	for _, a := range manifests {
		if !a.Hooked {
			continue
		}
		out = append(out, a.Key)
		out = append(out, a.Aliases...)
	}
	return out
}

// Profiles returns the radar's built-in detection profiles, in the legacy order.
func Profiles() []Profile {
	out := make([]Profile, 0, len(profileOrder))
	for _, key := range profileOrder {
		a := byKey[key]
		out = append(out, Profile{Label: a.Label, Commands: a.Detect, IdleGlyph: a.IdleGlyph, Icon: a.Icon})
	}
	return out
}

// KeyForLabel resolves an agent's canonical key from its display label (e.g.
// "Claude Code" → "claude", "opencode" → "opencode"). "" when no manifest matches.
// Used where a surface has only the display name (the icon endpoint) but needs the
// key to look up a committed icon.
func KeyForLabel(label string) string {
	for _, a := range manifests {
		if a.Label == label {
			return a.Key
		}
	}
	return ""
}

// DisplayNames maps each hook-registered key to its display label (the hook-time
// known-agent gate).
func DisplayNames() map[string]string {
	out := make(map[string]string)
	for _, a := range manifests {
		if a.HookDisplay {
			out[a.Key] = a.Label
		}
	}
	return out
}

// ResumeArgv maps each resumable agent key to the argv that relaunches a session.
func ResumeArgv() map[string][]string {
	out := make(map[string][]string)
	for _, a := range manifests {
		if a.Resume != nil {
			out[a.Key] = a.Resume
		}
	}
	return out
}

// CommandKeys maps every EXECUTABLE NAME that identifies an agent to its canonical
// key — the radar's detection commands plus the binary each resume argv launches
// ("claude"→claude, "cursor-agent"→cursor, "kiro-cli"→kiro).
//
// It exists for the one place gtmux has to identify an agent from a command line it
// cannot inspect as a process: the `pane_full_command` recorded in a tmux-resurrect
// save, read at restore time when that process is long dead. Identity stays in the
// registry rather than a private table in the restore code.
func CommandKeys() map[string]string {
	out := make(map[string]string)
	for _, a := range manifests {
		for _, c := range a.Detect {
			out[c] = a.Key
		}
		if len(a.Resume) > 0 {
			out[a.Resume[0]] = a.Key
		}
	}
	return out
}

// ResourceNames returns the agent names the resource attributor scans, in order.
func ResourceNames() []string {
	out := make([]string, 0, len(resourceOrder))
	for _, key := range resourceOrder {
		if a, ok := byKey[key]; ok && a.Resource != "" {
			out = append(out, a.Resource)
		}
	}
	return out
}

// ContentKeys returns the keys with a transcript parser (digest content source).
func ContentKeys() []string {
	var out []string
	for _, a := range manifests {
		if a.Content != "" {
			out = append(out, a.Key)
		}
	}
	return out
}

// HeadlessKeys returns the keys with a headless one-shot mode.
func HeadlessKeys() []string {
	var out []string
	for _, a := range manifests {
		if a.Headless != "" {
			out = append(out, a.Key)
		}
	}
	return out
}

// DedicatedSemanticsKeys returns the keys with a dedicated classifier
// event-semantics table (all others use the generic table).
func DedicatedSemanticsKeys() []string {
	var out []string
	for _, a := range manifests {
		if a.Semantics {
			out = append(out, a.Key)
		}
	}
	return out
}
