package client

func (c *Client) snapshotState() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.snapshot
}

func (c *Client) setSnapshot(snapshot *Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.snapshot = snapshot
}

func (c *Client) stopBackgroundTasks() {
	c.TerminateSnapshotAutoUpdate()
	c.UnwatchSnapshot()
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
