package client

import (
	"encoding/json"
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
