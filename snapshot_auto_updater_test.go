package client

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotAutoUpdater(t *testing.T) {
	t.Run("should load the snapshot from remote when fetch remote is enabled", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusOK,
			snapshotCheckBody:   map[string]any{"status": false},
			resolveStatus:       http.StatusOK,
			resolveDomain:       loadSnapshotFixture(t, "default_load_1"),
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:      "My Domain",
			URL:         server.URL,
			APIKey:      "[YOUR_API_KEY]",
			Component:   "MyApp",
			Environment: "default_load_1",
			Options: ContextOptions{
				Local: true,
			},
		})

		version, err := client.LoadSnapshot(&LoadSnapshotOptions{FetchRemote: true})
		enabled, enabledErr := client.GetSwitcher("FF2FOR2030").IsOn()

		assert.NoError(t, err)
		assert.NoError(t, enabledErr)
		assert.Equal(t, 1588557288040, version)
		assert.Equal(t, 1588557288040, client.SnapshotVersion())
		assert.True(t, enabled)
	})

	t.Run("should auto update the snapshot on schedule", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus: http.StatusOK,
			authBody:   map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotChecks: []snapshotCheckStep{
				{status: http.StatusOK, body: map[string]any{"status": false}},
				{status: http.StatusOK, body: map[string]any{"status": false}},
			},
			resolveSteps: []resolveSnapshotStep{
				{status: http.StatusOK, domain: loadSnapshotFixture(t, "default_load_1")},
				{status: http.StatusOK, domain: loadSnapshotFixture(t, "default_load_2")},
			},
		})
		defer server.Close()

		snapshotDir := t.TempDir()
		client := NewClient(Context{
			Domain:      "My Domain",
			URL:         server.URL,
			APIKey:      "[YOUR_API_KEY]",
			Component:   "MyApp",
			Environment: "generated-auto-update",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotDir,
			},
		})
		t.Cleanup(client.TerminateSnapshotAutoUpdate)

		version, err := client.LoadSnapshot(&LoadSnapshotOptions{FetchRemote: true})
		require.NoError(t, err)
		require.Equal(t, 1588557288040, version)

		callbacks := make(chan struct {
			err     error
			updated bool
		}, 1)
		client.ScheduleSnapshotAutoUpdate(50*time.Millisecond, func(err error, updated bool) {
			select {
			case callbacks <- struct {
				err     error
				updated bool
			}{err: err, updated: updated}:
			default:
			}
		})

		select {
		case callback := <-callbacks:
			assert.NoError(t, callback.err)
			assert.True(t, callback.updated)
		case <-time.After(5 * time.Second):
			t.Fatal("expected scheduled snapshot update callback")
		}

		assert.Eventually(t, func() bool {
			got, gotErr := client.GetSwitcher("FF2FOR2030").IsOn()
			return gotErr == nil && !got && client.SnapshotVersion() == 1588557288041
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("should schedule snapshot auto update using package function", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus: http.StatusOK,
			authBody:   map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotChecks: []snapshotCheckStep{
				{status: http.StatusOK, body: map[string]any{"status": false}},
				{status: http.StatusOK, body: map[string]any{"status": false}},
			},
			resolveSteps: []resolveSnapshotStep{
				{status: http.StatusOK, domain: loadSnapshotFixture(t, "default_load_1")},
				{status: http.StatusOK, domain: loadSnapshotFixture(t, "default_load_2")},
			},
		})
		defer server.Close()

		BuildContext(Context{
			Domain:      "My Domain",
			URL:         server.URL,
			APIKey:      "[YOUR_API_KEY]",
			Component:   "MyApp",
			Environment: "generated-auto-update-global",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: t.TempDir(),
			},
		})
		t.Cleanup(TerminateSnapshotAutoUpdate)

		version, err := LoadSnapshot(&LoadSnapshotOptions{FetchRemote: true})
		require.NoError(t, err)
		require.Equal(t, 1588557288040, version)

		callbacks := make(chan struct {
			err     error
			updated bool
		}, 1)
		ScheduleSnapshotAutoUpdate(50*time.Millisecond, func(err error, updated bool) {
			select {
			case callbacks <- struct {
				err     error
				updated bool
			}{err: err, updated: updated}:
			default:
			}
		})

		select {
		case callback := <-callbacks:
			assert.NoError(t, callback.err)
			assert.True(t, callback.updated)
		case <-time.After(5 * time.Second):
			t.Fatal("expected scheduled snapshot update callback")
		}

		assert.Eventually(t, func() bool {
			got, gotErr := GetSwitcher("FF2FOR2030").IsOn()
			return gotErr == nil && !got && SnapshotVersion() == 1588557288041
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("should terminate snapshot auto update using package function", func(t *testing.T) {
		BuildContext(Context{
			Domain:      "My Domain",
			Environment: "default",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotFixtureDir(),
			},
		})
		t.Cleanup(TerminateSnapshotAutoUpdate)

		_, err := LoadSnapshot(nil)
		require.NoError(t, err)

		callbackCh := make(chan struct{}, 1)
		ScheduleSnapshotAutoUpdate(200*time.Millisecond, func(_ error, _ bool) {
			select {
			case callbackCh <- struct{}{}:
			default:
			}
		})
		TerminateSnapshotAutoUpdate()

		select {
		case <-callbackCh:
			t.Fatal("did not expect auto update callback after terminate")
		case <-time.After(400 * time.Millisecond):
		}
	})
}
