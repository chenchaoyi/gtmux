package hq

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
// (A local copy of the same test helper app uses — test helpers are duplicated
// across the package boundary rather than shared.)
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureBoth runs fn with BOTH streams redirected and returns (stdout, stderr). The B9
// warning's whole contract is that it lands on stderr while stdout stays byte-identical,
// so a test that watched only one of them could not see the difference.
func captureBoth(t *testing.T, fn func()) (string, string) {
	t.Helper()
	ro, wo, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	re, we, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = wo, we
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()
	fn()
	wo.Close()
	we.Close()
	var out, errb bytes.Buffer
	io.Copy(&out, ro)
	io.Copy(&errb, re)
	return out.String(), errb.String()
}
