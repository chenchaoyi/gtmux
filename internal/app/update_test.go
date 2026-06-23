package app

import (
	"strings"
	"testing"
)

// installScriptMirrors must lead with the raw GitHub URL, then list the CN
// mirrors (jsdelivr + gh-proxy family). Every entry must be https and reference
// the repo path so each one fetches the same install.sh.
func TestInstallScriptMirrorsShape(t *testing.T) {
	if len(installScriptMirrors) < 3 {
		t.Fatalf("expected GitHub + several mirrors, got %d", len(installScriptMirrors))
	}

	first := installScriptMirrors[0]
	if !strings.HasPrefix(first, "https://raw.githubusercontent.com/") {
		t.Errorf("first mirror must be the raw GitHub URL, got %q", first)
	}

	rest := strings.Join(installScriptMirrors, "\n")
	if !strings.Contains(rest, "cdn.jsdelivr.net") {
		t.Errorf("mirrors must include the jsdelivr CDN")
	}
	if !strings.Contains(rest, "gh-proxy.com") {
		t.Errorf("mirrors must include the gh-proxy mirror")
	}

	for _, u := range installScriptMirrors {
		if !strings.HasPrefix(u, "https://") {
			t.Errorf("mirror not https: %q", u)
		}
		if !strings.Contains(u, "chenchaoyi/gtmux") {
			t.Errorf("mirror missing repo path chenchaoyi/gtmux: %q", u)
		}
		if !strings.HasSuffix(u, "install.sh") {
			t.Errorf("mirror must fetch install.sh: %q", u)
		}
	}
}

// fetchLatestTag is intentionally NOT unit-tested: it does a live HTTP GET to
// api.github.com and inlines its tag_name parse (no extractable pure helper).
// Exercising it would require network, which is out of scope for these tests.
