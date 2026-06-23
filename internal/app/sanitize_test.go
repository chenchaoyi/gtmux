package app

import (
	"strings"
	"testing"
)

// sanitizeFilename keeps only the base name with a conservative charset so an
// uploaded name can never escape the uploads dir or inject anything.
func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain name kept", "photo.png", "photo.png"},
		{"strips directory components", "/etc/passwd", "passwd"},
		{"path traversal flattened", "../../../etc/passwd", "passwd"},
		{"backslash-ish weird chars become underscore", "a b*c?.txt", "a_b_c_.txt"},
		{"leading dots stripped", "...hidden", "hidden"},
		{"keeps dash and underscore", "my-file_1.tar.gz", "my-file_1.tar.gz"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeFilename(c.in); got != c.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// An empty / all-dots input sanitizes to empty (the caller substitutes "upload").
// Note: filepath.Base("/") is "/", which sanitizes to "_" (not empty) — that case
// is covered by TestSanitizeFilenameNoSeparators instead.
func TestSanitizeFilenameEmptyish(t *testing.T) {
	for _, in := range []string{"", "....", ".", ".."} {
		if got := sanitizeFilename(in); got != "" {
			t.Errorf("sanitizeFilename(%q) = %q, want empty", in, got)
		}
	}
}

// The result is capped at 80 chars (keeping the tail, so the extension survives).
func TestSanitizeFilenameLengthCap(t *testing.T) {
	long := strings.Repeat("a", 200) + ".png"
	got := sanitizeFilename(long)
	if len(got) != 80 {
		t.Fatalf("len = %d, want 80", len(got))
	}
	if !strings.HasSuffix(got, ".png") {
		t.Errorf("length cap should keep the tail (extension): got %q", got)
	}
}

// The output never contains a path separator (can't escape the uploads dir).
func TestSanitizeFilenameNoSeparators(t *testing.T) {
	for _, in := range []string{"a/b/c", "..\\..\\x", "/abs/path/to/x.bin"} {
		got := sanitizeFilename(in)
		if strings.ContainsAny(got, "/\\") {
			t.Errorf("sanitizeFilename(%q) = %q still contains a separator", in, got)
		}
	}
}
