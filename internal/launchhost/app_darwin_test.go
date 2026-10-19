package launchhost

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMacBundleNamePreferenceAndFallback(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "Sample Workbench.app")
	contents := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(contents, 0700); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(contents, "Info.plist")
	command := filepath.Join(contents, "Frameworks", "Helper.app", "Contents", "MacOS", "Helper")
	for _, tc := range []struct{ body, want string }{{`<key>CFBundleName</key><string>Bundle Name</string><key>CFBundleDisplayName</key><string>Display Name</string>`, "Display Name"}, {`<key>CFBundleName</key><string>Bundle Name</string>`, "Bundle Name"}, {"", "Sample Workbench"}} {
		if err := os.WriteFile(plist, []byte(`<?xml version="1.0"?><plist version="1.0"><dict>`+tc.body+`</dict></plist>`), 0600); err != nil {
			t.Fatal(err)
		}
		if got := appName(context.Background(), command); got != tc.want {
			t.Fatalf("name=%q want %q", got, tc.want)
		}
	}
	if err := os.Truncate(plist, 256<<10+1); err != nil {
		t.Fatal(err)
	}
	if got := appName(context.Background(), command); got != "" {
		t.Fatal("oversized plist accepted", got)
	}
}
