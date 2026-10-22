package app

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/releasefeed"
	"github.com/chenchaoyi/gtmux/internal/userservice"
	"howett.net/plist"
)

type releaseRoundTrip func(*http.Request) (*http.Response, error)

func (f releaseRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func releaseTar(t *testing.T, entries ...tar.Header) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			_, _ = tw.Write(bytes.Repeat([]byte("x"), int(h.Size)))
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func releaseZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, data := range files {
		f, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(f, data)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestNativeReleaseInstallTrustBoundary(t *testing.T) {
	archive := releaseTar(t, tar.Header{Name: "gtmux", Mode: 0o755, Size: 5, Typeflag: tar.TypeReg})
	const asset = "gtmux-1.2.3-darwin-arm64.tar.gz"
	for _, tc := range []struct {
		name                                                    string
		badChecksum, tampered, unavailable, app, appUnavailable bool
		wantErr                                                 bool
		appTimeout, appReplaceFailed, appActivateFailed         bool
	}{
		{name: "verified mirror archive"},
		{name: "tampered mirror archive", tampered: true, wantErr: true},
		{name: "malicious script as checksum", badChecksum: true, wantErr: true},
		{name: "no authoritative checksum", unavailable: true, wantErr: true},
		{name: "official app", app: true},
		{name: "app timeout", app: true, appTimeout: true},
		{name: "app replace failure", app: true, appReplaceFailed: true},
		{name: "app activation failure", app: true, appActivateFailed: true},
		{name: "app unavailable cannot fall back to mirror", app: true, appUnavailable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.app && runtime.GOOS != "darwin" {
				t.Skip("ditto is macOS-only")
			}
			dir := t.TempDir()
			binDir := filepath.Join(dir, "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(binDir, "gtmux")
			if err := os.WriteFile(binary, []byte("old"), 0o755); err != nil {
				t.Fatal(err)
			}
			var requested []string
			appActivated := false
			x := releaseInstaller{repo: "chenchaoyi/gtmux", version: "v1.2.3", platform: "darwin", arch: "arm64", binDir: binDir,
				appDir: filepath.Join(dir, "Applications"), mirror: "ghproxy", cliOnly: !tc.app,
				activateApp: func(target string) error {
					appActivated = true
					if tc.appActivateFailed {
						return fmt.Errorf("injected activation failure")
					}
					return nil
				},
				client: &http.Client{Transport: releaseRoundTrip(func(r *http.Request) (*http.Response, error) {
					requested = append(requested, r.URL.String())
					status, data := 200, []byte(nil)
					switch {
					case strings.HasSuffix(r.URL.Path, "SHASUMS256.txt"):
						if r.URL.Host != "github.com" {
							t.Fatal("checksum requested from a proxy")
						}
						data = []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), asset))
						if tc.badChecksum {
							data = []byte("#!/bin/bash\n# gtmux\ntouch /tmp/should-never-execute\n")
						}
						if tc.unavailable {
							status = 503
						}
					case strings.HasSuffix(r.URL.Path, ".tar.gz"):
						data = archive
						if tc.tampered {
							data = []byte("#!/bin/bash\n# gtmux\necho malicious")
						}
					case strings.HasSuffix(r.URL.Path, ".zip"):
						if data, err := os.ReadFile(binary); err != nil || string(data) != "xxxxx" {
							t.Fatalf("app attempted before verified CLI replacement: %q %v", data, err)
						}
						if r.URL.Host != "github.com" {
							t.Fatal("app requested from a proxy")
						}
						if tc.appTimeout {
							return nil, context.DeadlineExceeded
						}
						if tc.appUnavailable {
							status = 503
						} else {
							data = releaseZip(t, map[string]string{"Gtmux.app/Contents/Info.plist": "test bundle", "Gtmux.app/Contents/MacOS/GtmuxBar": "app"})
						}
					default:
						t.Fatalf("unexpected remote request (scripts must never be fetched): %s", r.URL)
					}
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
				})},
			}
			if tc.appReplaceFailed {
				x.replaceApp = func(string, string) error { return fmt.Errorf("injected replacement failure") }
			}

			result, err := x.install()
			if (err != nil) != tc.wantErr {
				t.Fatalf("install=%v wantErr=%v", err, tc.wantErr)
			}
			data, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			want := "xxxxx"
			if tc.wantErr && !tc.appUnavailable {
				want = "old"
			}
			if (tc.appUnavailable || tc.appTimeout || tc.appReplaceFailed || tc.appActivateFailed) && (!result.CLIInstalled || result.AppInstalled != tc.appActivateFailed || result.AppError == nil) {
				t.Fatalf("lost partial success: %+v", result)
			}
			if string(data) != want {
				t.Fatalf("installed bytes %q want %q", data, want)
			}
			if !tc.wantErr {
				fi, _ := os.Stat(binary)
				if fi.Mode().Perm() != 0o755 {
					t.Fatalf("binary mode=%v", fi.Mode())
				}
			}
			if appActivated != (tc.app && !tc.wantErr && !tc.appUnavailable && !tc.appTimeout && !tc.appReplaceFailed) {
				t.Fatalf("app activated=%v", appActivated)
			}
			if (tc.badChecksum || tc.unavailable) && len(requested) != 1 {
				t.Fatal("unverified downloads continued after invalid checksums")
			}
		})
	}
}

func TestReleaseTrustAnchorRejectsUntrustedRedirects(t *testing.T) {
	for _, location := range []string{"https://evil.example/SHASUMS256.txt", "http://github.com/SHASUMS256.txt", "https://github.com.evil.example/app.zip"} {
		calls := 0
		x := releaseInstaller{client: &http.Client{Transport: releaseRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{location}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})}}
		if err := x.download("https://github.com/release", filepath.Join(t.TempDir(), "asset"), 1000, true); err == nil {
			t.Fatalf("trusted redirect accepted %s", location)
		}
		if calls != 1 {
			t.Fatal("followed an untrusted redirect")
		}
	}
}

func TestReleaseArchiveCannotWritePathsOrLinks(t *testing.T) {
	for _, entries := range [][]tar.Header{
		{{Name: "../gtmux", Mode: 0o755, Size: 1, Typeflag: tar.TypeReg}},
		{{Name: "gtmux", Mode: 0o755, Typeflag: tar.TypeSymlink, Linkname: "../victim"}},
		{{Name: "gtmux", Mode: 0o755, Size: 1, Typeflag: tar.TypeReg}, {Name: "./gtmux", Mode: 0o755, Size: 1, Typeflag: tar.TypeReg}},
	} {
		dir := t.TempDir()
		archive := filepath.Join(dir, "cli.tar.gz")
		if err := os.WriteFile(archive, releaseTar(t, entries...), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := extractReleaseCLI(archive, filepath.Join(dir, "out")); err == nil {
			t.Fatalf("unsafe archive accepted: %+v", entries)
		}
		if _, err := os.Stat(filepath.Join(dir, "victim")); !os.IsNotExist(err) {
			t.Fatal("archive wrote an unrelated file")
		}
	}
}

func TestReleaseChecksumRejectsAmbiguousEntries(t *testing.T) {
	valid := strings.Repeat("a", 64) + "  gtmux.tar.gz\n"
	for _, body := range []string{"gtmux", valid + valid, strings.Repeat("a", 63) + "  gtmux.tar.gz", strings.ReplaceAll(valid, "gtmux.tar.gz", "other.tar.gz")} {
		if _, err := releaseChecksum([]byte(body), "gtmux.tar.gz"); err == nil {
			t.Fatalf("bad checksum accepted: %q", body)
		}
	}
	if _, err := releaseChecksum([]byte(valid), "gtmux.tar.gz"); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseAppRollbackPreservesOldBundle(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Gtmux.app")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old"), []byte("old app"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceReleaseApp(filepath.Join(dir, "missing"), target); err == nil {
		t.Fatal("missing staged app accepted")
	}
	if data, err := os.ReadFile(filepath.Join(target, "old")); err != nil || string(data) != "old app" {
		t.Fatalf("old app lost after failed swap: %q %v", data, err)
	}
}

func TestNativeReleaseInstallerHonorsPinsAndNoApp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_VERSION", "v1.2.3")
	t.Setenv("GTMUX_BIN_DIR", t.TempDir())
	t.Setenv("GTMUX_REPO", "owner/fork")
	t.Setenv("GTMUX_NO_APP", "1")
	x, err := newReleaseInstaller(false, "v9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if x.version != "v1.2.3" || !x.cliOnly || x.repo != "owner/fork" || x.binDir != os.Getenv("GTMUX_BIN_DIR") {
		t.Fatalf("pins lost: %+v", x)
	}
	for _, pin := range []string{"../../evil", "v1.2.3?x=y", "v1.2.3\nscript"} {
		t.Setenv("GTMUX_VERSION", pin)
		if _, err := newReleaseInstaller(true, "v1.2.3"); err == nil {
			t.Fatalf("unsafe tag accepted %q", pin)
		}
	}
}

func TestAppLoginPlistHasNoKeepAlive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := serviceManager
	serviceManager = func() userservice.Manager {
		return userservice.Launchd{Home: os.Getenv("HOME"), Run: func(context.Context, string, ...string) ([]byte, error) { return nil, nil }}
	}
	t.Cleanup(func() { serviceManager = old })
	target := filepath.Join(os.Getenv("HOME"), "Applications & Apps", "Gtmux.app")
	if err := setAppAutostart(target); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(launchAgentPath())
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Label                string
		ProgramArguments     []string
		RunAtLoad, KeepAlive bool
	}
	if _, err := plist.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Label != menubarBundleID || !got.RunAtLoad || got.KeepAlive || len(got.ProgramArguments) != 1 || got.ProgramArguments[0] != filepath.Join(target, "Contents", "MacOS", "GtmuxBar") {
		t.Fatalf("login contract changed: %+v", got)
	}
}

// Exercise selection at the actual filesystem installer, rather than only its JSON.
func TestReleaseInstallerComponentSelection(t *testing.T) {
	for _, tc := range []struct {
		cli, app                 string
		pinned, wantCLI, wantApp bool
	}{
		{"1.0.144", "1.0.144", false, false, false},
		{"1.0.143", "1.0.143", false, false, false},
		{"1.0.144", "1.0.140", false, false, true},
		{"1.0.140", "1.0.144", false, true, false},
		{"dev", "1.0.140", false, false, true},
		{"1.0.140", "unknown", false, true, false},
		{"1.0.140", "", false, true, true},
		{"1.0.144", "1.0.144", true, true, true},
	} {
		t.Run(tc.cli+"/"+tc.app+fmt.Sprint(tc.pinned), func(t *testing.T) {
			dir := t.TempDir()
			binDir := filepath.Join(dir, "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(binDir, "gtmux")
			if err := os.WriteFile(binary, []byte("old"), 0o755); err != nil {
				t.Fatal(err)
			}
			archive := releaseTar(t, tar.Header{Name: "gtmux", Mode: 0o755, Size: 5, Typeflag: tar.TypeReg})
			appRequested := false
			x := releaseInstaller{version: "v1.0.143", repo: "owner/repo", platform: "darwin", arch: "arm64", binDir: binDir, appDir: filepath.Join(dir, "apps"), mirror: "github",
				client: &http.Client{Transport: releaseRoundTrip(func(r *http.Request) (*http.Response, error) {
					status, data := 200, archive
					switch {
					case strings.HasSuffix(r.URL.Path, "SHASUMS256.txt"):
						data = []byte(fmt.Sprintf("%x  gtmux-1.0.143-darwin-arm64.tar.gz\n", sha256.Sum256(archive)))
					case strings.HasSuffix(r.URL.Path, ".zip"):
						appRequested = true
						status = 404
						data = nil
					}
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
				})}}
			x.selectComponents(tc.cli, tc.app, tc.pinned)
			result, err := x.install()
			if err != nil || result.CLIInstalled != tc.wantCLI || appRequested != tc.wantApp || (result.AppError != nil) != tc.wantApp {
				t.Fatalf("result=%+v err=%v appRequested=%v", result, err, appRequested)
			}
			got, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			want := "old"
			if tc.wantCLI {
				want = "xxxxx"
			}
			if string(got) != want {
				t.Fatalf("CLI bytes=%q want=%q", got, want)
			}
		})
	}
}

func TestReportPartialInstallPreservesComponentFacts(t *testing.T) {
	for _, installed := range []bool{false, true} {
		var rc int
		out := captureStderr(t, func() {
			rc = reportReleaseInstall(releaseInstallResult{CLIInstalled: true, AppInstalled: installed, AppError: fmt.Errorf("injected failure")}, nil)
		})
		want := "menu-bar app not installed"
		if installed {
			want = "menu-bar app installed, activation failed"
		}
		if rc != partialUpdateExit || !strings.Contains(out, "CLI installed") || !strings.Contains(out, want) {
			t.Fatalf("partial rc=%d output=%s", rc, out)
		}
	}
}

func TestSignedMirrorArchivesRequireExactDigestAndSize(t *testing.T) {
	archive := releaseTar(t, tar.Header{Name: "gtmux", Mode: 0o755, Size: 5, Typeflag: tar.TypeReg})
	name := "gtmux-1.2.3-linux-arm64.tar.gz"
	for _, mode := range []string{"valid", "bad digest", "bad size", "GitHub fallback"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			binary := filepath.Join(dir, "gtmux")
			if err := os.WriteFile(binary, []byte("old"), 0o755); err != nil {
				t.Fatal(err)
			}
			expectedSize := int64(len(archive))
			if mode == "bad size" {
				expectedSize++
			}
			manifest := &releasefeed.Manifest{Version: "1.2.3", Assets: map[string]releasefeed.Artifact{name: {SHA256: fmt.Sprintf("%x", sha256.Sum256(archive)), Size: expectedSize}}}
			var requests []string
			x := releaseInstaller{feed: manifest, repo: "chenchaoyi/gtmux", version: "v1.2.3", platform: "linux", arch: "arm64", binDir: dir, cliOnly: true, mirror: "github"}
			x.client = &http.Client{Transport: releaseRoundTrip(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.Host+r.URL.Path)
				if !strings.HasSuffix(r.URL.Path, name) {
					t.Fatalf("unsigned checksum or script requested: %s", r.URL)
				}
				data := archive
				status := 200
				if mode == "bad digest" {
					data = append([]byte(nil), archive...)
					data[len(data)-1] ^= 1
				}
				if mode == "GitHub fallback" && r.URL.Host != "github.com" {
					status = 503
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
			})}
			result, err := x.install()
			bad := mode == "bad digest" || mode == "bad size"
			if (err != nil) != bad || result.CLIInstalled == bad {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			got, _ := os.ReadFile(binary)
			want := "xxxxx"
			if bad {
				want = "old"
			}
			if string(got) != want {
				t.Fatalf("replaced unverified bytes: %q", got)
			}
			if len(requests) == 0 || !strings.HasPrefix(requests[0], "downloads.ccy.dev/") {
				t.Fatalf("not mirror first: %v", requests)
			}
		})
	}
}
