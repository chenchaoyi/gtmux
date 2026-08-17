package app

import "testing"

// The default language honors the system locale when GTMUX_LANG is unset
// (empty-gap-and-locale): a zh* locale reads Chinese with no gtmux-specific
// setup, and an explicit GTMUX_LANG always wins — even a broken one, which
// resolves to English rather than silently falling through to the locale.
func TestLangFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	for _, c := range []struct {
		name string
		m    map[string]string
		want string
	}{
		{"explicit zh wins over an en locale", map[string]string{"GTMUX_LANG": "zh", "LANG": "en_US.UTF-8"}, "zh"},
		{"explicit en wins over a zh locale", map[string]string{"GTMUX_LANG": "en", "LANG": "zh_CN.UTF-8"}, "en"},
		{"broken explicit value is english, not locale", map[string]string{"GTMUX_LANG": "fr", "LANG": "zh_CN.UTF-8"}, "en"},
		{"unset falls back to LANG", map[string]string{"LANG": "zh_CN.UTF-8"}, "zh"},
		{"LC_ALL outranks LANG", map[string]string{"LC_ALL": "zh_TW.UTF-8", "LANG": "en_US.UTF-8"}, "zh"},
		{"an en locale stays english", map[string]string{"LANG": "en_US.UTF-8"}, "en"},
		{"nothing set is english", map[string]string{}, "en"},
	} {
		if got := langFromEnv(env(c.m)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
