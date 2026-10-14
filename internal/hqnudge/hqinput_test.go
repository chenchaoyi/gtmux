package hqnudge

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hqinput"
)

func TestPublicWakeChannelsKeepQueueWhileWorkspaceOwnsInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	previous := prod
	prod = f.io()
	t.Cleanup(func() { prod = previous })

	release, acquired, err := hqinput.Acquire()
	if err != nil || !acquired {
		t.Fatal("workspace input lease", acquired, err)
	}
	defer func() { release() }()
	Deliver("%isolated", "» gtmux·waiting ordinary wake")
	DeliverKeyedAt("%isolated", "same-key", "» gtmux·tick old keyed wake", 1)
	DeliverKeyedAt("%isolated", "same-key", "» gtmux·tick newest keyed wake", 1)
	Drain("%isolated")
	if f.pastes != 0 || len(f.sent) != 0 || f.slept != 0 {
		t.Fatal("wake touched input while another channel owns it", f)
	}
	if queuedCount(t) != 2 || !Pending() {
		t.Fatal("contended wake disappeared or lost keyed coalescing")
	}

	release()
	release = func() {}
	Drain("%isolated")
	if len(f.sent) != 1 || Pending() {
		t.Fatal("saved wakes did not drain after release", f.sent)
	}
	if !strings.Contains(f.sent[0], "ordinary wake") || !strings.Contains(f.sent[0], "newest keyed wake") || strings.Contains(f.sent[0], "old keyed wake") {
		t.Fatal("wrong queued payload delivered", f.sent)
	}
}
