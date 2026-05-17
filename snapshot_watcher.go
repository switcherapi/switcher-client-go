package client

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// snapshotWatcherPollInterval is the polling frequency used when watching the snapshot file for changes.
const snapshotWatcherPollInterval = 100 * time.Millisecond

// WatchSnapshotCallback contains hooks invoked when the snapshot watcher detects a successful
// load (Success) or when loading the snapshot fails (Reject).
type WatchSnapshotCallback struct {
	Success func()
	Reject  func(error)
}

// snapshotWatcher monitors the snapshot file on disk and reloads it when changes are detected.
// It runs a short polling loop (snapshotWatcherPollInterval) and calls back to the provided
// WatchSnapshotCallback when the snapshot is successfully reloaded or when an error occurs.
type snapshotWatcher struct {
	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

func newSnapshotWatcher() *snapshotWatcher {
	return &snapshotWatcher{}
}

// WatchSnapshot starts monitoring the configured snapshot file using the package default client.
// The provided callback will be invoked on successful reloads or failures.
//
// See README snapshot
// management for usage: https://github.com/switcherapi/switcher-client-go#snapshot-management
func WatchSnapshot(callback WatchSnapshotCallback) error {
	return defaultClient().WatchSnapshot(callback)
}

// WatchSnapshot starts monitoring the snapshot file for this client. It returns an error
// when the snapshot location is not configured or the initial file cannot be stat'd.
func (c *Client) WatchSnapshot(callback WatchSnapshotCallback) error {
	return c.snapshotWatcher.Start(c, callback)
}

// UnwatchSnapshot stops monitoring the snapshot file for the package default client.
func UnwatchSnapshot() {
	defaultClient().UnwatchSnapshot()
}

// UnwatchSnapshot stops monitoring the snapshot file for this client instance.
func (c *Client) UnwatchSnapshot() {
	if c.snapshotWatcher != nil {
		c.snapshotWatcher.Stop()
	}
}

// Start begins watching the snapshot file. It verifies the snapshot location and initial
// file state, then polls the file periodically. On detected changes it attempts to reload
// the snapshot; successful loads trigger callback.Success, failures trigger callback.Reject.
func (w *snapshotWatcher) Start(client *Client, callback WatchSnapshotCallback) error {
	snapshotLocation := strings.TrimSpace(client.Context().Options.SnapshotLocation)
	if snapshotLocation == "" {
		return fmt.Errorf("snapshot location is not defined in the context options")
	}

	snapshotFile := snapshotFilePath(client.Context())
	info, err := os.Stat(snapshotFile)
	if err != nil {
		return err
	}

	w.Stop()

	stop := make(chan struct{})
	done := make(chan struct{})

	w.mu.Lock()
	w.stop = stop
	w.done = done
	w.mu.Unlock()

	go func() {
		defer close(done)

		ticker := time.NewTicker(snapshotWatcherPollInterval)
		defer ticker.Stop()

		lastModified := info.ModTime()
		lastSize := info.Size()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				currentInfo, statErr := os.Stat(snapshotFile)
				if statErr != nil {
					invokeWatchReject(callback, statErr)
					continue
				}

				if currentInfo.ModTime().Equal(lastModified) && currentInfo.Size() == lastSize {
					continue
				}

				lastModified = currentInfo.ModTime()
				lastSize = currentInfo.Size()

				if _, loadErr := client.loadSnapshotFromCurrentFile(); loadErr != nil {
					invokeWatchReject(callback, loadErr)
					continue
				}

				invokeWatchSuccess(callback)
			}
		}
	}()

	return nil
}

// Stop signals the watcher goroutine to exit and waits for it to complete.
func (w *snapshotWatcher) Stop() {
	w.mu.Lock()
	stop := w.stop
	done := w.done
	w.stop = nil
	w.done = nil
	w.mu.Unlock()

	if stop != nil {
		close(stop)
	}

	if done != nil {
		<-done
	}
}

func invokeWatchSuccess(callback WatchSnapshotCallback) {
	if callback.Success != nil {
		callback.Success()
	}
}

func invokeWatchReject(callback WatchSnapshotCallback, err error) {
	if callback.Reject != nil {
		callback.Reject(err)
	}
}
