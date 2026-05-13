package client

import (
	"net"
	"slices"
)

func checkLocalCriteria(snapshot *Snapshot, switcher *Switcher) (ResultDetail, error) {
	if snapshot == nil {
		return ResultDetail{}, newLocalCriteriaError("Snapshot not loaded. Try to use 'Client.load_snapshot()'")
	}

	return checkLocalDomain(snapshot, switcher)
}

func checkLocalDomain(snapshot *Snapshot, switcher *Switcher) (ResultDetail, error) {
	if !snapshot.Domain.Activated {
		return ResultDetail{Result: false, Reason: "Domain is disabled", Metadata: map[string]any{}}, nil
	}

	return checkLocalGroup(snapshot.Domain.Groups, switcher)
}

func checkLocalGroup(groups []SnapshotGroup, switcher *Switcher) (ResultDetail, error) {
	key := switcher.key

	for _, group := range groups {
		for _, config := range group.Configs {
			if config.Key != key {
				continue
			}

			if !group.Activated {
				return ResultDetail{Result: false, Reason: "Group disabled", Metadata: map[string]any{}}, nil
			}

			return checkLocalConfig(config, switcher)
		}
	}

	return ResultDetail{}, newLocalCriteriaError("Config with key '%s' not found in the snapshot", switcher.key)
}

func checkLocalConfig(config SnapshotConfig, switcher *Switcher) (ResultDetail, error) {
	if !config.Activated {
		return ResultDetail{Result: false, Reason: "Config disabled", Metadata: map[string]any{}}, nil
	}

	if config.Relay != nil && config.Relay.Activated && switcher.client.Context().Options.RestrictRelay {
		return ResultDetail{Result: false, Reason: "Config has relay enabled", Metadata: map[string]any{}}, nil
	}

	return checkLocalStrategies(config.Strategies, switcher.entries)
}

func checkLocalStrategies(strategies []SnapshotStrategy, entries []criteriaEntry) (ResultDetail, error) {
	activeStrategies := 0

	for _, strategy := range strategies {
		if !strategy.Activated {
			continue
		}

		activeStrategies++
		if len(entries) == 0 {
			return ResultDetail{Result: false, Reason: "Strategy '" + strategy.Strategy + "' did not receive any input", Metadata: map[string]any{}}, nil
		}

		entry, ok := findCriteriaEntry(entries, strategy.Strategy)
		if !ok || !evaluateLocalStrategy(strategy, entry.Input) {
			return ResultDetail{Result: false, Reason: "Strategy '" + strategy.Strategy + "' does not agree", Metadata: map[string]any{}}, nil
		}
	}

	return ResultDetail{Result: true, Reason: "Success", Metadata: map[string]any{}}, nil
}

func findCriteriaEntry(entries []criteriaEntry, strategy string) (criteriaEntry, bool) {
	for _, entry := range entries {
		if entry.Strategy == strategy {
			return entry, true
		}
	}

	return criteriaEntry{}, false
}

func evaluateLocalStrategy(strategy SnapshotStrategy, input string) bool {
	switch strategy.Strategy {
	case StrategyValue:
		switch strategy.Operation {
		case "EXIST", "EQUAL":
			return containsString(strategy.Values, input)
		case "NOT_EXIST", "NOT_EQUAL":
			return !containsString(strategy.Values, input)
		}
	case StrategyNetwork:
		switch strategy.Operation {
		case "EXIST":
			return networkExists(strategy.Values, input)
		case "NOT_EXIST":
			return !networkExists(strategy.Values, input)
		}
	}

	return false
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}

func networkExists(values []string, input string) bool {
	ip := net.ParseIP(input)
	if ip == nil {
		return false
	}

	for _, value := range values {
		if _, network, err := net.ParseCIDR(value); err == nil {
			if network.Contains(ip) {
				return true
			}
			continue
		}

		if parsed := net.ParseIP(value); parsed != nil && parsed.Equal(ip) {
			return true
		}
	}

	return false
}
