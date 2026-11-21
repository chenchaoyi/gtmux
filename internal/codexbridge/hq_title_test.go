package codexbridge

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestHQRootTitleBeforeBinding(t *testing.T) {
	o, bound := readyObserver(t)
	var named []string
	o.seedRootName = func(id string) string {
		if len(*bound) != len(named) {
			t.Fatal("root was published before title confirmation")
		}
		named = append(named, id)
		return hqThreadTitle
	}
	for i, method := range []string{"thread/start", "thread/start", "thread/resume"} {
		id, thread := i+1, fmt.Sprintf("hq-root-%d", i+1)
		o.request([]byte(fmt.Sprintf(`{"id":%d,"method":%q}`, id, method)))
		data := []byte(fmt.Sprintf(`{"id":%d,"result":{"thread":{"id":%q,"name":null,"source":"cli","unknown":{"retain":42}},"model":"test"}}`, id, thread))
		got := o.rootResponse(data)
		for _, want := range []string{`"name":"` + hqThreadTitle + `"`, `"unknown":{"retain":42}`, `"model":"test"`} {
			if !bytes.Contains(got, []byte(want)) {
				t.Fatalf("%s response lost %s: %s", method, want, got)
			}
		}
	}
	if !slices.Equal(named, []string{"hq-root-1", "hq-root-2", "hq-root-3"}) || !slices.Equal(named, *bound) {
		t.Fatalf("naming = %v, bindings = %v", named, *bound)
	}
}

func TestHQRootTitlePreservesUnownedOrNamedFrames(t *testing.T) {
	o, bound := readyObserver(t)
	o.seedRootName = func(id string) string {
		t.Fatalf("unexpected title operation on %s", id)
		return ""
	}
	o.request([]byte(`{"id":1,"method":"thread/start"}`))
	o.request([]byte(`{"id":2,"method":"thread/resume"}`))
	for _, data := range [][]byte{
		rootResponse(1, "stale"),
		[]byte(`{"method":"thread/started","params":{"thread":{"id":"foreign"}}}`),
		[]byte(`{"id":2,"result":{"thread":{"id":"user-renamed","name":"My coordination notes","source":"cli"}}}`),
	} {
		if got := o.rootResponse(data); !bytes.Equal(got, data) {
			t.Fatalf("altered frame: %s", got)
		}
	}
	o.request([]byte(`{"id":3,"method":"thread/start","params":{"ephemeral":true}}`))
	data := rootResponse(3, "helper")
	if got := o.rootResponse(data); !bytes.Equal(got, data) || strings.Join(*bound, ",") != "user-renamed" {
		t.Fatalf("helper or stale response affected bindings %v or frame %s", *bound, got)
	}
	// A normal pane and a failed HQ naming call both pass through the original
	// root response and do not block that root's ordinary publication.
	for _, seed := range []func(string) string{nil, func(string) string { return "" }} {
		o.seedRootName = seed
		o.request([]byte(`{"id":4,"method":"thread/start"}`))
		data := rootResponse(4, "usable")
		if got := o.rootResponse(data); !bytes.Equal(got, data) || (*bound)[len(*bound)-1] != "usable" {
			t.Fatalf("lost original root response: %s, %v", got, *bound)
		}
	}
}

func TestHQTitleRequiresExactStampedPane(t *testing.T) {
	attachment := resume.CodexFrontend{CodexBindingTarget: resume.CodexBindingTarget{
		Pane: "%11", PanePID: 123, TTY: "ttys001",
	}}
	home := t.TempDir()
	prefix := "%11\t123\t/dev/ttys001\t"
	for name, snapshot := range map[string]string{
		"stamped":     prefix + home,
		"unstamped":   prefix,
		"foreign":     prefix + t.TempDir(),
		"wrong pane":  "%21\t123\t/dev/ttys001\t" + home,
		"wrong pid":   "%11\t321\t/dev/ttys001\t" + home,
		"wrong tty":   "%11\t123\t/dev/ttys002\t" + home,
		"no terminal": "",
	} {
		t.Run(name, func(t *testing.T) {
			if got := matchesHQTitlePane(attachment, snapshot, home); got != (name == "stamped") {
				t.Fatalf("stamped pane match = %v", got)
			}
		})
	}
}
