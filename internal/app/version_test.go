package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Version has a non-empty default (overridden at release build via -ldflags).
func TestVersionDefaultNonEmpty(t *testing.T) {
	if strings.TrimSpace(Version) == "" {
		t.Fatalf("Version must have a non-empty default")
	}
}

// tagline is bilingual: distinct, non-empty strings for en and zh.
func TestTaglineBilingual(t *testing.T) {
	defer i18n.SetLang("en")

	i18n.SetLang("en")
	en := tagline()
	i18n.SetLang("zh")
	zh := tagline()

	if en == "" || zh == "" {
		t.Fatalf("tagline must be non-empty in both langs (en=%q zh=%q)", en, zh)
	}
	if en == zh {
		t.Errorf("tagline en and zh must differ (got %q for both)", en)
	}
}
