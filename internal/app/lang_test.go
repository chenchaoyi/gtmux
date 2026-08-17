package app

import "testing"

// The default language honors, in order: GTMUX_LANG, the machine-level config
// ("auto" deliberately falls through to the locale), the system locale, then
// English (machine-level-language). One resolution for every gtmux process —
// a launchd serve and the user's shell reach the same answer through config.
func TestResolveLang(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	for _, c := range []struct {
		name string
		m    map[string]string
		cfg  string
		want string
	}{
		{"explicit zh wins over an en locale", map[string]string{"GTMUX_LANG": "zh", "LANG": "en_US.UTF-8"}, "", "zh"},
		{"explicit en wins over a zh config", map[string]string{"GTMUX_LANG": "en"}, "zh", "en"},
		{"broken explicit value is english, not config", map[string]string{"GTMUX_LANG": "fr"}, "zh", "en"},
		{"config zh wins over an en locale", map[string]string{"LANG": "en_US.UTF-8"}, "zh", "zh"},
		{"config en wins over a zh locale", map[string]string{"LANG": "zh_CN.UTF-8"}, "en", "en"},
		{"broken config value is english, not locale", map[string]string{"LANG": "zh_CN.UTF-8"}, "fr", "en"},
		{"config auto follows the locale", map[string]string{"LANG": "zh_CN.UTF-8"}, "auto", "zh"},
		{"unset falls back to LANG", map[string]string{"LANG": "zh_CN.UTF-8"}, "", "zh"},
		{"LC_ALL outranks LANG", map[string]string{"LC_ALL": "zh_TW.UTF-8", "LANG": "en_US.UTF-8"}, "", "zh"},
		{"an en locale stays english", map[string]string{"LANG": "en_US.UTF-8"}, "", "en"},
		{"nothing set is english", map[string]string{}, "", "en"},
	} {
		if got := resolveLang(env(c.m), c.cfg); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
