package client

import (
	"fmt"
	"strings"
)

const (
	mockReasonTrue  = "Forced to True"
	mockReasonFalse = "Forced to False"
)

type cleanupRegistrar interface {
	Cleanup(func())
	Helper()
}

type MockAssumption struct {
	client *Client
	key    string
	entry  *mockDefinition
}

type mockDefinition struct {
	result   bool
	reason   string
	metadata map[string]any
	when     []mockCondition
}

type mockCondition struct {
	Strategy string
	Inputs   []string
}

// Assume registers a client-scoped mocked result for key and returns a fluent builder.
func (c *Client) Assume(key string) *MockAssumption {
	c.mockMu.Lock()
	defer c.mockMu.Unlock()

	entry := &mockDefinition{}
	c.mocks[key] = entry
	return &MockAssumption{
		client: c,
		key:    key,
		entry:  entry,
	}
}

// Forget removes a previously assumed mocked result for key from this client.
func (c *Client) Forget(key string) {
	c.mockMu.Lock()
	defer c.mockMu.Unlock()

	delete(c.mocks, key)
}

// True forces the switcher result to true for this assumption.
func (a *MockAssumption) True() *MockAssumption {
	a.client.mockMu.Lock()
	a.entry.result = true
	a.entry.reason = mockReasonTrue
	a.client.mockMu.Unlock()
	return a
}

// False forces the switcher result to false for this assumption.
func (a *MockAssumption) False() *MockAssumption {
	a.client.mockMu.Lock()
	a.entry.result = false
	a.entry.reason = mockReasonFalse
	a.client.mockMu.Unlock()
	return a
}

// When limits the mock to specific strategy inputs. Unsupported strategies are ignored.
func (a *MockAssumption) When(strategy string, input any) *MockAssumption {
	inputs := normalizeMockInputs(input)

	a.client.mockMu.Lock()
	a.entry.when = upsertMockCondition(a.entry.when, mockCondition{
		Strategy: strategy,
		Inputs:   inputs,
	})
	a.client.mockMu.Unlock()
	return a
}

// WithMetadata attaches response metadata to the mocked result.
func (a *MockAssumption) WithMetadata(metadata map[string]any) *MockAssumption {
	a.client.mockMu.Lock()
	a.entry.metadata = cloneMetadata(metadata)
	a.client.mockMu.Unlock()
	return a
}

// Cleanup registers automatic cleanup for the assumption through a test-like cleanup registrar.
func (a *MockAssumption) Cleanup(t cleanupRegistrar) *MockAssumption {
	t.Helper()
	client := a.client
	key := a.key
	t.Cleanup(func() {
		client.Forget(key)
	})
	return a
}

func (c *Client) mockedResult(switcher *Switcher) (ResultDetail, bool) {
	c.mockMu.RLock()
	entry, ok := c.mocks[switcher.key]
	if !ok {
		c.mockMu.RUnlock()
		return ResultDetail{}, false
	}

	definition := entry.clone()
	c.mockMu.RUnlock()

	return definition.responseFor(switcher.entries), true
}

func (m *mockDefinition) clone() mockDefinition {
	cloned := mockDefinition{
		result:   m.result,
		reason:   m.reason,
		metadata: cloneMetadata(m.metadata),
		when:     make([]mockCondition, len(m.when)),
	}
	for i := range m.when {
		cloned.when[i] = mockCondition{
			Strategy: m.when[i].Strategy,
			Inputs:   append([]string(nil), m.when[i].Inputs...),
		}
	}

	return cloned
}

func (m mockDefinition) responseFor(entries []criteriaEntry) ResultDetail {
	result := m.result
	reason := m.reason
	for _, condition := range m.when {
		input, ok := findMockInput(entries, condition.Strategy)
		if !ok || containsString(condition.Inputs, input) {
			continue
		}

		result = !m.result
		reason = mismatchMockReason(result, condition.Inputs, input)
		break
	}

	return ResultDetail{
		Result:   result,
		Reason:   reason,
		Metadata: cloneMetadata(m.metadata),
	}
}

func findMockInput(entries []criteriaEntry, strategy string) (string, bool) {
	for _, entry := range entries {
		if entry.Strategy == strategy {
			return entry.Input, true
		}
	}

	return "", false
}

func upsertMockCondition(conditions []mockCondition, next mockCondition) []mockCondition {
	for i := range conditions {
		if conditions[i].Strategy == next.Strategy {
			conditions[i] = next
			return conditions
		}
	}

	return append(conditions, next)
}

func normalizeMockInputs(input any) []string {
	switch value := input.(type) {
	case []string:
		return append([]string(nil), value...)
	default:
		return []string{fmt.Sprint(value)}
	}
}

func mismatchMockReason(result bool, expected []string, input string) string {
	label := "False"
	if result {
		label = "True"
	}

	return fmt.Sprintf(
		"Forced to %s when: [%s] - input: %s",
		label,
		strings.Join(expected, ", "),
		input,
	)
}
