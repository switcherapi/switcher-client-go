package client

import (
	"net/http"
	"sync"
	"sync/atomic"
)

var globalClient atomic.Pointer[Client]
var getSwitcherAfterReadMissHook func()
var defaultClientBeforeCompareAndSwapHook func()

type Client struct {
	mu        sync.RWMutex
	context   Context
	switchers map[string]*Switcher
	snapshot  *Snapshot

	authMu       sync.Mutex
	authToken    string
	authTokenExp int64

	httpClientMu sync.Mutex
	httpClient_  *http.Client
}

func NewClient(ctx Context) *Client {
	return &Client{
		context:   ctx.withDefaults(),
		switchers: make(map[string]*Switcher),
	}
}

func BuildContext(ctx Context) {
	globalClient.Store(NewClient(ctx))
}

func GetSwitcher(key string) *Switcher {
	return defaultClient().GetSwitcher(key)
}

func (c *Client) GetSwitcher(key string) *Switcher {
	if key == "" {
		return &Switcher{
			client: c,
			key:    key,
		}
	}

	c.mu.RLock()
	switcher, ok := c.switchers[key]
	c.mu.RUnlock()
	if ok {
		return switcher
	}

	if getSwitcherAfterReadMissHook != nil {
		getSwitcherAfterReadMissHook()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if switcher, ok = c.switchers[key]; ok {
		return switcher
	}

	switcher = &Switcher{
		client: c,
		key:    key,
	}
	c.switchers[key] = switcher
	return switcher
}

func (c *Client) Context() Context {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.context
}

func LoadSnapshot(options *LoadSnapshotOptions) (int, error) {
	return defaultClient().LoadSnapshot(options)
}

func (c *Client) LoadSnapshot(options *LoadSnapshotOptions) (int, error) {
	snapshot, err := loadSnapshotFromFile(c.Context())
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	c.snapshot = snapshot
	c.mu.Unlock()

	return c.SnapshotVersion(), nil
}

func SnapshotVersion() int {
	return defaultClient().SnapshotVersion()
}

func (c *Client) SnapshotVersion() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.snapshot == nil {
		return 0
	}

	return c.snapshot.Domain.Version
}

func (c *Client) snapshotState() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.snapshot
}

func defaultClient() *Client {
	if client := globalClient.Load(); client != nil {
		return client
	}

	client := NewClient(Context{})
	if defaultClientBeforeCompareAndSwapHook != nil {
		defaultClientBeforeCompareAndSwapHook()
	}
	if globalClient.CompareAndSwap(nil, client) {
		return client
	}

	return globalClient.Load()
}
