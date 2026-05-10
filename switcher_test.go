package client

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwitcherValidate(t *testing.T) {
	t.Run("should return an error when remote fields are missing", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
		})

		err := GetSwitcher("").Validate()
		assert.EqualError(t, err, "something went wrong: missing or empty required fields (url, component, api_key)")
	})

	t.Run("should return an error when the key is missing", func(t *testing.T) {
		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       "https://api.switcherapi.com",
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		err := client.GetSwitcher("").Validate()
		assert.EqualError(t, err, "something went wrong: missing key field")
	})

	t.Run("should succeed when remote context and key are valid", func(t *testing.T) {
		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       "https://api.switcherapi.com",
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		err := client.GetSwitcher("FEATURE_LOGIN_V2").Validate()
		assert.NoError(t, err)
	})
}

func TestSwitcherPrepare(t *testing.T) {
	t.Run("should return a validation error when the switcher is invalid", func(t *testing.T) {
		client := NewClient(Context{
			Domain: "My Domain",
		})

		err := client.GetSwitcher("").Prepare("")

		assert.EqualError(t, err, "something went wrong: missing or empty required fields (url, component, api_key)")
	})

	t.Run("should return the auth error when token preparation fails", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
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

		err := client.GetSwitcher("MY_SWITCHER").Prepare("")

		assert.Error(t, err)
		var remoteAuthErr *RemoteAuthError
		assert.ErrorAs(t, err, &remoteAuthErr)
		assert.EqualError(t, err, "invalid API key")
	})
}

func TestSwitcherIsOnWithDetails(t *testing.T) {
	t.Run("should return a validation error when the switcher is invalid", func(t *testing.T) {
		client := NewClient(Context{
			Domain: "My Domain",
		})

		got, err := client.GetSwitcher("").IsOnWithDetails()

		assert.Equal(t, ResultDetail{}, got)
		assert.EqualError(t, err, "something went wrong: missing or empty required fields (url, component, api_key)")
	})
}

func TestAppendFilteredEntries(t *testing.T) {
	t.Run("should preserve entries whose strategy does not match the filtered strategy", func(t *testing.T) {
		entries := []criteriaEntry{
			{Strategy: StrategyValue, Input: "user_id"},
			{Strategy: "NETWORK_VALIDATION", Input: "127.0.0.1"},
		}

		got := appendFilteredEntries(entries, StrategyValue)

		assert.Equal(t, []criteriaEntry{
			{Strategy: "NETWORK_VALIDATION", Input: "127.0.0.1"},
		}, got)
	})
}
