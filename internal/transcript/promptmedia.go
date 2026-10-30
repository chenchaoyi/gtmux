package transcript

import (
	"net/url"
	"regexp"
	"strings"
)

// The observed Codex attachment envelope is data, not arbitrary HTML. Normalize it
// once in the public transcript so phone, tablet and browser receive the same link.
var promptImage = regexp.MustCompile(`^<image name=\[([^\n<>]{1,120})\] path="([^"\n<>]+)">[ \t]*</image>[ \t]*`)

func normalizePromptMedia(prompt string) string {
	if !strings.Contains(prompt, "<image name=[") {
		return prompt
	}
	lines := strings.Split(prompt, "\n")
	out := make([]string, 0, len(lines))
	fence := byte(0)
	fenceWidth := 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if indent <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			width := 0
			for width < len(trimmed) && trimmed[width] == trimmed[0] {
				width++
			}
			if width >= 3 {
				if fence == 0 && (trimmed[0] != '`' || !strings.Contains(trimmed[width:], "`")) {
					fence, fenceWidth = trimmed[0], width
				} else if fence == trimmed[0] && width >= fenceWidth && strings.TrimSpace(trimmed[width:]) == "" {
					fence, fenceWidth = 0, 0
				}
				out = append(out, line)
				continue
			}
		}
		if fence != 0 {
			out = append(out, line)
			continue
		}
		match := promptImage.FindStringSubmatchIndex(line)
		split := false
		// Current Codex emits the opening/closing image markers as separate text
		// blocks around non-text image data. Only fold an otherwise empty envelope.
		if match == nil && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "</image>" {
			candidate := line + "</image>"
			if found := promptImage.FindStringSubmatchIndex(candidate); found != nil && found[1] == len(candidate) {
				line, match, split = candidate, found, true
			}
		}
		if match == nil {
			out = append(out, line)
			continue
		}
		path := line[match[4]:match[5]]
		if len(path) > 4096 || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\x00\r\t") {
			out = append(out, lines[i])
			continue
		}
		u := url.URL{Path: path}
		label := strings.NewReplacer("[", "", "]", "", "\\", "").Replace(line[match[2]:match[3]])
		out = append(out, "!["+label+"]("+u.EscapedPath()+")\n"+line[match[1]:])
		if split {
			i++
		}
	}
	return strings.Join(out, "\n")
}
