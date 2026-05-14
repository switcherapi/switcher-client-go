package client

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotLoading(t *testing.T) {
	t.Run("should load snapshot version from a local file", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotFixtureDir(),
			},
		})

		assert.Equal(t, 0, SnapshotVersion())

		version, err := LoadSnapshot(nil)

		assert.NoError(t, err)
		assert.Equal(t, 1, version)
		assert.Equal(t, 1, SnapshotVersion())
	})

	t.Run("should return an error when the snapshot file is malformed", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotFixtureDir(),
			},
			Environment: "default_malformed",
		})

		version, err := LoadSnapshot(nil)

		assert.Error(t, err)
		assert.Zero(t, version)
	})

	t.Run("should return an error when the snapshot file is not accessible", func(t *testing.T) {
		snapshotLocation := filepath.Join(t.TempDir(), "snapshot-location-file")
		writeErr := os.WriteFile(snapshotLocation, []byte("not-a-directory"), 0o644)
		assert.NoError(t, writeErr)

		BuildContext(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotLocation,
			},
			Environment: "default",
		})

		version, err := LoadSnapshot(nil)

		assert.Error(t, err)
		assert.Zero(t, version)
	})

	t.Run("should return an error when the snapshot file is not accessible while saving remote updates", func(t *testing.T) {
		snapshotDir := t.TempDir()
		writeSnapshotFixture(t, snapshotDir, "default_load_1", "default_load_1")

		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusOK,
			snapshotCheckBody:   map[string]any{"status": false},
			resolveStatus:       http.StatusOK,
			resolveDomain:       loadSnapshotFixture(t, "default_load_2"),
		})
		defer server.Close()

		BuildContext(Context{
			Domain:      "My Domain",
			URL:         server.URL,
			APIKey:      "[YOUR_API_KEY]",
			Component:   "MyApp",
			Environment: "default_load_1",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})

		version, loadErr := LoadSnapshot(nil)
		assert.NoError(t, loadErr)
		assert.Equal(t, 1588557288040, version)

		removeErr := os.RemoveAll(snapshotDir)
		assert.NoError(t, removeErr)
		blockErr := os.WriteFile(snapshotDir, []byte("not-a-directory"), 0o644)
		assert.NoError(t, blockErr)

		updated, err := CheckSnapshot()

		assert.Error(t, err)
		assert.False(t, updated)
	})

	t.Run("should return an error when the snapshot file path cannot be created", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: t.TempDir(),
			},
			Environment: filepath.Join("nested", "missing"),
		})

		version, err := LoadSnapshot(nil)

		assert.Error(t, err)
		assert.Zero(t, version)
	})

	t.Run("should return an error when check snapshot fails during load snapshot", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusInternalServerError,
			snapshotCheckBody:   map[string]any{"status": false},
		})
		defer server.Close()

		BuildContext(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Local: false,
			},
		})

		version, err := LoadSnapshot(nil)

		assert.Error(t, err)
		assert.Zero(t, version)
		assert.EqualError(t, err, "[check_snapshot_version] failed with status: 500")
	})

	t.Run("should return an error when watch snapshot fails during load snapshot", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local: true,
			},
		})

		version, err := LoadSnapshot(&LoadSnapshotOptions{
			WatchSnapshot: true,
		})

		assert.Error(t, err)
		assert.Zero(t, version)
		assert.EqualError(t, err, "snapshot location is not defined in the context options")
	})

	t.Run("should create a clean snapshot when no file exists", func(t *testing.T) {
		snapshotDir := t.TempDir()

		BuildContext(Context{
			Domain:      "My Domain",
			Environment: "generated-clean",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})

		version, err := LoadSnapshot(nil)

		assert.NoError(t, err)
		assert.Equal(t, 0, version)
		assert.Equal(t, 0, SnapshotVersion())

		content, readErr := os.ReadFile(filepath.Join(snapshotDir, "generated-clean.json"))
		assert.NoError(t, readErr)

		var snapshot Snapshot
		unmarshalErr := json.Unmarshal(content, &snapshot)
		assert.NoError(t, unmarshalErr)
		assert.Equal(t, 0, snapshot.Domain.Version)
	})
}

func TestSwitcherLocalEvaluation(t *testing.T) {
	t.Run("should use local snapshot to evaluate a switcher without strategies", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("FF2FOR2022").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should use local snapshot to evaluate a switcher with value validation", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("FF2FOR2020").CheckValue("Japan").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when a value strategy does not receive any input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("FF2FOR2020").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'VALUE_VALIDATION' did not receive any input", got.Reason)
	})

	t.Run("should return enabled when value EXIST matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("VALUE_EXIST").CheckValue("guest").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when value EXIST does not match the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("VALUE_EXIST").CheckValue("anonymous").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'VALUE_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return disabled when operation is invalid for the strategy", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("HAS_ALL").CheckValue("anonymous").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'VALUE_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return disabled when strategy input does not match snapshot settings", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("FF2FOR2020").CheckNetwork("10.0.0.3").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'VALUE_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return enabled when value EQUAL matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("VALUE_EQUAL").CheckValue("pro-user").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when the domain is deactivated", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_disabled")

		got, err := GetSwitcher("FEATURE").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Domain is disabled", got.Reason)
	})

	t.Run("should return disabled when the group is deactivated", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("FF2FOR2040").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Group disabled", got.Reason)
	})

	t.Run("should return disabled when the config is deactivated", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("FF2FOR2031").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Config disabled", got.Reason)
	})

	t.Run("should return enabled when the strategy is deactivated", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("FF2FOR2021").CheckNetwork("10.0.0.3").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when the network input is inside a CIDR range", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_EXIST_CIDR").CheckNetwork("10.0.0.3").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when the network input is outside a CIDR range", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_EXIST_CIDR").CheckNetwork("192.168.1.2").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'NETWORK_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return enabled when the network input exactly matches an IP value", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_EXIST_IP").CheckNetwork("10.0.0.3").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when the network input is invalid", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_EXIST_CIDR").CheckNetwork("not-an-ip").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'NETWORK_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return enabled when NOT_EXIST network strategy does not match the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_NOT_EXIST_CIDR").CheckNetwork("192.168.1.10").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when NOT_EXIST network strategy matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_NOT_EXIST_CIDR").CheckNetwork("10.0.0.3").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'NETWORK_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return disabled when relay is enabled and relay restriction is active", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("USECASE103").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Config has relay enabled", got.Reason)
	})

	t.Run("should return an error when the key is not found in the snapshot", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		_, err := GetSwitcher("INVALID_KEY").IsOn()

		assert.Error(t, err)
		var localCriteriaErr *LocalCriteriaError
		assert.ErrorAs(t, err, &localCriteriaErr)
		assert.EqualError(t, err, "Config with key 'INVALID_KEY' not found in the snapshot")
	})

	t.Run("should return an error when no snapshot has been loaded", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: filepath.Join(t.TempDir(), "missing"),
			},
		})

		_, err := GetSwitcher("FF2FOR2022").IsOn()

		assert.Error(t, err)
		var localCriteriaErr *LocalCriteriaError
		assert.ErrorAs(t, err, &localCriteriaErr)
		assert.EqualError(t, err, "Snapshot not loaded. Try to use 'Client.load_snapshot()'")
	})
}

func useLocalSnapshotFixture(t *testing.T, environment string) {
	t.Helper()

	BuildContext(Context{
		Domain:      "My Domain",
		Environment: environment,
		Options: ContextOptions{
			Local:            true,
			SnapshotLocation: snapshotFixtureDir(),
		},
	})

	_, err := LoadSnapshot(nil)
	assert.NoError(t, err)
}

func snapshotFixtureDir() string {
	return filepath.Join("tests", "snapshots")
}
