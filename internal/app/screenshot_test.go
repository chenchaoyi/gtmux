package app

import (
	"context"
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/screenshot"
	"io"
	"os"
	"testing"
)

func screenshotCLIResult(t *testing.T, args ...string) (int, map[string]any) {
	t.Helper()
	old := os.Stdout
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	code := cmdScreenshot(args)
	w.Close()
	b, _ := io.ReadAll(r)
	r.Close()
	var v map[string]any
	if json.Unmarshal(b, &v) != nil {
		t.Fatalf("not JSON: %s", b)
	}
	return code, v
}
func TestASC001ScreenshotCLIRefusesAmbiguousOrUnsupportedInput(t *testing.T) {
	old := screenshotNative
	t.Cleanup(func() { screenshotNative = old })
	calls := 0
	screenshotNative = func(context.Context, screenshot.Request, []*os.File) (screenshot.Reply, error) {
		calls++
		return screenshot.Reply{}, screenshot.Fail("app-unavailable", "fixture")
	}
	for _, args := range [][]string{{"unknown"}, {"capture", "--source-ref", "x", "--rect", "NaN,0,1,1"}, {"sources"}, {"sources", "--app", "example.app", "--displays"}, {"capabilities", "--timeout", "11s"}, {"capabilities", "extra"}, {"status"}, {"capture", "--unit", "px"}} {
		code, v := screenshotCLIResult(t, args...)
		if code == 0 || v["ok"] != false {
			t.Fatal(args, v)
		}
	}
	if calls != 0 {
		t.Fatal("invalid command called native host")
	}
}
func TestASC006UnavailableNativeCapabilityIsTruthfulAndNonMutating(t *testing.T) {
	old := screenshotNative
	t.Cleanup(func() { screenshotNative = old })
	screenshotNative = func(context.Context, screenshot.Request, []*os.File) (screenshot.Reply, error) {
		return screenshot.Reply{}, screenshot.Fail("app-unavailable", "fixture")
	}
	code, v := screenshotCLIResult(t, "capabilities")
	if code != 0 || v["ok"] != true || v["result"].(map[string]any)["capture_available"] != false {
		t.Fatal(v)
	}
	code, v = screenshotCLIResult(t, "sources", "--app", "example.app")
	if code == 0 || v["error"].(map[string]any)["code"] != "app-unavailable" {
		t.Fatal(v)
	}
}
