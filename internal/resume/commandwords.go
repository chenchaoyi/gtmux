package resume

import "strings"

// commandWords also reads the quoted recipes produced by CommandWithBridge.
// It only splits text: it never evaluates shell expansions or executes a command.
// ps output can contain literal, unmatched quotes in prompt arguments; retain the
// original Fields fallback for that lossy representation and legacy agents.
func commandWords(line string) []string {
	var words []string
	var word strings.Builder
	var quote byte
	started := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote == '\'' {
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		if c == '\\' {
			if i+1 == len(line) {
				return strings.Fields(line)
			}
			next := line[i+1]
			if quote == '"' && !strings.ContainsRune("$`\"\\\n", rune(next)) {
				word.WriteByte(c)
			} else {
				i++
				if next != '\n' {
					word.WriteByte(next)
				}
			}
			started = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote, started = c, true
		case ' ', '\t', '\n', '\r':
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
		default:
			word.WriteByte(c)
			started = true
		}
	}
	if quote != 0 {
		return strings.Fields(line)
	}
	if started {
		words = append(words, word.String())
	}
	return words
}
