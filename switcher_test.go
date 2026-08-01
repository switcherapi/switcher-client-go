package client

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("should return a validation error when the switcher is invalid", func(t *testing.T) {
		client := NewClient(Context{
			Domain: "My Domain",
		})

		got, err := client.GetSwitcher("").IsOn()
		assert.Equal(t, false, got)
		assert.EqualError(t, err, "something went wrong: missing or empty required fields (url, component, api_key)")

		gotD, errD := client.GetSwitcher("").IsOnWithDetails()
		assert.Equal(t, ResultDetail{}, gotD)
		assert.EqualError(t, errD, "something went wrong: missing or empty required fields (url, component, api_key)")
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

func TestSwitcherMustOrDefault(t *testing.T) {
	t.Run("should return the default value when the switcher is invalid", func(t *testing.T) {
		client := NewClient(Context{Domain: "My Domain"})
		var errs []error
		client.SubscribeNotifyError(func(err error) {
			errs = append(errs, err)
		})

		defBool := true
		got := client.GetSwitcher("").IsOnOrDefault(defBool)
		assert.Equal(t, defBool, got)
		if assert.Len(t, errs, 1) {
			assert.EqualError(t, errs[0], "something went wrong: missing or empty required fields (url, component, api_key)")
		}

		defs := ResultDetail{Result: true, Reason: "default"}
		gotD := client.GetSwitcher("").IsOnWithDetailsOrDefault(defs)
		assert.Equal(t, defs, gotD)
		if assert.Len(t, errs, 2) {
			assert.EqualError(t, errs[1], "something went wrong: missing or empty required fields (url, component, api_key)")
		}
	})

	t.Run("should return without error when the switcher is valid", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true, "reason": "Success", "metadata": map[string]any{"env": "prod"}},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		got := client.GetSwitcher("MY_SWITCHER").IsOnOrDefault(false)
		assert.Equal(t, true, got)

		gotD := client.GetSwitcher("MY_SWITCHER").IsOnWithDetailsOrDefault(ResultDetail{Result: false, Reason: "default"})
		assert.Equal(t, ResultDetail{Result: true, Reason: "Success", Metadata: map[string]any{"env": "prod"}}, gotD)
	})
}

func TestSwitcherRemoteHybrid(t *testing.T) {
	t.Run("should call the remote API even when local mode is enabled and a snapshot is loaded", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			// Local snapshot has FF2FOR2022 activated (true) with no strategies; the remote
			// mock returns false so we can prove the remote path (not local) was used.
			criteriaBody: map[string]any{"result": false, "reason": "remote"},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotFixtureDir(),
			},
		})

		_, loadErr := client.LoadSnapshot(nil)
		require.NoError(t, loadErr)

		// sanity check: plain local evaluation returns true and never hits the server.
		localResult, localErr := client.GetSwitcher("FF2FOR2022").IsOn()
		require.NoError(t, localErr)
		assert.True(t, localResult)

		remoteResult, remoteErr := client.GetSwitcher("FF2FOR2022").Remote().IsOn()
		require.NoError(t, remoteErr)
		assert.False(t, remoteResult)
	})

	t.Run("should return an error from Validate/IsOn when local mode is not enabled", func(t *testing.T) {
		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       "https://api.switcherapi.com",
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
		})

		switcher := client.GetSwitcher("FEATURE_LOGIN_V2").Remote()

		err := switcher.Validate()
		assert.EqualError(t, err, "something went wrong: local mode is not enabled")

		_, isOnErr := switcher.IsOn()
		assert.EqualError(t, isOnErr, "something went wrong: local mode is not enabled")
	})

	t.Run("should behave as a no-op override when Remote(false) is used", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": false, "reason": "remote"},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotFixtureDir(),
			},
		})

		_, loadErr := client.LoadSnapshot(nil)
		require.NoError(t, loadErr)

		result, err := client.GetSwitcher("FF2FOR2022").Remote(false).IsOn()
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("should compose with Check*, Throttle and IsOnWithDetails chains", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true, "reason": "Success"},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotFixtureDir(),
			},
		})

		_, loadErr := client.LoadSnapshot(nil)
		require.NoError(t, loadErr)

		result, err := client.GetSwitcher("FF2FOR2022").
			Remote().
			CheckValue("USER_1").
			Throttle(time.Second).
			IsOnWithDetails()

		require.NoError(t, err)
		assert.Equal(t, ResultDetail{Result: true, Reason: "Success", Metadata: map[string]any{}}, result)
	})
}
