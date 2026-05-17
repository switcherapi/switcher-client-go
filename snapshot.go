package client

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadSnapshotOptions controls behavior when loading snapshots programmatically.
// FetchRemote will cause the client to check the remote API for a newer snapshot.
// WatchSnapshot will start a file watcher after loading.
type LoadSnapshotOptions struct {
	FetchRemote   bool
	WatchSnapshot bool
}

// Snapshot is the top-level representation of a domain snapshot loaded from disk or remote.
type Snapshot struct {
	Domain SnapshotDomain `json:"domain"`
}

// SnapshotDomain describes a Switcher domain and contains groups of configs.
type SnapshotDomain struct {
	Name        string          `json:"name"`
	Activated   bool            `json:"activated"`
	Version     int             `json:"version"`
	Groups      []SnapshotGroup `json:"group"`
	Description string          `json:"description"`
}

// SnapshotGroup represents a logical grouping of Switcher configs within a domain.
type SnapshotGroup struct {
	Name        string           `json:"name"`
	Activated   bool             `json:"activated"`
	Configs     []SnapshotConfig `json:"config"`
	Description string           `json:"description"`
}

// SnapshotConfig describes a single feature flag (switcher) stored in the snapshot.
type SnapshotConfig struct {
	Key         string             `json:"key"`
	Activated   bool               `json:"activated"`
	Strategies  []SnapshotStrategy `json:"strategies"`
	Relay       *SnapshotRelay     `json:"relay"`
	Description string             `json:"description"`
}

// SnapshotStrategy models a single strategy entry within a snapshot config.
type SnapshotStrategy struct {
	Strategy  string   `json:"strategy"`
	Activated bool     `json:"activated"`
	Operation string   `json:"operation"`
	Values    []string `json:"values"`
}

// SnapshotRelay models relay configuration associated with a snapshot config.
type SnapshotRelay struct {
	Type      string `json:"type"`
	Activated bool   `json:"activated"`
}

func createDefaultSnapshot(ctx Context, snapshotFile string) (*Snapshot, error) {
	snapshot := &Snapshot{
		Domain: SnapshotDomain{
			Version: 0,
		},
	}

	if ctx.Options.SnapshotLocation != "" {
		if err := os.MkdirAll(ctx.Options.SnapshotLocation, 0o755); err != nil {
			return nil, err
		}

		content, _ := json.MarshalIndent(snapshot, "", "    ")
		if err := os.WriteFile(snapshotFile, content, 0o644); err != nil {
			return nil, err
		}
	}

	return snapshot, nil
}

func loadSnapshotFromFile(ctx Context) (*Snapshot, error) {
	snapshotFile := snapshotFilePath(ctx)
	if _, err := os.Stat(snapshotFile); err != nil {
		return createDefaultSnapshot(ctx, snapshotFile)
	}

	content, _ := os.ReadFile(snapshotFile)

	var snapshot Snapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func saveSnapshotToFile(ctx Context, snapshot *Snapshot) error {
	if ctx.Options.SnapshotLocation == "" {
		return nil
	}

	if err := os.MkdirAll(ctx.Options.SnapshotLocation, 0o755); err != nil {
		return err
	}

	content, _ := json.MarshalIndent(snapshot, "", "    ")
	return os.WriteFile(snapshotFilePath(ctx), content, 0o644)
}

func snapshotFilePath(ctx Context) string {
	return filepath.Join(ctx.Options.SnapshotLocation, ctx.Environment+".json")
}
