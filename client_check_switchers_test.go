package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckSwitchers(t *testing.T) {
	t.Run("should validate switchers through the package-level helper", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		err := CheckSwitchers([]string{"FF2FOR2022", "FF2FOR2040"})

		assert.NoError(t, err)
	})

	t.Run("should validate switchers through the remote API", func(t *testing.T) {
		var captured map[string]any
		server := newCheckSwitchersTestServer(t, checkSwitchersTestHandlers{
			authStatus:      http.StatusOK,
			authBody:        map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			switchersStatus: http.StatusOK,
			switchersBody:   map[string]any{"not_found": []string{}},
			onSwitchersRequest: func(body map[string]any, _ *http.Request) {
				captured = body
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		err := client.CheckSwitchers([]string{"MY_SWITCHER", "ANOTHER_SWITCHER"})

		assert.NoError(t, err)
		assert.Equal(t, map[string]any{
			"switchers": []any{"MY_SWITCHER", "ANOTHER_SWITCHER"},
		}, captured)
	})

	t.Run("should return a remote switcher error when the remote API reports missing keys", func(t *testing.T) {
		server := newCheckSwitchersTestServer(t, checkSwitchersTestHandlers{
			authStatus:      http.StatusOK,
			authBody:        map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			switchersStatus: http.StatusOK,
			switchersBody:   map[string]any{"not_found": []string{"MY_SWITCHER"}},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		err := client.CheckSwitchers([]string{"MY_SWITCHER", "ANOTHER_SWITCHER"})

		assert.Error(t, err)
		var remoteSwitcherErr *RemoteSwitcherError
		assert.ErrorAs(t, err, &remoteSwitcherErr)
		assert.EqualError(t, err, "MY_SWITCHER not found")
	})

	t.Run("should return a remote auth error when authentication fails", func(t *testing.T) {
		server := newCheckSwitchersTestServer(t, checkSwitchersTestHandlers{
			authStatus: http.StatusUnauthorized,
			authBody:   map[string]any{},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		err := client.CheckSwitchers([]string{"MY_SWITCHER"})

		assert.Error(t, err)
		var remoteAuthErr *RemoteAuthError
		assert.ErrorAs(t, err, &remoteAuthErr)
		assert.EqualError(t, err, "invalid API key")
	})

	t.Run("should return a remote error when the switchers endpoint fails", func(t *testing.T) {
		server := newCheckSwitchersTestServer(t, checkSwitchersTestHandlers{
			authStatus:      http.StatusOK,
			authBody:        map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			switchersStatus: http.StatusInternalServerError,
			switchersBody:   map[string]any{"not_found": []string{}},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		err := client.CheckSwitchers([]string{"MY_SWITCHER"})

		assert.Error(t, err)
		var remoteErr *RemoteError
		assert.ErrorAs(t, err, &remoteErr)
		assert.EqualError(t, err, "[check_switchers] failed with status: 500")
	})

	t.Run("should return an error when the auth response token is missing", func(t *testing.T) {
		server := newCheckSwitchersTestServer(t, checkSwitchersTestHandlers{
			authStatus: http.StatusOK,
			authBody:   map[string]any{"exp": time.Now().Add(time.Hour).Unix()},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		err := client.CheckSwitchers([]string{"MY_SWITCHER"})

		assert.EqualError(t, err, "something went wrong: missing token field")
	})

	t.Run("should return an error when check switchers endpoint is unavailable", func(t *testing.T) {
		server := newCheckSwitchersTestServer(t, checkSwitchersTestHandlers{
			authStatus: http.StatusOK,
			authBody:   map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		err := client.CheckSwitchers([]string{"MY_SWITCHER"})

		assert.Error(t, err)
		var remoteErr *RemoteError
		assert.ErrorAs(t, err, &remoteErr)
		assert.EqualError(t, err, "[check_switchers] remote unavailable")
	})

	t.Run("should return a local switcher error when keys are missing from the snapshot", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		err := CheckSwitchers([]string{"FF2FOR2022", "NON_EXISTENT_SWITCHER"})

		assert.Error(t, err)
		var localSwitcherErr *LocalSwitcherError
		assert.ErrorAs(t, err, &localSwitcherErr)
		assert.EqualError(t, err, "NON_EXISTENT_SWITCHER not found")
	})

	t.Run("should return an error when check switchers response cannot be decoded", func(t *testing.T) {
		rawBody := "{invalid-json"
		server := newCheckSwitchersTestServer(t, checkSwitchersTestHandlers{
			authStatus:       http.StatusOK,
			authBody:         map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			switchersStatus:  http.StatusOK,
			switchersRawBody: &rawBody,
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		err := client.CheckSwitchers([]string{"MY_SWITCHER"})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("should return all requested keys when no local snapshot is loaded", func(t *testing.T) {
		client := NewClient(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: t.TempDir(),
			},
		})

		err := client.CheckSwitchers([]string{"FF2FOR2022", "NON_EXISTENT_SWITCHER"})

		assert.Error(t, err)
		var localSwitcherErr *LocalSwitcherError
		assert.ErrorAs(t, err, &localSwitcherErr)
		assert.EqualError(t, err, "FF2FOR2022, NON_EXISTENT_SWITCHER not found")
	})
}

type checkSwitchersTestHandlers struct {
	authStatus         int
	authBody           map[string]any
	authRawBody        *string
	switchersStatus    int
	switchersBody      map[string]any
	switchersRawBody   *string
	onAuthRequest      func(*http.Request)
	onSwitchersRequest func(body map[string]any, request *http.Request)
}

func newCheckSwitchersTestServer(t *testing.T, handlers checkSwitchersTestHandlers) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/criteria/auth", func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		if handlers.onAuthRequest != nil {
			handlers.onAuthRequest(request)
		}
		if handlers.authRawBody != nil {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(handlers.authStatus)
			_, err := writer.Write([]byte(*handlers.authRawBody))
			assert.NoError(t, err)
			return
		}

		writeJSONResponse(t, writer, handlers.authStatus, handlers.authBody)
	})
	mux.HandleFunc("/criteria/switchers_check", func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)

		var body map[string]any
		err := json.NewDecoder(request.Body).Decode(&body)
		assert.NoError(t, err)

		if handlers.onSwitchersRequest != nil {
			handlers.onSwitchersRequest(body, request)
		}

		if handlers.switchersRawBody != nil {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(handlers.switchersStatus)
			_, err := writer.Write([]byte(*handlers.switchersRawBody))
			assert.NoError(t, err)
			return
		}

		writeJSONResponse(t, writer, handlers.switchersStatus, handlers.switchersBody)
	})

	return httptest.NewServer(mux)
}
