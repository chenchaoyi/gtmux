// Package releasecheck holds no code, only tests that run the release pipeline's own
// shell (the signing step of .github/workflows/release.yml and the signing section of
// macapp/build.sh) against stubbed security/openssl/codesign/xcrun in a temp dir. No
// keychain, certificate, Apple service or secret is touched.
package releasecheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// signingStep returns the run: block of the workflow step with `id: signing`, dedented.
func signingStep(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "id: signing" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("no step with id: signing")
	}
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "run: |" {
			continue
		}
		key := len(lines[i]) - len(strings.TrimLeft(lines[i], " "))
		var body []string
		indent := -1
		for _, l := range lines[i+1:] {
			if strings.TrimSpace(l) == "" {
				body = append(body, "")
				continue
			}
			n := len(l) - len(strings.TrimLeft(l, " "))
			if n <= key {
				break
			}
			if indent < 0 {
				indent = n
			}
			body = append(body, l[indent:])
		}
		return strings.Join(body, "\n") + "\n"
	}
	t.Fatal("the signing step has no run: | block")
	return ""
}

// signingSection returns build.sh from its signing setup to the line before "Built".
func signingSection(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "macapp", "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, `ENT="Gtmux.entitlements"`)
	j := strings.Index(s, `echo "Built `)
	if i < 0 || j < i {
		t.Fatal("build.sh signing section not found")
	}
	return "set -euo pipefail\nBUNDLE=Gtmux.app\nAPP_BIN=GtmuxBar\n" + s[i:j]
}

// stub writes an executable that logs its call to calls.log and runs body.
func stub(t *testing.T, dir, name, body string) {
	t.Helper()
	script := "#!/bin/sh\necho \"" + name + " $*\" >>\"$STUB_LOG\"\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

type run struct {
	code              int
	out, env, outputs string
	calls             string
}

func runScript(t *testing.T, script string, env map[string]string) run {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	stub(t, bin, "security", `[ "$1" = find-identity ] && echo '  1) ABC "Developer ID Application: Test (TEAM123)"'; exit 0`)
	stub(t, bin, "openssl", `echo 0123456789abcdef`)
	stub(t, bin, "base64", `cat`)
	stub(t, bin, "codesign", `exit 0`)
	stub(t, bin, "ditto", `exit 0`)
	stub(t, bin, "xcrun", `exit 0`)
	path := filepath.Join(dir, "step.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	envFile, outFile, log := filepath.Join(dir, "env"), filepath.Join(dir, "out"), filepath.Join(dir, "calls.log")
	cmd := exec.Command("bash", path)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + bin + ":/usr/bin:/bin",
		"HOME=" + dir,
		"RUNNER_TEMP=" + dir,
		"GITHUB_ENV=" + envFile,
		"GITHUB_OUTPUT=" + outFile,
		"STUB_LOG=" + log,
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	r := run{out: string(out)}
	if ee, ok := err.(*exec.ExitError); ok {
		r.code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	r.env, r.outputs, r.calls = read(envFile), read(outFile), read(log)
	return r
}

func secrets(drop ...string) map[string]string {
	m := map[string]string{
		"MACOS_CERT_P12":      "cert-bytes",
		"MACOS_CERT_PASSWORD": "pw",
		"MACOS_NOTARY_KEY_P8": "p8-bytes",
		"MACOS_NOTARY_KEY_ID": "KEYID",
		"MACOS_NOTARY_ISSUER": "issuer-uuid",
	}
	for _, d := range drop {
		delete(m, d)
	}
	return m
}

func with(m map[string]string, k, v string) map[string]string { m[k] = v; return m }

// A tag needs all five secrets before any keychain work. With the cert and the .p8 but
// no key id or issuer, the step used to configure signing, export a partial notary set,
// and the build skipped notarization while the upload went ahead (%12, 2026-10-06).
func TestATagWithoutEveryNotarySecretStopsBeforeTheKeychain(t *testing.T) {
	step := signingStep(t)
	for _, missing := range []string{"MACOS_NOTARY_KEY_ID", "MACOS_NOTARY_ISSUER", "MACOS_CERT_PASSWORD", "MACOS_NOTARY_KEY_P8", "MACOS_CERT_P12"} {
		r := runScript(t, step, with(secrets(missing), "GITHUB_REF_TYPE", "tag"))
		if r.code == 0 || strings.Contains(r.outputs, "configured=true") {
			t.Errorf("tag without %s: exit %d outputs %q\n%s", missing, r.code, r.outputs, r.out)
		}
		if !strings.Contains(r.out, missing) {
			t.Errorf("tag without %s: the error does not name it:\n%s", missing, r.out)
		}
		if strings.Contains(r.calls, "security") {
			t.Errorf("tag without %s: touched the keychain first:\n%s", missing, r.calls)
		}
	}
}

func TestATagWithEverySecretIsSignedAndMustNotarize(t *testing.T) {
	r := runScript(t, signingStep(t), with(secrets(), "GITHUB_REF_TYPE", "tag"))
	if r.code != 0 || !strings.Contains(r.outputs, "configured=true") {
		t.Fatalf("exit %d outputs %q\n%s", r.code, r.outputs, r.out)
	}
	for _, want := range []string{"GTMUX_SIGN_ID=Developer ID Application: Test (TEAM123)", "GTMUX_NOTARY_KEY=", "GTMUX_NOTARY_KEY_ID=KEYID", "GTMUX_NOTARY_ISSUER=issuer-uuid", "GTMUX_REQUIRE_NOTARIZE=1"} {
		if !strings.Contains(r.env, want) {
			t.Errorf("GITHUB_ENV lacks %q:\n%s", want, r.env)
		}
	}
}

// Snapshot and dispatch builds keep their old freedom: ad-hoc without a cert, signed but
// not notarized with an incomplete notary set (said as a warning), never required.
func TestANonTagBuildMayBeAdHocOrSignedOnly(t *testing.T) {
	adhoc := runScript(t, signingStep(t), map[string]string{"GITHUB_REF_TYPE": "branch"})
	if adhoc.code != 0 || strings.Contains(adhoc.outputs, "configured=true") || strings.Contains(adhoc.env, "GTMUX_") {
		t.Errorf("no cert: exit %d outputs %q env %q\n%s", adhoc.code, adhoc.outputs, adhoc.env, adhoc.out)
	}
	partial := runScript(t, signingStep(t), with(secrets("MACOS_NOTARY_ISSUER"), "GITHUB_REF_TYPE", "branch"))
	if partial.code != 0 || !strings.Contains(partial.outputs, "configured=true") {
		t.Fatalf("partial: exit %d\n%s", partial.code, partial.out)
	}
	if strings.Contains(partial.env, "GTMUX_NOTARY_") || strings.Contains(partial.env, "GTMUX_REQUIRE_NOTARIZE") {
		t.Errorf("partial notary set exported: %s", partial.env)
	}
	if !strings.Contains(partial.out, "NOT notarized") {
		t.Errorf("no warning for the partial set:\n%s", partial.out)
	}
}

// build.sh: with GTMUX_REQUIRE_NOTARIZE=1 an incomplete notary set or a missing signing
// identity fails before anything is submitted; without it, a local build still prints
// "NOT notarized" and goes on, as before.
func TestBuildRefusesToSkipARequiredNotarization(t *testing.T) {
	sec := signingSection(t)
	full := map[string]string{"GTMUX_SIGN_ID": "Developer ID Application: Test", "GTMUX_NOTARY_KEY": "k.p8", "GTMUX_NOTARY_KEY_ID": "KEYID", "GTMUX_NOTARY_ISSUER": "iss"}
	if r := runScript(t, sec, with(copyMap(full), "GTMUX_REQUIRE_NOTARIZE", "1")); r.code != 0 || !strings.Contains(r.calls, "xcrun notarytool submit") || !strings.Contains(r.calls, "xcrun stapler validate") {
		t.Errorf("full set: exit %d calls:\n%s\n%s", r.code, r.calls, r.out)
	}
	noIssuer := copyMap(full)
	delete(noIssuer, "GTMUX_NOTARY_ISSUER")
	if r := runScript(t, sec, with(copyMap(noIssuer), "GTMUX_REQUIRE_NOTARIZE", "1")); r.code == 0 || strings.Contains(r.calls, "xcrun") {
		t.Errorf("required, no issuer: exit %d calls:\n%s\n%s", r.code, r.calls, r.out)
	}
	if r := runScript(t, sec, map[string]string{"GTMUX_REQUIRE_NOTARIZE": "1"}); r.code == 0 {
		t.Errorf("required, ad-hoc: exit 0\n%s", r.out)
	}
	if r := runScript(t, sec, noIssuer); r.code != 0 || !strings.Contains(r.out, "NOT notarized") || strings.Contains(r.calls, "xcrun") {
		t.Errorf("not required, no issuer: exit %d\n%s", r.code, r.out)
	}
	if r := runScript(t, sec, map[string]string{"GTMUX_SIGN_ID": "Developer ID Application: Test", "GTMUX_NOTARY_PROFILE": "gtmux", "GTMUX_REQUIRE_NOTARIZE": "1"}); r.code != 0 || !strings.Contains(r.calls, "--keychain-profile gtmux") {
		t.Errorf("keychain profile: exit %d calls:\n%s", r.code, r.calls)
	}
}

func copyMap(m map[string]string) map[string]string {
	c := map[string]string{}
	for k, v := range m {
		c[k] = v
	}
	return c
}
