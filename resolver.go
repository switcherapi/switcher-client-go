package client

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
		if !ok || !processLocalStrategy(strategy, entry.Input) {
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
