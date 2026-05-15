package client

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const snapshotWatcherPollInterval = 100 * time.Millisecond

type WatchSnapshotCallback struct {
	Success func()
	Reject  func(error)
}

type snapshotWatcher struct {
	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

func newSnapshotWatcher() *snapshotWatcher {
	return &snapshotWatcher{}
}

func WatchSnapshot(callback WatchSnapshotCallback) error {
	return defaultClient().WatchSnapshot(callback)
}

func (c *Client) WatchSnapshot(callback WatchSnapshotCallback) error {
	return c.snapshotWatcher.Start(c, callback)
}

func UnwatchSnapshot() {
	defaultClient().UnwatchSnapshot()
}

func (c *Client) UnwatchSnapshot() {
	if c.snapshotWatcher != nil {
		c.snapshotWatcher.Stop()
	}
}

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
