package selftunnel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The nginx stub fails `nginx -t` while any enabled site says BROKEN, or always when the
// box's config is already broken elsewhere.
const nginxStub = `#!/bin/sh
[ -e "$NGINX_ETC/broken-elsewhere" ] && { echo "nginx: [emerg] another site is broken" >&2; exit 1; }
grep -qs BROKEN "$NGINX_ETC"/sites-enabled/* && { echo "nginx: [emerg] BROKEN directive" >&2; exit 1; }
exit 0
`

// nginxBox is /etc/nginx in a temp dir, with a stub nginx and a reload that leaves a mark.
type nginxBox struct {
	t                *testing.T
	etc, bin, reload string
}

func newNginxBox(t *testing.T) *nginxBox {
	t.Helper()
	b := &nginxBox{t: t, etc: t.TempDir(), bin: t.TempDir()}
	b.reload = filepath.Join(b.bin, "reloaded")
	for _, d := range []string{"sites-available", "sites-enabled"} {
		if err := os.MkdirAll(filepath.Join(b.etc, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(b.bin, "nginx"), []byte(nginxStub), 0o755); err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *nginxBox) site() string { return filepath.Join(b.etc, "sites-available", "gtmux-direct.conf") }
func (b *nginxBox) link() string { return filepath.Join(b.etc, "sites-enabled", "gtmux-direct.conf") }

// serving puts a working gtmux site in place, as a first run plus certbot leaves it.
func (b *nginxBox) serving() {
	b.t.Helper()
	if err := os.WriteFile(b.site(), []byte("server { certbot-edited OLD-WORKING }\n"), 0o644); err != nil {
		b.t.Fatal(err)
	}
	if err := os.Symlink(b.site(), b.link()); err != nil {
		b.t.Fatal(err)
	}
}

// install runs the real helper with a template whose content is given.
func (b *nginxBox) install(template string) (int, string) {
	b.t.Helper()
	src := filepath.Join(b.bin, "template.conf")
	if err := os.WriteFile(src, []byte(template), 0o644); err != nil {
		b.t.Fatal(err)
	}
	_ = os.Remove(b.reload)
	cmd := exec.Command("bash", "nginx-site-install.sh", src, "direct.example")
	cmd.Env = append(os.Environ(),
		"PATH="+b.bin+":"+os.Getenv("PATH"),
		"NGINX_ETC="+b.etc,
		"NGINX_RELOAD=touch "+b.reload,
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		b.t.Fatal(err)
	}
	return code, string(out)
}

func (b *nginxBox) siteContent() string {
	s, err := os.ReadFile(b.site())
	if err != nil {
		return "<none>"
	}
	return strings.TrimSpace(string(s))
}

func (b *nginxBox) linkTarget() string {
	if t, err := os.Readlink(b.link()); err == nil {
		return t
	}
	if _, err := os.Stat(b.link()); err == nil {
		return "<a file>"
	}
	return "<none>"
}

func (b *nginxBox) reloaded() bool { _, err := os.Stat(b.reload); return err == nil }

// A re-run whose new site fails the check puts the site that WAS serving back, link and
// all. It used to leave the failing file in its place and remove the link, taking the
// gtmux site down while saying "nginx left as it was" (%12, 2026-10-06).
func TestAFailedRerunPutsTheServingSiteBack(t *testing.T) {
	b := newNginxBox(t)
	b.serving()
	code, out := b.install("server { BROKEN __DOMAIN__ }\n")
	if code == 0 {
		t.Fatalf("a failed check exited 0:\n%s", out)
	}
	if got := b.siteContent(); got != "server { certbot-edited OLD-WORKING }" {
		t.Errorf("site not restored: %q", got)
	}
	if got := b.linkTarget(); got != b.site() {
		t.Errorf("link not restored: %q", got)
	}
	if !strings.Contains(out, "BROKEN directive") || !strings.Contains(out, "previous gtmux site (or none) is back") {
		t.Errorf("output does not show the check and the restore:\n%s", out)
	}
}

// A fresh install whose site fails the check leaves no gtmux site at all.
func TestAFailedFreshInstallLeavesNoSite(t *testing.T) {
	b := newNginxBox(t)
	if code, out := b.install("server { BROKEN __DOMAIN__ }\n"); code == 0 {
		t.Fatalf("exited 0:\n%s", out)
	}
	if b.siteContent() != "<none>" || b.linkTarget() != "<none>" {
		t.Errorf("left behind: site %q link %q", b.siteContent(), b.linkTarget())
	}
}

// When the config is broken elsewhere, nothing of ours is left in place and nginx is not
// reloaded onto a config that does not check, and the message says so.
func TestABoxBrokenElsewhereIsNotReloaded(t *testing.T) {
	b := newNginxBox(t)
	b.serving()
	if err := os.WriteFile(filepath.Join(b.etc, "broken-elsewhere"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := b.install("server { listen 443; server_name __DOMAIN__; }\n")
	if code == 0 || b.reloaded() {
		t.Fatalf("exit %d reloaded %v:\n%s", code, b.reloaded(), out)
	}
	if b.siteContent() != "server { certbot-edited OLD-WORKING }" || b.linkTarget() != b.site() {
		t.Errorf("not restored: site %q link %q", b.siteContent(), b.linkTarget())
	}
	if !strings.Contains(out, "NOT reloaded") {
		t.Errorf("does not say nginx was not reloaded:\n%s", out)
	}
}

// An enabled site that is a plain file, not a link, comes back as that file.
func TestAnEnabledFileComesBackAsAFile(t *testing.T) {
	b := newNginxBox(t)
	if err := os.WriteFile(b.link(), []byte("server { hand-made }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := b.install("server { BROKEN __DOMAIN__ }\n"); code == 0 {
		t.Fatalf("exited 0:\n%s", out)
	}
	if got, _ := os.ReadFile(b.link()); b.linkTarget() != "<a file>" || strings.TrimSpace(string(got)) != "server { hand-made }" {
		t.Errorf("enabled file not restored: %q %q", b.linkTarget(), got)
	}
	if b.siteContent() != "<none>" {
		t.Errorf("a failed site file was left in sites-available: %q", b.siteContent())
	}
}

// The ordinary paths are unchanged: a fresh install and a re-run that check out are put
// in place, linked, and nginx is reloaded (never restarted).
func TestAGoodSiteIsInstalledAndReloaded(t *testing.T) {
	for _, rerun := range []bool{false, true} {
		b := newNginxBox(t)
		if rerun {
			b.serving()
		}
		code, out := b.install("server { listen 443; server_name __DOMAIN__; }\n")
		if code != 0 || !b.reloaded() {
			t.Fatalf("rerun=%v: exit %d reloaded %v\n%s", rerun, code, b.reloaded(), out)
		}
		if b.siteContent() != "server { listen 443; server_name direct.example; }" || b.linkTarget() != b.site() {
			t.Errorf("rerun=%v: site %q link %q", rerun, b.siteContent(), b.linkTarget())
		}
	}
}
