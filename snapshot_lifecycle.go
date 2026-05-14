package client

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type snapshotCheckResponse struct {
	Status bool `json:"status"`
}

type resolveSnapshotResponse struct {
	Data struct {
		Domain SnapshotDomain `json:"domain"`
	} `json:"data"`
}

type snapshotWatcher struct {
	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

type snapshotAutoUpdater struct {
	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

func newSnapshotWatcher() *snapshotWatcher {
	return &snapshotWatcher{}
}

func newSnapshotAutoUpdater() *snapshotAutoUpdater {
	return &snapshotAutoUpdater{}
}

func CheckSnapshot() (bool, error) {
	return defaultClient().CheckSnapshot()
}

func (c *Client) CheckSnapshot() (bool, error) {
	token, err := c.ensureToken()
	if err != nil {
		return false, err
	}

	if err := missingTokenError(token); err != nil {
		return false, err
	}

	upToDate, err := c.checkSnapshotVersion(token, c.SnapshotVersion())
	if err != nil {
		return false, err
	}

	if upToDate {
		return false, nil
	}

	snapshot, err := c.resolveSnapshot(token)
	if err != nil {
		return false, err
	}

	if err := saveSnapshotToFile(c.Context(), snapshot); err != nil {
		return false, err
	}

	c.setSnapshot(snapshot)
	return true, nil
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

func ScheduleSnapshotAutoUpdate(interval time.Duration, callback func(error, bool)) {
	defaultClient().ScheduleSnapshotAutoUpdate(interval, callback)
}

func (c *Client) ScheduleSnapshotAutoUpdate(interval time.Duration, callback func(error, bool)) {
	if interval > 0 {
		c.mu.Lock()
		c.context.Options.SnapshotAutoUpdateInterval = interval
		c.mu.Unlock()
	}

	effectiveInterval := interval
	if effectiveInterval <= 0 {
		effectiveInterval = c.Context().Options.SnapshotAutoUpdateInterval
	}

	if effectiveInterval <= 0 || c.snapshotAutoUpdater == nil {
		return
	}

	c.snapshotAutoUpdater.Start(c, effectiveInterval, callback)
}

func TerminateSnapshotAutoUpdate() {
	defaultClient().TerminateSnapshotAutoUpdate()
}

func (c *Client) TerminateSnapshotAutoUpdate() {
	if c.snapshotAutoUpdater != nil {
		c.snapshotAutoUpdater.Stop()
	}
}

func (c *Client) shouldCheckSnapshot(fetchRemote bool) bool {
	ctx := c.Context()
	return c.SnapshotVersion() == 0 && (fetchRemote || !ctx.Options.Local)
}

func (c *Client) loadSnapshotFromCurrentFile() (*Snapshot, error) {
	snapshot, err := loadSnapshotFromFile(c.Context())
	if err != nil {
		return nil, err
	}

	c.setSnapshot(snapshot)
	return snapshot, nil
}

func (c *Client) checkSnapshotVersion(token string, snapshotVersion int) (bool, error) {
	ctx := c.Context()
	endpoint := fmt.Sprintf("%s/criteria/snapshot_check/%d", strings.TrimRight(ctx.URL, "/"), snapshotVersion)

	response, err := c.doJSONRequest(
		http.MethodGet,
		endpoint,
		nil,
		map[string]string{
			"Authorization": "Bearer " + token,
			"Content-Type":  "application/json",
		},
	)
	if err != nil {
		return false, newRemoteSnapshotError("[check_snapshot_version] remote unavailable")
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return false, newRemoteSnapshotError("[check_snapshot_version] failed with status: %d", response.StatusCode)
	}

	var payload snapshotCheckResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return false, newRemoteSnapshotError("[check_snapshot_version] failed to decode response: %v", err)
	}

	return payload.Status, nil
}

func (c *Client) resolveSnapshot(token string) (*Snapshot, error) {
	ctx := c.Context()
	endpoint := strings.TrimRight(ctx.URL, "/") + "/graphql"

	response, err := c.doJSONRequest(
		http.MethodPost,
		endpoint,
		map[string]string{
			"query": fmt.Sprintf(`
				query domain {
					domain(name: %q, environment: %q, _component: %q) {
						name version activated
						group { name activated
							config { key activated
								strategies { strategy activated operation values }
								relay { type activated }
							}
						}
					}
				}
			`, ctx.Domain, ctx.Environment, ctx.Component),
		},
		map[string]string{
			"Authorization": "Bearer " + token,
			"Content-Type":  "application/json",
		},
	)
	if err != nil {
		return nil, newRemoteSnapshotError("[resolve_snapshot] remote unavailable")
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return nil, newRemoteSnapshotError("[resolve_snapshot] failed with status: %d", response.StatusCode)
	}

	var payload resolveSnapshotResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, newRemoteSnapshotError("[resolve_snapshot] failed to decode response: %v", err)
	}

	return &Snapshot{Domain: payload.Data.Domain}, nil
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

func (u *snapshotAutoUpdater) Start(client *Client, interval time.Duration, callback func(error, bool)) {
	u.Stop()

	stop := make(chan struct{})
	done := make(chan struct{})

	u.mu.Lock()
	u.stop = stop
	u.done = done
	u.mu.Unlock()

	go func() {
		defer close(done)

		timer := time.NewTimer(interval)
		defer timer.Stop()

		select {
		case <-stop:
			return
		case <-timer.C:
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			updated, err := client.CheckSnapshot()
			if callback != nil {
				callback(err, updated)
			}

			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
}

func (u *snapshotAutoUpdater) Stop() {
	u.mu.Lock()
	stop := u.stop
	done := u.done
	u.stop = nil
	u.done = nil
	u.mu.Unlock()

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
