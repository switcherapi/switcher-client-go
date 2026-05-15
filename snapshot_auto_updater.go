package client

import (
	"sync"
	"time"
)

type snapshotAutoUpdater struct {
	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

func newSnapshotAutoUpdater() *snapshotAutoUpdater {
	return &snapshotAutoUpdater{}
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
