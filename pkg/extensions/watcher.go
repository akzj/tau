package extensions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// WatchExtensions watches the extension directory for .js changes and triggers reload.
// Returns a cleanup function to stop watching.
func WatchExtensions(dir string, rt *Runtime) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify: %w", err)
	}

	if err := watcher.Add(dir); err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
				watcher.Close()
				return nil, fmt.Errorf("mkdir %s: %w", dir, mkErr)
			}
			if addErr := watcher.Add(dir); addErr != nil {
				watcher.Close()
				return nil, fmt.Errorf("watch after mkdir: %w", addErr)
			}
		} else {
			watcher.Close()
			return nil, fmt.Errorf("watch: %w", err)
		}
	}

	go func() {
		for {
			select {
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Ext(ev.Name) == ".js" &&
					(ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) || ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename)) {
					fmt.Fprintf(os.Stderr, "extensions: reloading (%s)\n", ev.Name)
					if err := rt.Reload(); err != nil {
						fmt.Fprintf(os.Stderr, "extensions: reload error: %v\n", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "extensions: watch error: %v\n", err)
			}
		}
	}()

	return func() { watcher.Close() }, nil
}
