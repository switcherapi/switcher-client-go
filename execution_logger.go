package client

import (
	"maps"
	"sync"
)

// ExecutionInput represents a single strategy input captured when a Switcher is evaluated.
type ExecutionInput struct {
	Strategy string
	Input    string
}

// ExecutionEntry stores a cached evaluation for a Switcher including inputs and the response.
// This is used by the client's execution logger to implement throttling and cached returns.
type ExecutionEntry struct {
	Key      string
	Inputs   []ExecutionInput
	Response ResultDetail
}

type executionLogger struct {
	mu      sync.RWMutex
	entries []ExecutionEntry
}

func newExecutionLogger() *executionLogger {
	return &executionLogger{
		entries: make([]ExecutionEntry, 0),
	}
}

func (l *executionLogger) add(key string, inputs []criteriaEntry, response ResultDetail) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i := range l.entries {
		if executionEntryMatches(l.entries[i], key, inputs) {
			l.entries = append(l.entries[:i], l.entries[i+1:]...)
			break
		}
	}

	l.entries = append(l.entries, ExecutionEntry{
		Key:      key,
		Inputs:   executionInputsFromCriteria(inputs),
		Response: cachedResultDetail(response),
	})
}

func (l *executionLogger) get(key string, inputs []criteriaEntry) ExecutionEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, entry := range l.entries {
		if executionEntryMatches(entry, key, inputs) {
			return cloneExecutionEntry(entry)
		}
	}

	return ExecutionEntry{}
}

func (l *executionLogger) clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = l.entries[:0]
}

func executionEntryMatches(entry ExecutionEntry, key string, inputs []criteriaEntry) bool {
	return entry.Key == key && executionInputsMatch(entry.Inputs, inputs)
}

func executionInputsMatch(logged []ExecutionInput, current []criteriaEntry) bool {
	if len(logged) == 0 {
		return len(current) == 0
	}

	if len(current) == 0 {
		return false
	}

	for _, loggedInput := range logged {
		found := false
		for _, currentInput := range current {
			if currentInput.Strategy == loggedInput.Strategy && currentInput.Input == loggedInput.Input {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func executionInputsFromCriteria(inputs []criteriaEntry) []ExecutionInput {
	if len(inputs) == 0 {
		return nil
	}

	converted := make([]ExecutionInput, len(inputs))
	for i, input := range inputs {
		converted[i] = ExecutionInput(input)
	}

	return converted
}

func cloneExecutionEntry(entry ExecutionEntry) ExecutionEntry {
	return ExecutionEntry{
		Key:      entry.Key,
		Inputs:   cloneExecutionInputs(entry.Inputs),
		Response: cloneResultDetail(entry.Response),
	}
}

func cloneExecutionInputs(inputs []ExecutionInput) []ExecutionInput {
	if len(inputs) == 0 {
		return nil
	}

	cloned := make([]ExecutionInput, len(inputs))
	copy(cloned, inputs)
	return cloned
}

func cloneResultDetail(result ResultDetail) ResultDetail {
	return ResultDetail{
		Result:   result.Result,
		Reason:   result.Reason,
		Metadata: cloneMetadata(result.Metadata),
	}
}

func cachedResultDetail(result ResultDetail) ResultDetail {
	cached := cloneResultDetail(result)
	cached.Metadata["cached"] = true
	return cached
}

func cloneMetadata(metadata map[string]any) map[string]any {
	cloned := make(map[string]any, len(metadata))
	maps.Copy(cloned, metadata)

	return cloned
}
