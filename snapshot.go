package client

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type LoadSnapshotOptions struct {
	FetchRemote   bool
	WatchSnapshot bool
}

type Snapshot struct {
	Domain SnapshotDomain `json:"domain"`
}

type SnapshotDomain struct {
	Name        string          `json:"name"`
	Activated   bool            `json:"activated"`
	Version     int             `json:"version"`
	Groups      []SnapshotGroup `json:"group"`
	Description string          `json:"description"`
}

type SnapshotGroup struct {
	Name        string           `json:"name"`
	Activated   bool             `json:"activated"`
	Configs     []SnapshotConfig `json:"config"`
	Description string           `json:"description"`
}

type SnapshotConfig struct {
	Key         string             `json:"key"`
	Activated   bool               `json:"activated"`
	Strategies  []SnapshotStrategy `json:"strategies"`
	Relay       *SnapshotRelay     `json:"relay"`
	Description string             `json:"description"`
}

type SnapshotStrategy struct {
	Strategy  string   `json:"strategy"`
	Activated bool     `json:"activated"`
	Operation string   `json:"operation"`
	Values    []string `json:"values"`
}

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
	snapshotFile := filepath.Join(ctx.Options.SnapshotLocation, ctx.Environment+".json")
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
