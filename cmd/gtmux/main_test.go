package main

import (
	"os"
	"strings"
	"testing"
)

// Every gtmux process creates files under the two roots, and 155 call sites pass a
// literal 0644 or 0755. The private umask is what makes them 0600 / 0700, so it has to
// run before anything else in main.
func TestMainSetsThePrivateUmaskFirst(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	u, r := strings.Index(s, "state.PrivateUmask()"), strings.Index(s, "app.Run(")
	if u < 0 {
		t.Fatal("main does not set the private umask")
	}
	if r < 0 || u > r {
		t.Error("main runs the command before setting the private umask")
	}
}
