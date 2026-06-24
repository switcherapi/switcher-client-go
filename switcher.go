package client

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Switcher represents a feature-flag accessor bound to a specific switcher key.
//
// Obtain a Switcher via client.GetSwitcher or GetSwitcher("KEY").
//
// Quick Start: https://github.com/switcherapi/switcher-client-go#quick-start
type Switcher struct {
	client         *Client
	key            string
	entries        []criteriaEntry
	throttlePeriod time.Duration
	nextRefreshAt  time.Time
	mu             sync.RWMutex
}

type executionMode uint8

const (
	executionModeLocal executionMode = iota
	executionModeSilentLocal
	executionModeRemote
)

// Validate ensures the Client context and the Switcher key are properly configured.
//
// Returns an error describing missing fields when validation fails.
func (s *Switcher) Validate() error {
	ctx := s.client.Context()
	missingFields := make([]string, 0, 3)

	if strings.TrimSpace(ctx.URL) == "" {
		missingFields = append(missingFields, "url")
	}

	if strings.TrimSpace(ctx.Component) == "" {
		missingFields = append(missingFields, "component")
	}

	if strings.TrimSpace(ctx.APIKey) == "" {
		missingFields = append(missingFields, "api_key")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf(
			"something went wrong: missing or empty required fields (%s)",
			strings.Join(missingFields, ", "),
		)
	}

	if strings.TrimSpace(s.key) == "" {
		return fmt.Errorf("something went wrong: missing key field")
	}

	return nil
}

// CheckValue appends a value-based strategy input to this Switcher and returns the Switcher
// to allow method chaining (fluent API).
func (s *Switcher) CheckValue(input string) *Switcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = upsertEntry(s.entries, criteriaEntry{
		Strategy: StrategyValue,
		Input:    input,
	})

	return s
}

// CheckNetwork appends a network-based strategy input (e.g., IP or CIDR) to this Switcher
// and returns the Switcher for chaining.
func (s *Switcher) CheckNetwork(input string) *Switcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = upsertEntry(s.entries, criteriaEntry{
		Strategy: StrategyNetwork,
		Input:    input,
	})

	return s
}

// Throttle enables stale-while-revalidate behavior for this Switcher. When enabled the SDK
// may return a cached result immediately and refresh the value in the background.
// A non-positive period disables throttling for this Switcher.
func (s *Switcher) Throttle(period time.Duration) *Switcher {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.throttlePeriod = period
	if period <= 0 {
		s.nextRefreshAt = time.Time{}
		return s
	}

	if s.nextRefreshAt.IsZero() {
		s.nextRefreshAt = time.Now().Add(period)
	}

	return s
}

// Prepare validates the switcher can be executed and ensures an auth token is present.
// If key is non-empty it will be set on the Switcher. Useful when preparing before execution.
func (s *Switcher) Prepare(key string) error {
	if strings.TrimSpace(key) != "" {
		s.mu.Lock()
		s.key = key
		s.mu.Unlock()
	}

	execution := s.snapshotForExecution()
	if err := execution.Validate(); err != nil {
		return err
	}

	token, err := s.client.ensureToken()
	if err != nil {
		return err
	}

	return missingTokenError(token)
}

// IsOn evaluates the Switcher and returns true when the feature is enabled.
// It performs the standard execution flow and may return an error for remote failures.
func (s *Switcher) IsOn() (bool, error) {
	result, err := s.submit(false)
	if err != nil {
		return false, err
	}

	return result.Result, nil
}

// IsOnWithDetails evaluates the Switcher and returns a ResultDetail containing the boolean
// result plus metadata, reason and other execution information.
func (s *Switcher) IsOnWithDetails() (ResultDetail, error) {
	return s.submit(true)
}

// IsOnOrDefault evaluates the Switcher and returns the provided default boolean when an
// error occurs. The client's error callback is notified when available.
func (s *Switcher) IsOnOrDefault(def bool) bool {
	got, err := s.IsOn()
	if err != nil {
		if s != nil && s.client != nil {
			s.client.notifyError(err)
		}
		return def
	}
	return got
}

// IsOnWithDetailsOrDefault evaluates the Switcher and returns the provided default ResultDetail
// when an error occurs. The client's error callback is notified when available.
func (s *Switcher) IsOnWithDetailsOrDefault(def ResultDetail) ResultDetail {
	res, err := s.IsOnWithDetails()
	if err != nil {
		if s != nil && s.client != nil {
			s.client.notifyError(err)
		}
		return def
	}
	return res
}

func (s *Switcher) submit(showDetails bool) (ResultDetail, error) {
	execution := s.snapshotForExecution()
	if mocked, ok := execution.client.mockedResult(execution); ok {
		execution.logResult(mocked)
		s.markFreshExecution()
		return mocked, nil
	}

	if cached, ok := s.tryCachedResult(execution, showDetails); ok {
		return cached, nil
	}

	result, err := s.execute(execution, showDetails)
	if err != nil {
		return ResultDetail{}, err
	}

	s.markFreshExecution()
	return result, nil
}

func (s *Switcher) tryCachedResult(execution *Switcher, showDetails bool) (ResultDetail, bool) {
	if !execution.hasThrottle() {
		return ResultDetail{}, false
	}

	entry := execution.client.executionLogger.get(execution.key, execution.entries)
	if entry.Key == "" {
		return ResultDetail{}, false
	}

	if !execution.client.Context().Options.Freeze && s.shouldScheduleRefresh(time.Now()) {
		s.scheduleBackgroundRefresh(execution, showDetails)
	}

	return entry.Response, true
}

func (s *Switcher) execute(execution *Switcher, showDetails bool) (ResultDetail, error) {
	mode, err := execution.resolveExecutionMode()
	if err != nil {
		return ResultDetail{}, err
	}

	result, err := execution.executeMode(mode, showDetails)
	if err != nil {
		return ResultDetail{}, err
	}

	execution.logResult(result)
	return result, nil
}

func (s *Switcher) resolveExecutionMode() (executionMode, error) {
	if s.client.Context().Options.Local {
		return executionModeLocal, nil
	}

	if err := s.Validate(); err != nil {
		return executionModeRemote, err
	}

	if s.client.shouldUseLocalSilentMode() {
		return executionModeSilentLocal, nil
	}

	return executionModeRemote, nil
}

func (s *Switcher) executeMode(mode executionMode, showDetails bool) (ResultDetail, error) {
	if mode == executionModeLocal || mode == executionModeSilentLocal {
		return s.executeLocal()
	}

	return s.executeRemote(showDetails)
}

func (s *Switcher) executeLocal() (ResultDetail, error) {
	return checkLocalCriteria(s.client.snapshotState(), s)
}

func (s *Switcher) executeRemote(showDetails bool) (ResultDetail, error) {
	token, err := s.remoteToken()
	if err != nil {
		return s.client.fallbackToSilentMode(s, err)
	}

	result, err := s.client.checkCriteria(token, s, showDetails)
	if err != nil {
		return s.client.fallbackToSilentMode(s, err)
	}

	return result, nil
}

func (s *Switcher) remoteToken() (string, error) {
	token, err := s.client.ensureToken()
	if err != nil {
		return "", err
	}

	if err := missingTokenError(token); err != nil {
		return "", err
	}

	return token, nil
}

func (s *Switcher) snapshotForExecution() *Switcher {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clonedEntries := make([]criteriaEntry, len(s.entries))
	copy(clonedEntries, s.entries)

	return &Switcher{
		client:         s.client,
		key:            s.key,
		entries:        clonedEntries,
		throttlePeriod: s.throttlePeriod,
		nextRefreshAt:  s.nextRefreshAt,
	}
}

func (s *Switcher) logResult(result ResultDetail) {
	if !s.canLog() {
		return
	}

	s.client.executionLogger.add(s.key, s.entries, result)
}

func (s *Switcher) canLog() bool {
	return strings.TrimSpace(s.key) != "" && (s.client.Context().Options.Logger || s.hasThrottle())
}

func (s *Switcher) hasThrottle() bool {
	return s.throttlePeriod > 0
}

func (s *Switcher) markFreshExecution() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.throttlePeriod <= 0 {
		return
	}

	s.nextRefreshAt = time.Now().Add(s.throttlePeriod)
}

func (s *Switcher) shouldScheduleRefresh(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.throttlePeriod <= 0 || s.nextRefreshAt.IsZero() || !now.After(s.nextRefreshAt) {
		return false
	}

	s.nextRefreshAt = now.Add(s.throttlePeriod)
	return true
}

func (s *Switcher) scheduleBackgroundRefresh(execution *Switcher, showDetails bool) {
	s.client.runBackgroundTask(func() {
		if _, err := s.execute(execution, showDetails); err != nil {
			s.client.notifyError(err)
		}
	})
}

func upsertEntry(entries []criteriaEntry, next criteriaEntry) []criteriaEntry {
	for i := range entries {
		if entries[i].Strategy == next.Strategy {
			entries[i] = next
			return entries
		}
	}

	return append(entries, next)
}
