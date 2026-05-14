package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotLifecycle(t *testing.T) {
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
}

func TestSnapshotLifecycleGlobalFunctions(t *testing.T) {
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

func loadSnapshotFixture(t *testing.T, environment string) map[string]any {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(snapshotFixtureDir(), environment+".json"))
	require.NoError(t, err)

	var payload map[string]map[string]any
	err = json.Unmarshal(content, &payload)
	require.NoError(t, err)

	return payload["domain"]
}

func writeSnapshotFixture(t *testing.T, snapshotDir, environment, fixture string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(snapshotFixtureDir(), fixture+".json"))
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(snapshotDir, environment+".json"), content, 0o644)
	require.NoError(t, err)
}

type snapshotRemoteHandlers struct {
	authStatus           int
	authBody             map[string]any
	snapshotCheckStatus  int
	snapshotCheckBody    map[string]any
	snapshotCheckRawBody *string
	resolveStatus        int
	resolveDomain        map[string]any
	resolveRawBody       *string
	resolveUnavailable   bool
	snapshotChecks       []snapshotCheckStep
	resolveSteps         []resolveSnapshotStep
}

type snapshotCheckStep struct {
	status int
	body   map[string]any
}

type resolveSnapshotStep struct {
	status int
	domain map[string]any
}

func newSnapshotTestServer(t *testing.T, handlers snapshotRemoteHandlers) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	var snapshotCheckIndex int
	var resolveIndex int
	var mu sync.Mutex

	mux.HandleFunc("/criteria/auth", func(writer http.ResponseWriter, request *http.Request) {
		handleAuth(t, writer, request, handlers)
	})
	mux.HandleFunc("/criteria/snapshot_check/", func(writer http.ResponseWriter, request *http.Request) {
		handleSnapshotCheck(t, writer, request, handlers, &snapshotCheckIndex, &mu)
	})
	mux.HandleFunc("/graphql", func(writer http.ResponseWriter, request *http.Request) {
		handleGraphQL(t, writer, request, handlers, &resolveIndex, &mu)
	})

	return httptest.NewServer(mux)
}

func handleAuth(t *testing.T, writer http.ResponseWriter, request *http.Request, handlers snapshotRemoteHandlers) {
	assert.Equal(t, http.MethodPost, request.Method)
	writeJSONResponse(t, writer, handlers.authStatus, handlers.authBody)
}

func handleSnapshotCheck(t *testing.T, writer http.ResponseWriter, request *http.Request, handlers snapshotRemoteHandlers, index *int, mu *sync.Mutex) {
	assert.Equal(t, http.MethodGet, request.Method)

	mu.Lock()
	defer mu.Unlock()

	if len(handlers.snapshotChecks) > 0 {
		step := handlers.snapshotChecks[min(*index, len(handlers.snapshotChecks)-1)]
		*index++
		writeJSONResponse(t, writer, step.status, step.body)
		return
	}

	if handlers.snapshotCheckRawBody != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(handlers.snapshotCheckStatus)
		_, err := writer.Write([]byte(*handlers.snapshotCheckRawBody))
		assert.NoError(t, err)
		return
	}

	writeJSONResponse(t, writer, handlers.snapshotCheckStatus, handlers.snapshotCheckBody)
}

func handleGraphQL(t *testing.T, writer http.ResponseWriter, request *http.Request, handlers snapshotRemoteHandlers, index *int, mu *sync.Mutex) {
	assert.Equal(t, http.MethodPost, request.Method)

	mu.Lock()
	defer mu.Unlock()

	if handlers.resolveUnavailable {
		handleUnavailable(t, writer)
		return
	}

	if handlers.resolveRawBody != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(handlers.resolveStatus)
		_, err := writer.Write([]byte(*handlers.resolveRawBody))
		assert.NoError(t, err)
		return
	}

	if len(handlers.resolveSteps) > 0 {
		step := handlers.resolveSteps[min(*index, len(handlers.resolveSteps)-1)]
		*index++
		writeJSONResponse(t, writer, step.status, map[string]any{"data": map[string]any{"domain": step.domain}})
		return
	}

	writeJSONResponse(t, writer, handlers.resolveStatus, map[string]any{"data": map[string]any{"domain": handlers.resolveDomain}})
}

func handleUnavailable(t *testing.T, writer http.ResponseWriter) {
	hijacker, ok := writer.(http.Hijacker)
	assert.True(t, ok)
	if !ok {
		return
	}

	conn, _, err := hijacker.Hijack()
	assert.NoError(t, err)
	if err != nil {
		return
	}

	_ = conn.Close()
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}
