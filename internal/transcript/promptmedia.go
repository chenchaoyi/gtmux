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
	fence := byte(0)
	fenceWidth := 0
	for i, line := range lines {
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
				continue
			}
		}
		if fence != 0 {
			continue
		}
		match := promptImage.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		path := line[match[4]:match[5]]
		if len(path) > 4096 || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\x00\r\t") {
			continue
		}
		u := url.URL{Path: path}
		label := strings.NewReplacer("[", "", "]", "", "\\", "").Replace(line[match[2]:match[3]])
		lines[i] = "![" + label + "](" + u.EscapedPath() + ")\n" + line[match[1]:]
	}
	return strings.Join(lines, "\n")
}
