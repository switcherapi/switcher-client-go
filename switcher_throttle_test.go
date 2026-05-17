package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSwitcherThrottle(t *testing.T) {
	t.Run("should reuse cached results during the throttle window", func(t *testing.T) {
		server, criteriaRequests := newThrottleTestServer(t, []map[string]any{
			{"result": true},
		})
		defer server.Close()

		client := newThrottleTestClient(server.URL, ContextOptions{
			ThrottleMaxWorkers: 1,
		})
		switcher := client.GetSwitcher("MY_SWITCHER").Throttle(50 * time.Millisecond)

		first, firstErr := switcher.IsOnWithDetails()
		second, secondErr := switcher.IsOnWithDetails()

		assert.NoError(t, firstErr)
		assert.NoError(t, secondErr)
		assert.True(t, first.Result)
		assert.Equal(t, map[string]any{}, first.Metadata)
		assert.True(t, second.Result)
		assert.Equal(t, map[string]any{"cached": true}, second.Metadata)
		assert.Equal(t, int32(1), criteriaRequests.Load())
	})

	t.Run("should disable throttle when the period is zero", func(t *testing.T) {
		server, criteriaRequests := newThrottleTestServer(t, []map[string]any{
			{"result": true},
			{"result": false},
		})
		defer server.Close()

		client := newThrottleTestClient(server.URL, ContextOptions{})
		switcher := client.GetSwitcher("MY_SWITCHER").Throttle(0)

		first, firstErr := switcher.IsOnWithDetails()
		second, secondErr := switcher.IsOnWithDetails()

		assert.NoError(t, firstErr)
		assert.NoError(t, secondErr)
		assert.True(t, first.Result)
		assert.Equal(t, map[string]any{}, first.Metadata)
		assert.False(t, second.Result)
		assert.Equal(t, map[string]any{}, second.Metadata)
		assert.Equal(t, int32(2), criteriaRequests.Load())
		assert.Equal(t, ExecutionEntry{}, client.GetExecution(client.GetSwitcher("MY_SWITCHER")))
	})

	t.Run("should not reset the refresh window when throttle is applied again to the cached switcher", func(t *testing.T) {
		server, criteriaRequests := newThrottleTestServer(t, []map[string]any{
			{"result": true},
			{"result": false},
		})
		defer server.Close()

		client := newThrottleTestClient(server.URL, ContextOptions{
			ThrottleMaxWorkers: 1,
		})

		first, firstErr := client.GetSwitcher("MY_SWITCHER").Throttle(30 * time.Millisecond).IsOnWithDetails()
		assert.NoError(t, firstErr)
		assert.True(t, first.Result)

		time.Sleep(40 * time.Millisecond)

		cached, cachedErr := client.GetSwitcher("MY_SWITCHER").Throttle(30 * time.Millisecond).IsOnWithDetails()
		assert.NoError(t, cachedErr)
		assert.True(t, cached.Result)
		assert.Equal(t, map[string]any{"cached": true}, cached.Metadata)

		assert.Eventually(t, func() bool {
			return criteriaRequests.Load() == 2
		}, time.Second, 10*time.Millisecond)

		assert.Eventually(t, func() bool {
			logged := client.GetExecution(client.GetSwitcher("MY_SWITCHER"))
			return logged.Key == "MY_SWITCHER" && !logged.Response.Result
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("should refresh cached results in the background after the throttle expires", func(t *testing.T) {
		server, criteriaRequests := newThrottleTestServer(t, []map[string]any{
			{"result": true},
			{"result": false},
		})
		defer server.Close()

		client := newThrottleTestClient(server.URL, ContextOptions{
			ThrottleMaxWorkers: 1,
		})
		switcher := client.GetSwitcher("MY_SWITCHER").Throttle(30 * time.Millisecond)

		first, firstErr := switcher.IsOnWithDetails()
		assert.NoError(t, firstErr)
		assert.True(t, first.Result)

		time.Sleep(40 * time.Millisecond)

		cached, cachedErr := switcher.IsOnWithDetails()
		assert.NoError(t, cachedErr)
		assert.True(t, cached.Result)
		assert.Equal(t, map[string]any{"cached": true}, cached.Metadata)

		assert.Eventually(t, func() bool {
			return criteriaRequests.Load() == 2
		}, time.Second, 10*time.Millisecond)

		assert.Eventually(t, func() bool {
			logged := client.GetExecution(client.GetSwitcher("MY_SWITCHER"))
			return logged.Key == "MY_SWITCHER" && !logged.Response.Result
		}, time.Second, 10*time.Millisecond)

		refreshed, refreshedErr := switcher.IsOnWithDetails()
		assert.NoError(t, refreshedErr)
		assert.False(t, refreshed.Result)
		assert.Equal(t, map[string]any{"cached": true}, refreshed.Metadata)
	})

	t.Run("should notify subscribed errors when background refresh fails", func(t *testing.T) {
		var criteriaRequests atomic.Int32
		mux := http.NewServeMux()
		mux.HandleFunc("/criteria/auth", func(writer http.ResponseWriter, request *http.Request) {
			assert.Equal(t, http.MethodPost, request.Method)
			writeJSONResponse(t, writer, http.StatusOK, map[string]any{
				"token": "[token]",
				"exp":   time.Now().Add(time.Hour).Unix(),
			})
		})
		mux.HandleFunc("/criteria", func(writer http.ResponseWriter, request *http.Request) {
			assert.Equal(t, http.MethodPost, request.Method)

			var body map[string]any
			err := json.NewDecoder(request.Body).Decode(&body)
			assert.NoError(t, err)

			if criteriaRequests.Add(1) == 1 {
				writeJSONResponse(t, writer, http.StatusOK, map[string]any{"result": true})
				return
			}

			writeJSONResponse(t, writer, http.StatusInternalServerError, map[string]any{"error": "boom"})
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		client := newThrottleTestClient(server.URL, ContextOptions{
			ThrottleMaxWorkers: 1,
		})
		switcher := client.GetSwitcher("MY_SWITCHER").Throttle(30 * time.Millisecond)

		errCh := make(chan error, 1)
		client.SubscribeNotifyError(func(err error) {
			errCh <- err
		})

		first, firstErr := switcher.IsOnWithDetails()
		assert.NoError(t, firstErr)
		assert.True(t, first.Result)

		time.Sleep(40 * time.Millisecond)

		cached, cachedErr := switcher.IsOnWithDetails()
		assert.NoError(t, cachedErr)
		assert.True(t, cached.Result)
		assert.Equal(t, map[string]any{"cached": true}, cached.Metadata)

		select {
		case err := <-errCh:
			var remoteCriteriaErr *RemoteCriteriaError
			assert.ErrorAs(t, err, &remoteCriteriaErr)
			assert.EqualError(t, err, "[check_criteria] failed with status: 500")
		case <-time.After(time.Second):
			t.Fatal("expected background refresh error notification")
		}

		logged := client.GetExecution(client.GetSwitcher("MY_SWITCHER"))
		assert.Equal(t, "MY_SWITCHER", logged.Key)
		assert.True(t, logged.Response.Result)
		assert.Equal(t, map[string]any{"cached": true}, logged.Response.Metadata)
	})

	t.Run("should refresh cached results in the background without a worker limit", func(t *testing.T) {
		server, criteriaRequests := newThrottleTestServer(t, []map[string]any{
			{"result": true},
			{"result": false},
		})
		defer server.Close()

		client := newThrottleTestClient(server.URL, ContextOptions{})
		switcher := client.GetSwitcher("MY_SWITCHER").Throttle(30 * time.Millisecond)

		first, firstErr := switcher.IsOnWithDetails()
		assert.NoError(t, firstErr)
		assert.True(t, first.Result)

		time.Sleep(40 * time.Millisecond)

		cached, cachedErr := switcher.IsOnWithDetails()
		assert.NoError(t, cachedErr)
		assert.True(t, cached.Result)
		assert.Equal(t, map[string]any{"cached": true}, cached.Metadata)

		assert.Eventually(t, func() bool {
			return criteriaRequests.Load() == 2
		}, time.Second, 10*time.Millisecond)

		assert.Eventually(t, func() bool {
			logged := client.GetExecution(client.GetSwitcher("MY_SWITCHER"))
			return logged.Key == "MY_SWITCHER" && !logged.Response.Result
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("should keep serving the frozen cached result until the logger is cleared", func(t *testing.T) {
		server, criteriaRequests := newThrottleTestServer(t, []map[string]any{
			{"result": true},
			{"result": false},
		})
		defer server.Close()

		client := newThrottleTestClient(server.URL, ContextOptions{
			Freeze:             true,
			ThrottleMaxWorkers: 1,
		})
		switcher := client.GetSwitcher("MY_SWITCHER").Throttle(30 * time.Millisecond)

		first, firstErr := switcher.IsOnWithDetails()
		assert.NoError(t, firstErr)
		assert.True(t, first.Result)

		time.Sleep(40 * time.Millisecond)

		frozen, frozenErr := switcher.IsOnWithDetails()
		assert.NoError(t, frozenErr)
		assert.True(t, frozen.Result)
		assert.Equal(t, map[string]any{"cached": true}, frozen.Metadata)
		assert.Equal(t, int32(1), criteriaRequests.Load())

		client.ClearLogger()

		refreshed, refreshedErr := switcher.IsOnWithDetails()
		assert.NoError(t, refreshedErr)
		assert.False(t, refreshed.Result)
		assert.Equal(t, map[string]any{}, refreshed.Metadata)
		assert.Equal(t, int32(2), criteriaRequests.Load())
	})

	t.Run("should use cached results for throttle even when logger is disabled", func(t *testing.T) {
		server, criteriaRequests := newThrottleTestServer(t, []map[string]any{
			{"result": true},
		})
		defer server.Close()

		client := newThrottleTestClient(server.URL, ContextOptions{
			Logger:             false,
			ThrottleMaxWorkers: 1,
		})
		switcher := client.GetSwitcher("MY_SWITCHER").Throttle(50 * time.Millisecond)

		_, firstErr := switcher.IsOnWithDetails()
		second, secondErr := switcher.IsOnWithDetails()

		assert.NoError(t, firstErr)
		assert.NoError(t, secondErr)
		assert.True(t, second.Result)
		assert.Equal(t, map[string]any{"cached": true}, second.Metadata)
		assert.Equal(t, int32(1), criteriaRequests.Load())

		logged := client.GetExecution(client.GetSwitcher("MY_SWITCHER"))
		assert.Equal(t, "MY_SWITCHER", logged.Key)
		assert.True(t, logged.Response.Result)
		assert.Equal(t, map[string]any{"cached": true}, logged.Response.Metadata)
	})
}

func newThrottleTestClient(serverURL string, options ContextOptions) *Client {
	return NewClient(Context{
		Domain:    "My Domain",
		URL:       serverURL,
		APIKey:    "[YOUR_API_KEY]",
		Component: "MyApp",
		Options:   options,
	})
}

func newThrottleTestServer(t *testing.T, criteriaResponses []map[string]any) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var criteriaRequests atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/criteria/auth", func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		writeJSONResponse(t, writer, http.StatusOK, map[string]any{
			"token": "[token]",
			"exp":   time.Now().Add(time.Hour).Unix(),
		})
	})
	mux.HandleFunc("/criteria", func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)

		var body map[string]any
		err := json.NewDecoder(request.Body).Decode(&body)
		assert.NoError(t, err)

		index := int(criteriaRequests.Add(1)) - 1
		if index >= len(criteriaResponses) {
			index = len(criteriaResponses) - 1
		}

		writeJSONResponse(t, writer, http.StatusOK, criteriaResponses[index])
	})

	return httptest.NewServer(mux), &criteriaRequests
}
