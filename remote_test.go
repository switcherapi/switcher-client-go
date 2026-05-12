package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSwitcherRemoteEvaluation(t *testing.T) {
	t.Run("should call the remote API with success", func(t *testing.T) {
		var captured map[string]any
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
			onCriteriaRequest: func(body map[string]any, _ *http.Request) {
				captured = body
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		got, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
		assert.Equal(t, map[string]any{
			"entry": []any{},
		}, captured)
	})

	t.Run("should send input parameters to the remote criteria endpoint", func(t *testing.T) {
		var captured map[string]any
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
			onCriteriaRequest: func(body map[string]any, request *http.Request) {
				captured = body
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		got, err := client.GetSwitcher("MY_SWITCHER").CheckValue("user_id").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
		assert.Equal(t, map[string]any{
			"entry": []any{
				map[string]any{
					"strategy": StrategyValue,
					"input":    "user_id",
				},
			},
		}, captured)
	})

	t.Run("should return response from the remote API without requesting details", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody: map[string]any{
				"result": true,
			},
			onCriteriaRequest: func(_ map[string]any, request *http.Request) {
				assert.Equal(t, "false", request.URL.Query().Get("showReason"))
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		got, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return detailed response from the remote API", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody: map[string]any{
				"result":   true,
				"reason":   "Success",
				"metadata": map[string]any{"key": "value"},
			},
			onCriteriaRequest: func(_ map[string]any, request *http.Request) {
				assert.Equal(t, "true", request.URL.Query().Get("showReason"))
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		got, err := client.GetSwitcher("MY_SWITCHER").IsOnWithDetails()

		assert.NoError(t, err)
		assert.True(t, got.Result)
		assert.Equal(t, "Success", got.Reason)
		assert.Equal(t, map[string]any{"key": "value"}, got.Metadata)
		assert.Equal(t, map[string]any{
			"result":   true,
			"reason":   "Success",
			"metadata": map[string]any{"key": "value"},
		}, got.ToMap())
	})

	t.Run("should request details only for the detailed call on the same switcher", func(t *testing.T) {
		showReasonValues := make([]string, 0, 2)
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true, "reason": "Success"},
			onCriteriaRequest: func(_ map[string]any, request *http.Request) {
				showReasonValues = append(showReasonValues, request.URL.Query().Get("showReason"))
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)
		switcher := client.GetSwitcher("MY_SWITCHER")

		_, detailErr := switcher.IsOnWithDetails()
		got, err := switcher.IsOn()

		assert.NoError(t, detailErr)
		assert.NoError(t, err)
		assert.True(t, got)
		assert.Equal(t, []string{"true", "false"}, showReasonValues)
	})

	t.Run("should authenticate during prepare and reuse the prepared key", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
			onCriteriaRequest: func(_ map[string]any, request *http.Request) {
				assert.Equal(t, "USER_FEATURE", request.URL.Query().Get("key"))
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)
		switcher := client.GetSwitcher("").CheckValue("user_id")

		err := switcher.Prepare("USER_FEATURE")
		got, evalErr := switcher.IsOn()

		assert.NoError(t, err)
		assert.NoError(t, evalErr)
		assert.True(t, got)
	})

	t.Run("should keep only the latest value check input", func(t *testing.T) {
		var captured map[string]any
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
			onCriteriaRequest: func(body map[string]any, _ *http.Request) {
				captured = body
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		got, err := client.GetSwitcher("MY_SWITCHER").CheckValue("first").CheckValue("second").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
		assert.Equal(t, map[string]any{
			"entry": []any{
				map[string]any{
					"strategy": StrategyValue,
					"input":    "second",
				},
			},
		}, captured)
	})

	t.Run("should reuse the token while a millisecond expiration is still valid", func(t *testing.T) {
		authRequests := 0
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).UnixMilli()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
			onAuthRequest: func(_ *http.Request) {
				authRequests++
			},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)
		switcher := client.GetSwitcher("MY_SWITCHER")

		first, firstErr := switcher.IsOn()
		second, secondErr := switcher.IsOn()

		assert.NoError(t, firstErr)
		assert.NoError(t, secondErr)
		assert.True(t, first)
		assert.True(t, second)
		assert.Equal(t, 1, authRequests)
	})

	t.Run("should renew the token when the cached expiration is zero", func(t *testing.T) {
		authRequests := 0
		mux := http.NewServeMux()
		mux.HandleFunc("/criteria/auth", func(writer http.ResponseWriter, request *http.Request) {
			authRequests++

			payload := map[string]any{"token": "[new_token]", "exp": time.Now().Add(time.Hour).Unix()}
			if authRequests == 1 {
				payload = map[string]any{"token": "[expired_token]", "exp": 0}
			}

			writeJSONResponse(t, writer, http.StatusOK, payload)
		})
		mux.HandleFunc("/criteria", func(writer http.ResponseWriter, request *http.Request) {
			writeJSONResponse(t, writer, http.StatusOK, map[string]any{"result": true})
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := newRemoteTestClient(server.URL)
		switcher := client.GetSwitcher("MY_SWITCHER")

		first, firstErr := switcher.IsOn()
		second, secondErr := switcher.IsOn()

		assert.NoError(t, firstErr)
		assert.NoError(t, secondErr)
		assert.True(t, first)
		assert.True(t, second)
		assert.Equal(t, 2, authRequests)
	})

	t.Run("should return an auth error when the API key is invalid", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus: http.StatusUnauthorized,
			authBody:   map[string]any{},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		_, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.Error(t, err)
		var remoteAuthErr *RemoteAuthError
		assert.ErrorAs(t, err, &remoteAuthErr)
		assert.EqualError(t, err, "invalid API key")
	})

	t.Run("should return an auth unavailable error when the auth endpoint cannot be reached", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
		})

		client := newRemoteTestClient(server.URL)
		server.Close()

		_, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.Error(t, err)
		var remoteAuthErr *RemoteAuthError
		assert.ErrorAs(t, err, &remoteAuthErr)
		assert.EqualError(t, err, "[auth] remote unavailable")
	})

	t.Run("should return a decode error when the auth response body is malformed", func(t *testing.T) {
		rawBody := "{invalid-json"
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:  http.StatusOK,
			authRawBody: &rawBody,
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		_, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("should return an error when the auth response token is missing", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus: http.StatusOK,
			authBody:   map[string]any{"token": nil, "exp": time.Now().Add(time.Hour).Unix()},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		_, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.EqualError(t, err, "something went wrong: missing token field")
	})

	t.Run("should return a remote criteria error when the criteria endpoint fails", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusInternalServerError,
			criteriaBody:   map[string]any{"error": "boom"},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		_, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.Error(t, err)
		var remoteCriteriaErr *RemoteCriteriaError
		assert.ErrorAs(t, err, &remoteCriteriaErr)
		assert.EqualError(t, err, "[check_criteria] failed with status: 500")
	})

	t.Run("should return a decode error when the criteria response body is malformed", func(t *testing.T) {
		rawBody := "{invalid-json"
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:      http.StatusOK,
			authBody:        map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus:  http.StatusOK,
			criteriaRawBody: &rawBody,
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		_, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("should return a remote unavailable error when the criteria endpoint cannot be reached", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
		})

		client := newRemoteTestClient(server.URL)
		switcher := client.GetSwitcher("MY_SWITCHER")

		err := switcher.Prepare("")
		server.Close()
		_, evalErr := switcher.IsOn()

		assert.NoError(t, err)
		assert.Error(t, evalErr)
		var remoteCriteriaErr *RemoteCriteriaError
		assert.ErrorAs(t, evalErr, &remoteCriteriaErr)
		assert.EqualError(t, evalErr, "[check_criteria] remote unavailable")
	})
}

type remoteTestHandlers struct {
	authStatus        int
	authBody          map[string]any
	authRawBody       *string
	criteriaStatus    int
	criteriaBody      map[string]any
	criteriaRawBody   *string
	onAuthRequest     func(request *http.Request)
	onCriteriaRequest func(body map[string]any, request *http.Request)
}

func newRemoteTestClient(serverURL string) *Client {
	return NewClient(Context{
		Domain:    "My Domain",
		URL:       serverURL,
		APIKey:    "[YOUR_API_KEY]",
		Component: "MyApp",
	})
}

func newRemoteTestServer(t *testing.T, handlers remoteTestHandlers) *httptest.Server {
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
	mux.HandleFunc("/criteria", func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)

		var body map[string]any
		err := json.NewDecoder(request.Body).Decode(&body)
		assert.NoError(t, err)

		if handlers.onCriteriaRequest != nil {
			handlers.onCriteriaRequest(body, request)
		}

		if handlers.criteriaRawBody != nil {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(handlers.criteriaStatus)
			_, writeErr := writer.Write([]byte(*handlers.criteriaRawBody))
			assert.NoError(t, writeErr)
			return
		}

		writeJSONResponse(t, writer, handlers.criteriaStatus, handlers.criteriaBody)
	})

	return httptest.NewServer(mux)
}

func writeJSONResponse(t *testing.T, writer http.ResponseWriter, status int, payload map[string]any) {
	t.Helper()

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	err := json.NewEncoder(writer).Encode(payload)
	assert.NoError(t, err)
}
