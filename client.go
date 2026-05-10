package client

import (
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

	authMu       sync.Mutex
	authToken    string
	authTokenExp int64
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
