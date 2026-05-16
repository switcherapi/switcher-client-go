package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSilentMode(t *testing.T) {
	t.Run("should fall back to the local snapshot and notify the remote failure", func(t *testing.T) {
		server := newSilentModeTestServer(t, silentModeServerOptions{
			authStatuses:     []int{http.StatusOK},
			criteriaStatuses: []int{http.StatusTooManyRequests},
			criteriaBodies:   []map[string]any{{"error": "Too many requests"}},
			healthStatuses:   []int{http.StatusInternalServerError},
			snapshotDomain:   loadSnapshotFixture(t, "default"),
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				SilentMode: time.Second,
			},
		})

		version, loadErr := client.LoadSnapshot(&LoadSnapshotOptions{FetchRemote: true})
		require.NoError(t, loadErr)
		require.Equal(t, 1, version)

		var asyncError string
		client.SubscribeNotifyError(func(err error) {
			asyncError = err.Error()
		})

		switcher := client.GetSwitcher("FF2FOR2022")

		first, firstErr := switcher.IsOn()
		require.NoError(t, firstErr)
		assert.True(t, first)
		assert.Equal(t, "[check_criteria] failed with status: 429", asyncError)
		assert.Equal(t, 1, server.criteriaRequests())
		assert.Equal(t, 0, server.healthRequests())

		asyncError = ""
		second, secondErr := switcher.IsOn()
		require.NoError(t, secondErr)
		assert.True(t, second)
		assert.Empty(t, asyncError)
		assert.Equal(t, 1, server.criteriaRequests())
		assert.Equal(t, 0, server.healthRequests())

		time.Sleep(1100 * time.Millisecond)

		third, thirdErr := switcher.IsOn()
		require.NoError(t, thirdErr)
		assert.True(t, third)
		assert.Empty(t, asyncError)
		assert.Equal(t, 1, server.criteriaRequests())
		assert.Equal(t, 1, server.healthRequests())
	})

	t.Run("should restore remote execution after the upstream health check succeeds", func(t *testing.T) {
		server := newSilentModeTestServer(t, silentModeServerOptions{
			authStatuses:     []int{http.StatusOK, http.StatusOK},
			criteriaStatuses: []int{http.StatusTooManyRequests, http.StatusOK},
			criteriaBodies: []map[string]any{
				{"error": "Too many requests"},
				{"result": false, "reason": "remote"},
			},
			healthStatuses: []int{http.StatusOK},
			snapshotDomain: loadSnapshotFixture(t, "default"),
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				SilentMode: time.Second,
			},
		})

		_, loadErr := client.LoadSnapshot(&LoadSnapshotOptions{FetchRemote: true})
		require.NoError(t, loadErr)

		var asyncError string
		client.SubscribeNotifyError(func(err error) {
			asyncError = err.Error()
		})

		switcher := client.GetSwitcher("FF2FOR2022")

		first, firstErr := switcher.IsOn()
		require.NoError(t, firstErr)
		assert.True(t, first)
		assert.Equal(t, "[check_criteria] failed with status: 429", asyncError)

		time.Sleep(1100 * time.Millisecond)
		asyncError = ""

		second, secondErr := switcher.IsOn()
		require.NoError(t, secondErr)
		assert.False(t, second)
		assert.Empty(t, asyncError)
		assert.Equal(t, 2, server.authRequests())
		assert.Equal(t, 2, server.criteriaRequests())
		assert.Equal(t, 1, server.healthRequests())
	})

	t.Run("should keep using the local snapshot when the health check request fails", func(t *testing.T) {
		server := newSilentModeTestServer(t, silentModeServerOptions{
			authStatuses:     []int{http.StatusOK},
			criteriaStatuses: []int{http.StatusTooManyRequests},
			criteriaBodies:   []map[string]any{{"error": "Too many requests"}},
			snapshotDomain:   loadSnapshotFixture(t, "default"),
		})

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				SilentMode: time.Second,
			},
		})

		_, loadErr := client.LoadSnapshot(&LoadSnapshotOptions{FetchRemote: true})
		require.NoError(t, loadErr)

		var asyncError string
		client.SubscribeNotifyError(func(err error) {
			asyncError = err.Error()
		})

		switcher := client.GetSwitcher("FF2FOR2022")

		first, firstErr := switcher.IsOn()
		require.NoError(t, firstErr)
		assert.True(t, first)
		assert.Equal(t, "[check_criteria] failed with status: 429", asyncError)

		time.Sleep(1100 * time.Millisecond)
		asyncError = ""
		server.Close()

		second, secondErr := switcher.IsOn()
		require.NoError(t, secondErr)
		assert.True(t, second)
		assert.Empty(t, asyncError)
		assert.Equal(t, 1, server.criteriaRequests())
		assert.Equal(t, 1, server.authRequests())
		assert.Equal(t, 0, server.healthRequests())
	})

	t.Run("should fall back to the local snapshot when authentication fails during silent mode", func(t *testing.T) {
		server := newSilentModeTestServer(t, silentModeServerOptions{
			authStatuses:     []int{http.StatusOK, http.StatusUnauthorized},
			criteriaStatuses: []int{http.StatusOK},
			criteriaBodies:   []map[string]any{{"result": true}},
			snapshotDomain:   loadSnapshotFixture(t, "default"),
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				SilentMode: time.Second,
			},
		})

		_, loadErr := client.LoadSnapshot(&LoadSnapshotOptions{FetchRemote: true})
		require.NoError(t, loadErr)

		client.authMu.Lock()
		client.authToken = ""
		client.authTokenExp = 0
		client.authMu.Unlock()

		var asyncError string
		client.SubscribeNotifyError(func(err error) {
			asyncError = err.Error()
		})

		enabled, err := client.GetSwitcher("FF2FOR2022").IsOn()

		require.NoError(t, err)
		assert.True(t, enabled)
		assert.Equal(t, "invalid API key", asyncError)
		assert.Equal(t, 2, server.authRequests())
		assert.Equal(t, 0, server.healthRequests())
	})

	t.Run("should surface the local snapshot error when silent mode has no snapshot to use", func(t *testing.T) {
		server := newSilentModeTestServer(t, silentModeServerOptions{
			authStatuses:     []int{http.StatusOK},
			criteriaStatuses: []int{http.StatusTooManyRequests},
			criteriaBodies:   []map[string]any{{"error": "Too many requests"}},
			snapshotDomain:   loadSnapshotFixture(t, "default"),
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				SilentMode: time.Second,
			},
		})

		var asyncError string
		client.SubscribeNotifyError(func(err error) {
			asyncError = err.Error()
		})

		enabled, err := client.GetSwitcher("FF2FOR2022").IsOn()

		assert.False(t, enabled)
		assert.EqualError(t, err, "Snapshot not loaded. Try to use 'Client.load_snapshot()'")
		assert.Equal(t, "[check_criteria] failed with status: 429", asyncError)
	})
}

type silentModeServerOptions struct {
	authStatuses     []int
	criteriaStatuses []int
	criteriaBodies   []map[string]any
	healthStatuses   []int
	snapshotDomain   map[string]any
}

type silentModeTestServer struct {
	*httptest.Server
	mu            sync.Mutex
	authCount     int
	criteriaCount int
	healthCount   int
}

func newSilentModeTestServer(t *testing.T, options silentModeServerOptions) *silentModeTestServer {
	t.Helper()

	server := &silentModeTestServer{}

	var authIndex int
	var criteriaIndex int
	var healthIndex int
	mux := http.NewServeMux()
	mux.HandleFunc("/criteria/auth", func(writer http.ResponseWriter, request *http.Request) {
		server.mu.Lock()
		server.authCount++
		status := selectStatus(options.authStatuses, authIndex, http.StatusOK)
		authIndex++
		server.mu.Unlock()

		assert.Equal(t, http.MethodPost, request.Method)

		if status != http.StatusOK {
			writeJSONResponse(t, writer, status, map[string]any{})
			return
		}

		writeJSONResponse(t, writer, status, map[string]any{
			"token": "[token]",
			"exp":   time.Now().Add(time.Hour).Unix(),
		})
	})
	mux.HandleFunc("/criteria/snapshot_check/", func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodGet, request.Method)
		writeJSONResponse(t, writer, http.StatusOK, map[string]any{"status": false})
	})
	mux.HandleFunc("/graphql", func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		writeJSONResponse(t, writer, http.StatusOK, map[string]any{
			"data": map[string]any{
				"domain": options.snapshotDomain,
			},
		})
	})
	mux.HandleFunc("/criteria", func(writer http.ResponseWriter, request *http.Request) {
		server.mu.Lock()
		server.criteriaCount++
		status := selectStatus(options.criteriaStatuses, criteriaIndex, http.StatusOK)
		body := selectBody(options.criteriaBodies, criteriaIndex, map[string]any{"result": true})
		criteriaIndex++
		server.mu.Unlock()

		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "FF2FOR2022", request.URL.Query().Get("key"))
		assert.Equal(t, "false", request.URL.Query().Get("showReason"))
		writeJSONResponse(t, writer, status, body)
	})
	mux.HandleFunc("/check", func(writer http.ResponseWriter, request *http.Request) {
		server.mu.Lock()
		server.healthCount++
		status := selectStatus(options.healthStatuses, healthIndex, http.StatusOK)
		healthIndex++
		server.mu.Unlock()

		assert.Equal(t, http.MethodGet, request.Method)
		writer.WriteHeader(status)
	})

	server.Server = httptest.NewServer(mux)
	return server
}

func (s *silentModeTestServer) authRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.authCount
}

func (s *silentModeTestServer) criteriaRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.criteriaCount
}

func (s *silentModeTestServer) healthRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.healthCount
}

func selectStatus(statuses []int, index int, fallback int) int {
	if len(statuses) == 0 {
		return fallback
	}

	return statuses[min(index, len(statuses)-1)]
}

func selectBody(bodies []map[string]any, index int, fallback map[string]any) map[string]any {
	if len(bodies) == 0 {
		return fallback
	}

	return bodies[min(index, len(bodies)-1)]
}

func TestSubscribeNotifyError(t *testing.T) {
	t.Run("should register the callback on the default client", func(t *testing.T) {
		BuildContext(Context{Domain: "My Domain"})

		var got error
		SubscribeNotifyError(func(err error) {
			got = err
		})

		defaultClient().notifyError(fmt.Errorf("boom"))

		require.EqualError(t, got, "boom")
	})
}
