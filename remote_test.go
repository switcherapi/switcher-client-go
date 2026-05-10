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
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
		})
		defer server.Close()

		client := newRemoteTestClient(server.URL)

		got, err := client.GetSwitcher("MY_SWITCHER").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
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

func TestClientDoJSONRequest(t *testing.T) {
	t.Run("should return an error when the payload cannot be marshaled", func(t *testing.T) {
		client := newRemoteTestClient("https://api.switcherapi.com")

		response, err := client.doJSONRequest(
			http.MethodPost,
			"https://api.switcherapi.com/criteria/auth",
			map[string]any{
				"invalid": func() {},
			},
			map[string]string{
				"Content-Type": "application/json",
			},
		)

		assert.Nil(t, response)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported type")
	})

	t.Run("should return an error when the request cannot be created", func(t *testing.T) {
		client := newRemoteTestClient("https://api.switcherapi.com")

		response, err := client.doJSONRequest(
			http.MethodPost,
			"://bad-url",
			map[string]any{
				"domain": "My Domain",
			},
			map[string]string{
				"Content-Type": "application/json",
			},
		)

		assert.Nil(t, response)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing protocol scheme")
	})
}

func TestRequestTimeout(t *testing.T) {
	t.Run("should return the default combined timeout when configured timeout is zero", func(t *testing.T) {
		got := requestTimeout(RemoteOptions{})

		assert.Equal(
			t,
			DefaultRemoteConnectTimeout+DefaultRemoteReadTimeout+DefaultRemoteWriteTimeout,
			got,
		)
	})

	t.Run("should return the default combined timeout when configured timeout is negative", func(t *testing.T) {
		got := requestTimeout(RemoteOptions{
			ConnectTimeout: -time.Second,
		})

		assert.Equal(
			t,
			DefaultRemoteConnectTimeout+DefaultRemoteReadTimeout+DefaultRemoteWriteTimeout,
			got,
		)
	})
}

func TestParseTokenExpiration(t *testing.T) {
	t.Run("should parse the expiration from a json number", func(t *testing.T) {
		got := parseTokenExpiration(json.Number("1700000000"))

		assert.Equal(t, int64(1700000000), got)
	})

	t.Run("should return zero when the json number is invalid", func(t *testing.T) {
		got := parseTokenExpiration(json.Number("invalid"))

		assert.Zero(t, got)
	})
}

func TestTokenExpired(t *testing.T) {
	t.Run("should treat zero expiration as expired", func(t *testing.T) {
		assert.True(t, tokenExpired(0))
	})

	t.Run("should compare millisecond expirations against the current time", func(t *testing.T) {
		assert.False(t, tokenExpired(time.Now().Add(time.Minute).UnixMilli()))
		assert.True(t, tokenExpired(time.Now().Add(-time.Minute).UnixMilli()))
	})
}

func TestStrconvFormatBool(t *testing.T) {
	t.Run("should return false when the input is false", func(t *testing.T) {
		assert.Equal(t, "false", strconvFormatBool(false))
	})
}

type remoteTestHandlers struct {
	authStatus        int
	authBody          map[string]any
	authRawBody       *string
	criteriaStatus    int
	criteriaBody      map[string]any
	criteriaRawBody   *string
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
