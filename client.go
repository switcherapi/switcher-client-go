// Package client provides the Switcher Client SDK for Go.
//
// For usage examples see README.md:
// https://github.com/switcherapi/switcher-client-go#quick-start
package client

import (
	"net/http"
	"sync"
	"sync/atomic"
)

var globalClient atomic.Pointer[Client]
var getSwitcherAfterReadMissHook func()
var defaultClientBeforeCompareAndSwapHook func()

// Client is the primary SDK instance that holds configuration, cached snapshot, switchers
// and background workers. Use NewClient to construct or BuildContext to set the package-wide client.
type Client struct {
	mu        sync.RWMutex
	context   Context
	switchers map[string]*Switcher
	snapshot  *Snapshot

	mockMu sync.RWMutex
	mocks  map[string]*mockDefinition

	executionLogger *executionLogger
	throttleTokens  chan struct{}

	snapshotWatcher     *snapshotWatcher
	snapshotAutoUpdater *snapshotAutoUpdater

	authMu       sync.Mutex
	authToken    string
	authTokenExp int64

	httpClientMu sync.Mutex
	httpClient_  *http.Client

	notifyErrorMu       sync.RWMutex
	notifyErrorCallback func(error)
}

// NewClient creates a new Client with defaults applied from the provided Context.
//
// Use BuildContext to install a global package-level client.
func NewClient(ctx Context) *Client {
	defaulted := ctx.withDefaults()
	return &Client{
		context:             defaulted,
		switchers:           make(map[string]*Switcher),
		mocks:               make(map[string]*mockDefinition),
		executionLogger:     newExecutionLogger(),
		throttleTokens:      newThrottleTokens(defaulted.Options.ThrottleMaxWorkers),
		snapshotWatcher:     newSnapshotWatcher(),
		snapshotAutoUpdater: newSnapshotAutoUpdater(),
	}
}

// BuildContext installs a package-level default client built from ctx.
func BuildContext(ctx Context) {
	client := NewClient(ctx)
	if current := globalClient.Load(); current != nil {
		current.stopBackgroundTasks()
	}
	globalClient.Store(client)

	client.ScheduleSnapshotAutoUpdate(0, nil)
}

// Context returns the client's effective Context (configuration).
func (c *Client) Context() Context {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.context
}

// GetSwitcher returns a Switcher for key.
func GetSwitcher(key string) *Switcher {
	return defaultClient().GetSwitcher(key)
}

// GetSwitcher returns a Switcher instance bound to this Client.
//
// Created Switchers are cached
// on the Client so subsequent calls for the same key return the same instance.
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

// LoadSnapshot delegates to the default client to load snapshot data from disk and optionally remote.
//
// Returns the loaded snapshot version or an error.
func LoadSnapshot(options *LoadSnapshotOptions) (int, error) {
	return defaultClient().LoadSnapshot(options)
}

// LoadSnapshot loads the snapshot according to options.
//
// It can read local files, check remote version
// and start watching for changes when requested.
func (c *Client) LoadSnapshot(options *LoadSnapshotOptions) (int, error) {
	settings := LoadSnapshotOptions{}
	if options != nil {
		settings = *options
	}

	if _, err := c.loadSnapshotFromCurrentFile(); err != nil {
		return 0, err
	}

	if c.shouldCheckSnapshot(settings.FetchRemote) {
		if _, err := c.CheckSnapshot(); err != nil {
			return 0, err
		}
	}

	if settings.WatchSnapshot {
		if err := c.WatchSnapshot(WatchSnapshotCallback{}); err != nil {
			return 0, err
		}
	}

	return c.SnapshotVersion(), nil
}

// SnapshotVersion returns current snapshot version from the package default client.
func SnapshotVersion() int {
	return defaultClient().SnapshotVersion()
}

// SnapshotVersion returns the client's current snapshot version (0 when not loaded).
func (c *Client) SnapshotVersion() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.snapshot == nil {
		return 0
	}

	return c.snapshot.Domain.Version
}

// CheckSnapshot checks the remote API for an updated snapshot and applies it when newer.
//
// Returns true when a new snapshot was applied.
func CheckSnapshot() (bool, error) {
	return defaultClient().CheckSnapshot()
}

// CheckSnapshot checks for and applies a newer snapshot from the remote API.
//
// Returns true when updated.
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

// CheckSwitchers validates that the provided switcher keys exist in the current snapshot
// or on the remote API, depending on the client's mode.
func CheckSwitchers(switcherKeys []string) error {
	return defaultClient().CheckSwitchers(switcherKeys)
}

// CheckSwitchers validates switcher keys against local snapshot data or the remote API.
func (c *Client) CheckSwitchers(switcherKeys []string) error {
	if c.Context().Options.Local {
		return checkLocalSwitchers(c.snapshotState(), switcherKeys)
	}

	token, err := c.ensureToken()
	if err != nil {
		return err
	}

	if err := missingTokenError(token); err != nil {
		return err
	}

	return c.checkSwitchers(token, switcherKeys)
}

// GetExecution retrieves the last execution log entry for the provided Switcher using the default client.
func GetExecution(switcher *Switcher) ExecutionEntry {
	return defaultClient().GetExecution(switcher)
}

// GetExecution returns the recorded execution entry for a Switcher, or an empty entry when nil.
func (c *Client) GetExecution(switcher *Switcher) ExecutionEntry {
	if switcher == nil {
		return ExecutionEntry{}
	}

	execution := switcher.snapshotForExecution()
	return c.executionLogger.get(execution.key, execution.entries)
}

// ClearLogger clears the execution log cache on the default client.
func ClearLogger() {
	defaultClient().ClearLogger()
}

// ClearLogger clears the client's execution logger cache.
func (c *Client) ClearLogger() {
	c.executionLogger.clear()
}

// SubscribeNotifyError registers a callback to receive asynchronous SDK errors on the default client.
func SubscribeNotifyError(callback func(error)) {
	defaultClient().SubscribeNotifyError(callback)
}

// SubscribeNotifyError registers a per-client callback invoked when the SDK encounters errors.
func (c *Client) SubscribeNotifyError(callback func(error)) {
	c.notifyErrorMu.Lock()
	defer c.notifyErrorMu.Unlock()

	c.notifyErrorCallback = callback
}

func (c *Client) notifyError(err error) {
	c.notifyErrorMu.RLock()
	callback := c.notifyErrorCallback
	c.notifyErrorMu.RUnlock()

	if callback != nil {
		callback(err)
	}
}

func (c *Client) runBackgroundTask(task func()) {
	if c.throttleTokens == nil {
		go task()
		return
	}

	go func() {
		c.throttleTokens <- struct{}{}
		defer func() {
			<-c.throttleTokens
		}()

		task()
	}()
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

func newThrottleTokens(maxWorkers int) chan struct{} {
	if maxWorkers <= 0 {
		return nil
	}

	return make(chan struct{}, maxWorkers)
}
