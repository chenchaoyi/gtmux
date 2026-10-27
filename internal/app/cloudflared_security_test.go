package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCloudflaredSecurityFloor(t *testing.T) {
	for _, tt := range []struct {
		output, platform string
		brew, safe       bool
	}{
		{"cloudflared version 2026.10.2 (built today)", "linux", false, true},
		{"cloudflared version 2026.10.2", "darwin", true, true},
		{"cloudflared version 2026.10.2", "darwin", false, false},
		{"cloudflared version 2026.10.3", "darwin", false, true},
		{"cloudflared version 2027.1.0", "linux", false, true},
		{"cloudflared version 2026.9.999", "linux", true, false},
		{"cloudflared version 2026.10.1", "linux", false, false},
		{"cloudflared version 2026.10.3-rc1", "darwin", false, false},
		{"cloudflared version 2026.10.3evil", "linux", false, false},
		{"cloudflared version 2026.99.0", "linux", false, false},
		{"", "linux", false, false},
		{"unrelated 2026.10.3", "linux", false, false},
	} {
		_, err := cloudflaredVersionSafe(tt.output, tt.platform, tt.brew)
		if (err == nil) != tt.safe {
			t.Errorf("%q / %s / brew=%v: safe=%v, error=%v", tt.output, tt.platform, tt.brew, tt.safe, err)
		}
	}
}

func TestCloudflaredMetricsTransportRefusesRedirectsAndOversizedBodies(t *testing.T) {
	for _, mode := range []string{"safe", "redirect", "large", "failed"} {
		t.Run(mode, func(t *testing.T) {
			targetCalls := 0
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetCalls++
				_, _ = w.Write([]byte(`build_info{version="2026.10.3"} 1`))
			}))
			defer target.Close()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "redirect":
					http.Redirect(w, r, target.URL, http.StatusFound)
				case "large":
					_, _ = w.Write([]byte(strings.Repeat("x", 262145)))
				case "failed":
					w.WriteHeader(http.StatusServiceUnavailable)
				default:
					_, _ = w.Write([]byte(`build_info{version="2026.10.3"} 1`))
				}
			}))
			defer srv.Close()
			got := readCloudflaredMetrics(srv.URL)
			if (got == "2026.10.3") != (mode == "safe") || targetCalls != 0 {
				t.Fatalf("mode=%s version=%q redirected=%d", mode, got, targetCalls)
			}
		})
	}
}

func TestCloudflaredRunnerRejectsOldClientBeforeLaunching(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix client fixture")
	}
	dir := t.TempDir()
	marker, bin := filepath.Join(dir, "launched"), filepath.Join(dir, "cloudflared")
	for _, version := range []string{"2026.6.1", "2026.10.3"} {
		body := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'cloudflared version " + version + "'; exit 0; fi\ntouch '" + marker + "'\necho READY >&2\n"
		if err := os.WriteFile(bin, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
		ready := false
		rc := runCloudflared(bin, []string{"tunnel"}, regexp.MustCompile("READY"), func(string) { ready = true })
		_, err := os.Stat(marker)
		if version == "2026.6.1" {
			if rc == 0 || ready || !os.IsNotExist(err) {
				t.Fatalf("old client launched: rc=%d, ready=%v, marker=%v", rc, ready, err)
			}
		} else if rc != 0 || !ready || err != nil {
			t.Fatalf("fixed client rejected: rc=%d, ready=%v, marker=%v", rc, ready, err)
		}
	}
}

func TestCloudflaredProbeIsBoundedAndMatchesItsHomebrew(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix client fixture")
	}
	prefix := t.TempDir()
	bin := filepath.Join(prefix, "Cellar", "cloudflared", "2026.10.3", "bin", "cloudflared")
	brew := filepath.Join(prefix, "bin", "brew")
	for _, f := range []string{bin, brew} {
		if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("#!/bin/sh\necho 'cloudflared version 2026.10.3'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	canonicalBrew, err := filepath.EvalSymlinks(brew)
	if err != nil {
		t.Fatal(err)
	}
	if got := cloudflaredBrew(bin); got != canonicalBrew {
		t.Fatalf("owner brew=%q, want %q", got, canonicalBrew)
	}
	if _, err := checkCloudflared(bin); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexec sleep 20\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := checkCloudflared(bin); err == nil || time.Since(start) > 5*time.Second {
		t.Fatal("version probe failed to stop on deadline")
	}
}

func TestUpdateDependencyPolicy(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		check, cliOnly, newer bool
		installRC, want       int
	}{
		{"read-only check", true, false, true, 0, 0},
		{"CLI-only", false, true, true, 0, 0},
		{"existing release", false, false, false, 0, 1},
		{"new release", false, false, true, 0, 1},
		{"failed release", false, false, true, 1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("GTMUX_VERSION", "")
			t.Setenv("GTMUX_NO_APP", "")
			calls := 0
			latest := "v1.0.1"
			if tt.newer {
				latest = "v1.0.2"
			}
			ops := updateOperations{current: "1.0.1", appVersion: func() string { return "1.0.1" }, latest: func() latestRelease { return latestRelease{Tag: latest} },
				install: func(bool, string) (releaseInstallResult, int) { return releaseInstallResult{}, tt.installRC },
				changes: func(string, string) {}, doctor: func() {}, dependencies: func() int { calls++; return 0 }}
			performUpdate(tt.check, tt.cliOnly, false, ops)
			if calls != tt.want {
				t.Fatalf("dependency changes=%d, want %d", calls, tt.want)
			}
		})
	}
}

func TestCloudflaredRunningVersionRequiresUnambiguousBuildInfo(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{`# HELP build_info Build information
build_info{goversion="go1.27.3",revision="abc",type="",version="2026.10.3"} 1
cloudflared_tunnel_ha_connections 4`, "2026.10.3"},
		{`build_info{version="2026.6.1"} 1`, "2026.6.1"},
		{`other_build_info{version="2026.10.3"} 1`, ""},
		{`build_info{version="2026.10.3-rc1"} 1`, ""},
		{`build_info{version="2026.10.3",version="2026.6.1"} 1`, ""},
		{"build_info{version=\"2026.10.3\"} 1\nbuild_info{version=\"2026.6.1\"} 1", ""},
		{`build_info{revision="escaped\"value",version="2026.10.3"} 1`, ""},
		{`build_info{version="2026.10.3"} 0`, ""},
		{`build_info{goversion="go1.27.3"} 1`, ""},
	} {
		if got := cloudflaredVersionFromMetrics(tt.body); got != tt.want {
			t.Errorf("metrics version=%q, want %q for %q", got, tt.want, tt.body)
		}
	}
}
