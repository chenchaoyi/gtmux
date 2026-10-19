package launchhost

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

func appName(ctx context.Context, command string) string {
	if !filepath.IsAbs(command) {
		return ""
	}
	index := strings.Index(command, ".app/Contents/")
	if index < 0 {
		return ""
	}
	bundle := command[:index+4]
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	info, err := os.Stat(plist)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 256<<10 {
		return ""
	}
	for _, key := range []string{"CFBundleDisplayName", "CFBundleName"} {
		name := run(ctx, "/usr/bin/plutil", "-extract", key, "raw", "-o", "-", plist)
		if validName(name, "app") {
			return name
		}
	}
	name := strings.TrimSuffix(filepath.Base(bundle), ".app")
	if validName(name, "app") {
		return name
	}
	return ""
}
