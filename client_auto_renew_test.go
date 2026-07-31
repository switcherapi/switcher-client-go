package client

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// These are rare, focused white-box tests covering internal generation/race semantics
// of tokenAutoRenewer that aren't practically observable (deterministically) via the
// public API. Please check the public API tests at remote_test.go (TestSwitcherRemoteAutoRenewToken).

func TestAutoRenewDelay(t *testing.T) {
	t.Run("should treat exp as Unix seconds when at or below the millisecond threshold", func(t *testing.T) {
		exp := time.Now().Add(time.Hour).Unix()

		delay := autoRenewDelay(exp)

		expected := time.Until(time.Unix(exp, 0)) - autoRenewBuffer
		assert.InDelta(t, expected.Seconds(), delay.Seconds(), 1)
	})

	t.Run("should treat exp as Unix milliseconds when above the millisecond threshold", func(t *testing.T) {
		exp := time.Now().Add(time.Hour).UnixMilli()
		assert.Greater(t, exp, int64(1_000_000_000_000))

		delay := autoRenewDelay(exp)

		expected := time.Until(time.UnixMilli(exp)) - autoRenewBuffer
		assert.InDelta(t, expected.Seconds(), delay.Seconds(), 1)
	})

	t.Run("should floor the delay at autoRenewMinDelay when exp is imminent or in the past", func(t *testing.T) {
		exp := time.Now().Add(-time.Minute).Unix()

		delay := autoRenewDelay(exp)

		assert.Equal(t, autoRenewMinDelay, delay)
	})
}

func TestTokenAutoRenewerGenerationSemantics(t *testing.T) {
	t.Run("should skip auth for a stale generation renew callback", func(t *testing.T) {
		var authRequests atomic.Int32
		mux := http.NewServeMux()
		mux.HandleFunc("/criteria/auth", func(writer http.ResponseWriter, request *http.Request) {
			authRequests.Add(1)
			writeJSONResponse(t, writer, http.StatusOK, map[string]any{
				"token": "[new_token]",
				"exp":   time.Now().Add(time.Hour).Unix(),
			})
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})
		client.authToken = "[current_token]"
		client.authTokenExp = time.Now().Add(time.Hour).Unix()

		renewer := newTokenAutoRenewer()
		renewer.generation = 5
		staleGeneration := 4

		renewer.renew(client, staleGeneration)

		assert.Equal(t, int32(0), authRequests.Load(), "expected no auth request for a stale generation")
		assert.Equal(t, "[current_token]", client.authToken)
	})

	t.Run("should discard the renewal result when stop is called while the request is in flight", func(t *testing.T) {
		authStarted := make(chan struct{})
		releaseAuth := make(chan struct{})
		var authRequests atomic.Int32

		mux := http.NewServeMux()
		mux.HandleFunc("/criteria/auth", func(writer http.ResponseWriter, request *http.Request) {
			authRequests.Add(1)
			close(authStarted)
			select {
			case <-releaseAuth:
			case <-time.After(2 * time.Second):
			}
			writeJSONResponse(t, writer, http.StatusOK, map[string]any{
				"token": "[new_token]",
				"exp":   time.Now().Add(time.Hour).Unix(),
			})
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})
		client.authToken = "[current_token]"
		client.authTokenExp = time.Now().Add(time.Hour).Unix()

		renewer := newTokenAutoRenewer()
		currentGeneration := renewer.generation

		done := make(chan struct{})
		go func() {
			defer close(done)
			renewer.renew(client, currentGeneration)
		}()

		select {
		case <-authStarted:
		case <-time.After(1 * time.Second):
			t.Fatal("expected auth request to start")
		}

		renewer.stop()
		close(releaseAuth)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("expected renew goroutine to finish")
		}

		assert.Equal(t, int32(1), authRequests.Load())
		assert.Equal(t, "[current_token]", client.authToken, "expected the stale renewal result to be discarded")
		assert.Nil(t, renewer.timer, "expected no new renewal to be scheduled after stop")
	})
}
