package menubar

import (
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchState fires on the returned channel (debounced) whenever gtmux's state
// markers change — the active/ and waiting/ dirs under stateDir, written by
// `gtmux hook`. This makes waiting/finished transitions show up instantly; the
// app still polls on a timer for working/idle (which come from pane titles, not
// state files). The dirs are created if missing so the watch attaches cleanly.
//
// Returns the event channel and a stop function; stop closes the watcher.
func WatchState(stateDir string, debounce time.Duration) (<-chan struct{}, func(), error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}
	active := filepath.Join(stateDir, "active")
	waiting := filepath.Join(stateDir, "waiting")
	for _, d := range []string{stateDir, active, waiting} {
		_ = os.MkdirAll(d, 0o755)
		_ = w.Add(d) // watch the parent too, in case a subdir is created/removed
	}

	out := make(chan struct{}, 1)
	stop := make(chan struct{})
	go func() {
		defer w.Close()
		var timer *time.Timer
		fire := func() {
			select {
			case out <- struct{}{}:
			default: // a pending signal already covers this change
			}
		}
		for {
			select {
			case <-stop:
				return
			case _, ok := <-w.Events:
				if !ok {
					return
				}
				if timer == nil {
					timer = time.AfterFunc(debounce, fire)
				} else {
					timer.Reset(debounce)
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return out, func() { close(stop) }, nil
}
