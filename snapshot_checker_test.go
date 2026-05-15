package client

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotChecker(t *testing.T) {
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

	t.Run("should not update the snapshot when the current version is still valid", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusOK,
			snapshotCheckBody:   map[string]any{"status": true},
		})
		defer server.Close()

		snapshotDir := t.TempDir()
		writeSnapshotFixture(t, snapshotDir, "default_load_1", "default_load_1")

		client := NewClient(Context{
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

		version, loadErr := client.LoadSnapshot(nil)
		updated, err := client.CheckSnapshot()

		assert.NoError(t, loadErr)
		assert.NoError(t, err)
		assert.Equal(t, 1588557288040, version)
		assert.Equal(t, 1588557288040, client.SnapshotVersion())
		assert.False(t, updated)
	})

	t.Run("should return an error when the snapshot version check fails", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusInternalServerError,
			snapshotCheckBody:   map[string]any{"status": false},
		})
		defer server.Close()

		snapshotDir := t.TempDir()
		writeSnapshotFixture(t, snapshotDir, "default_load_1", "default_load_1")

		client := NewClient(Context{
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

		_, loadErr := client.LoadSnapshot(nil)
		updated, err := client.CheckSnapshot()

		assert.NoError(t, loadErr)
		assert.False(t, updated)
		assert.Error(t, err)
		var snapshotErr *RemoteSnapshotError
		assert.ErrorAs(t, err, &snapshotErr)
		assert.EqualError(t, err, "[check_snapshot_version] failed with status: 500")
	})

	t.Run("should return an error when snapshot version response cannot be decoded", func(t *testing.T) {
		rawBody := "{invalid-json"
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:           http.StatusOK,
			authBody:             map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus:  http.StatusOK,
			snapshotCheckRawBody: &rawBody,
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		updated, err := client.CheckSnapshot()

		assert.False(t, updated)
		assert.Error(t, err)
		var snapshotErr *RemoteSnapshotError
		assert.ErrorAs(t, err, &snapshotErr)
		assert.Contains(t, err.Error(), "[check_snapshot_version] failed to decode response")
	})

	t.Run("should return an error when snapshot version endpoint is unavailable", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusOK,
			snapshotCheckBody:   map[string]any{"status": true},
		})

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		_, tokenErr := client.ensureToken()
		assert.NoError(t, tokenErr)

		server.Close()

		updated, err := client.CheckSnapshot()

		assert.False(t, updated)
		assert.Error(t, err)
		var snapshotErr *RemoteSnapshotError
		assert.ErrorAs(t, err, &snapshotErr)
		assert.EqualError(t, err, "[check_snapshot_version] remote unavailable")
	})

	t.Run("should return an error when ensure token fails during snapshot check", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus: http.StatusUnauthorized,
			authBody:   map[string]any{},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		updated, err := client.CheckSnapshot()

		assert.False(t, updated)
		assert.Error(t, err)
		var authErr *RemoteAuthError
		assert.ErrorAs(t, err, &authErr)
		assert.EqualError(t, err, "invalid API key")
	})

	t.Run("should return an error when auth response does not include a token", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus: http.StatusOK,
			authBody:   map[string]any{"token": nil, "exp": time.Now().Add(time.Hour).Unix()},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		updated, err := client.CheckSnapshot()

		assert.False(t, updated)
		assert.EqualError(t, err, "something went wrong: missing token field")
	})

	t.Run("should return an error when resolving snapshot fails", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusOK,
			snapshotCheckBody:   map[string]any{"status": false},
			resolveStatus:       http.StatusInternalServerError,
			resolveDomain:       map[string]any{},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		updated, err := client.CheckSnapshot()

		assert.False(t, updated)
		assert.Error(t, err)
		var snapshotErr *RemoteSnapshotError
		assert.ErrorAs(t, err, &snapshotErr)
		assert.EqualError(t, err, "[resolve_snapshot] failed with status: 500")
	})

	t.Run("should return an error when snapshot resolve response cannot be decoded", func(t *testing.T) {
		rawBody := "{invalid-json"
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusOK,
			snapshotCheckBody:   map[string]any{"status": false},
			resolveStatus:       http.StatusOK,
			resolveRawBody:      &rawBody,
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		updated, err := client.CheckSnapshot()

		assert.False(t, updated)
		assert.Error(t, err)
		var snapshotErr *RemoteSnapshotError
		assert.ErrorAs(t, err, &snapshotErr)
		assert.Contains(t, err.Error(), "[resolve_snapshot] failed to decode response")
	})

	t.Run("should return an error when snapshot resolve endpoint is unavailable", func(t *testing.T) {
		server := newSnapshotTestServer(t, snapshotRemoteHandlers{
			authStatus:          http.StatusOK,
			authBody:            map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			snapshotCheckStatus: http.StatusOK,
			snapshotCheckBody:   map[string]any{"status": false},
			resolveUnavailable:  true,
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		updated, err := client.CheckSnapshot()

		assert.False(t, updated)
		assert.Error(t, err)
		var snapshotErr *RemoteSnapshotError
		assert.ErrorAs(t, err, &snapshotErr)
		assert.EqualError(t, err, "[resolve_snapshot] remote unavailable")
	})
}
