package client

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotWatcher(t *testing.T) {
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

	t.Run("should return an error when watch snapshot has no snapshot location", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
		})
		t.Cleanup(UnwatchSnapshot)

		err := WatchSnapshot(WatchSnapshotCallback{})

		assert.EqualError(t, err, "snapshot location is not defined in the context options")
	})

	t.Run("should watch snapshot using package function", func(t *testing.T) {
		snapshotDir := t.TempDir()
		writeSnapshotFixture(t, snapshotDir, "watched-global", "default_load_1")

		BuildContext(Context{
			Domain:      "My Domain",
			Environment: "watched-global",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})
		t.Cleanup(UnwatchSnapshot)

		version, err := LoadSnapshot(nil)
		require.NoError(t, err)
		require.Equal(t, 1588557288040, version)

		successCh := make(chan struct{}, 1)
		watchErr := WatchSnapshot(WatchSnapshotCallback{
			Success: func() {
				select {
				case successCh <- struct{}{}:
				default:
				}
			},
		})
		require.NoError(t, watchErr)

		writeSnapshotFixture(t, snapshotDir, "watched-global", "default_load_2")

		select {
		case <-successCh:
		case <-time.After(5 * time.Second):
			t.Fatal("expected watch snapshot callback")
		}

		assert.Eventually(t, func() bool {
			got, gotErr := GetSwitcher("FF2FOR2030").IsOn()
			return gotErr == nil && !got && SnapshotVersion() == 1588557288041
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("should stop watching snapshot using package unwatch function", func(t *testing.T) {
		snapshotDir := t.TempDir()
		writeSnapshotFixture(t, snapshotDir, "watched-global-stop", "default_load_1")

		BuildContext(Context{
			Domain:      "My Domain",
			Environment: "watched-global-stop",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})
		t.Cleanup(UnwatchSnapshot)

		_, err := LoadSnapshot(nil)
		require.NoError(t, err)

		callbackCh := make(chan struct{}, 1)
		watchErr := WatchSnapshot(WatchSnapshotCallback{
			Success: func() {
				select {
				case callbackCh <- struct{}{}:
				default:
				}
			},
		})
		require.NoError(t, watchErr)

		UnwatchSnapshot()
		writeSnapshotFixture(t, snapshotDir, "watched-global-stop", "default_load_2")

		select {
		case <-callbackCh:
			t.Fatal("did not expect watch callback after unwatch")
		case <-time.After(400 * time.Millisecond):
		}

		assert.Equal(t, 1588557288040, SnapshotVersion())
	})

	t.Run("should return an error when watch snapshot cannot stat the file at startup", func(t *testing.T) {
		snapshotDir := t.TempDir()

		client := NewClient(Context{
			Domain:      "My Domain",
			Environment: "missing-watch-file",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})
		t.Cleanup(client.UnwatchSnapshot)

		err := client.WatchSnapshot(WatchSnapshotCallback{})

		assert.Error(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("should watch the snapshot file when load snapshot enables watch mode", func(t *testing.T) {
		snapshotDir := t.TempDir()
		writeSnapshotFixture(t, snapshotDir, "watched", "default_load_1")

		client := NewClient(Context{
			Domain:      "My Domain",
			Environment: "watched",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})
		t.Cleanup(client.UnwatchSnapshot)

		version, err := client.LoadSnapshot(&LoadSnapshotOptions{WatchSnapshot: true})
		require.NoError(t, err)
		require.Equal(t, 1588557288040, version)

		enabled, enabledErr := client.GetSwitcher("FF2FOR2030").IsOn()
		require.NoError(t, enabledErr)
		require.True(t, enabled)

		// delay to ensure the watcher goroutine is running before the file is updated
		time.Sleep(100 * time.Millisecond)

		writeSnapshotFixture(t, snapshotDir, "watched", "default_load_2")

		require.Eventually(t, func() bool {
			got, gotErr := client.GetSwitcher("FF2FOR2030").IsOn()
			return gotErr == nil && !got && client.SnapshotVersion() == 1588557288041
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("should reject watch updates when the modified snapshot is malformed", func(t *testing.T) {
		snapshotDir := t.TempDir()
		writeSnapshotFixture(t, snapshotDir, "watched", "default_load_1")

		client := NewClient(Context{
			Domain:      "My Domain",
			Environment: "watched",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})
		t.Cleanup(client.UnwatchSnapshot)

		_, err := client.LoadSnapshot(nil)
		require.NoError(t, err)

		rejectCh := make(chan error, 1)
		watchErr := client.WatchSnapshot(WatchSnapshotCallback{
			Reject: func(err error) {
				select {
				case rejectCh <- err:
				default:
				}
			},
		})
		require.NoError(t, watchErr)

		content, readErr := os.ReadFile(filepath.Join(snapshotFixtureDir(), "default_malformed.json"))
		require.NoError(t, readErr)
		writeErr := os.WriteFile(filepath.Join(snapshotDir, "watched.json"), content, 0o644)
		require.NoError(t, writeErr)

		select {
		case rejectErr := <-rejectCh:
			assert.Error(t, rejectErr)
		case <-time.After(5 * time.Second):
			t.Fatal("expected malformed snapshot watch callback")
		}
	})

	t.Run("should reject watch updates when watched snapshot file becomes unavailable", func(t *testing.T) {
		snapshotDir := t.TempDir()
		environment := "watched-missing-runtime"
		writeSnapshotFixture(t, snapshotDir, environment, "default_load_1")

		client := NewClient(Context{
			Domain:      "My Domain",
			Environment: environment,
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})
		t.Cleanup(client.UnwatchSnapshot)

		_, err := client.LoadSnapshot(nil)
		require.NoError(t, err)

		rejectCh := make(chan error, 1)
		watchErr := client.WatchSnapshot(WatchSnapshotCallback{
			Reject: func(err error) {
				select {
				case rejectCh <- err:
				default:
				}
			},
		})
		require.NoError(t, watchErr)

		removeErr := os.Remove(filepath.Join(snapshotDir, environment+".json"))
		require.NoError(t, removeErr)

		select {
		case rejectErr := <-rejectCh:
			assert.Error(t, rejectErr)
			assert.True(t, errors.Is(rejectErr, os.ErrNotExist))
		case <-time.After(5 * time.Second):
			t.Fatal("expected stat error callback when watched snapshot file is removed")
		}
	})
}
