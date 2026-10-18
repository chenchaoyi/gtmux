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
	}{
		{name: "verified mirror archive"},
		{name: "tampered mirror archive", tampered: true, wantErr: true},
		{name: "malicious script as checksum", badChecksum: true, wantErr: true},
		{name: "no authoritative checksum", unavailable: true, wantErr: true},
		{name: "official app", app: true},
		{name: "app unavailable cannot fall back to mirror", app: true, appUnavailable: true, wantErr: true},
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
				activateApp: func(target string) error { appActivated = true; return nil },
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
						if r.URL.Host != "github.com" {
							t.Fatal("app requested from a proxy")
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
			err := x.install()
			if (err != nil) != tc.wantErr {
				t.Fatalf("install=%v wantErr=%v", err, tc.wantErr)
			}
			data, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			want := "xxxxx"
			if tc.wantErr {
				want = "old"
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
			if appActivated != (tc.app && !tc.wantErr) {
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
