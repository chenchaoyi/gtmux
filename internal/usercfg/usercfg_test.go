package usercfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var c struct {
		X *int `json:"x"`
	}
	if err := Load(&c); err != nil {
		t.Fatalf("a missing config file must not be an error; got %v", err)
	}
	if c.X != nil {
		t.Fatalf("dst must keep its defaults when the file is absent")
	}
}

func TestLoad_RoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Dir(Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte(`{"x":7,"y":"hi"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var c struct {
		X *int    `json:"x"`
		Y *string `json:"y"`
	}
	if err := Load(&c); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.X == nil || *c.X != 7 || c.Y == nil || *c.Y != "hi" {
		t.Fatalf("round-trip failed: %+v", c)
	}
}

func TestLoad_MalformedReturnsError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Dir(Path()), 0o755)
	_ = os.WriteFile(Path(), []byte(`{not json`), 0o644)
	var c struct{ X *int }
	if err := Load(&c); err == nil {
		t.Fatalf("a malformed config must return an error (callers turn it into a default)")
	}
}
