package client

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExecutionLogger(t *testing.T) {
	t.Run("should return an empty execution when the package API receives a nil switcher", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
		})

		logged := GetExecution(nil)

		assert.Equal(t, ExecutionEntry{}, logged)
	})

	t.Run("should log remote executions and retrieve them by switcher", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Logger: true,
			},
		})

		got, err := client.GetSwitcher("MY_SWITCHER").CheckValue("user_id").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)

		logged := client.GetExecution(client.GetSwitcher("MY_SWITCHER").CheckValue("user_id"))
		assert.Equal(t, "MY_SWITCHER", logged.Key)
		assert.Equal(t, []ExecutionInput{{Strategy: StrategyValue, Input: "user_id"}}, logged.Inputs)
		assert.True(t, logged.Response.Result)
		assert.Equal(t, map[string]any{"cached": true}, logged.Response.Metadata)
	})

	t.Run("should return empty execution when inputs do not match", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Logger: true,
			},
		})

		_, err := client.GetSwitcher("MY_SWITCHER").CheckValue("user_id").IsOn()
		assert.NoError(t, err)

		logged := client.GetExecution(client.GetSwitcher("MY_SWITCHER").CheckValue("other_id"))
		assert.Equal(t, ExecutionEntry{}, logged)
	})

	t.Run("should return empty execution when the logged entry has inputs and the lookup has none", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Logger: true,
			},
		})

		_, err := client.GetSwitcher("MY_SWITCHER").CheckValue("user_id").IsOn()
		assert.NoError(t, err)

		lookup := client.GetSwitcher("")
		err = lookup.Prepare("MY_SWITCHER")
		assert.NoError(t, err)

		logged := client.GetExecution(lookup)
		assert.Equal(t, ExecutionEntry{}, logged)
	})

	t.Run("should clear logged executions", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
		})
		defer server.Close()

		client := NewClient(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Logger: true,
			},
		})

		_, err := client.GetSwitcher("MY_SWITCHER").IsOn()
		assert.NoError(t, err)

		client.ClearLogger()

		logged := client.GetExecution(client.GetSwitcher("MY_SWITCHER"))
		assert.Equal(t, ExecutionEntry{}, logged)
	})

	t.Run("should clear logged executions through the package API", func(t *testing.T) {
		server := newRemoteTestServer(t, remoteTestHandlers{
			authStatus:     http.StatusOK,
			authBody:       map[string]any{"token": "[token]", "exp": time.Now().Add(time.Hour).Unix()},
			criteriaStatus: http.StatusOK,
			criteriaBody:   map[string]any{"result": true},
		})
		defer server.Close()

		BuildContext(Context{
			Domain:    "My Domain",
			URL:       server.URL,
			APIKey:    "[YOUR_API_KEY]",
			Component: "MyApp",
			Options: ContextOptions{
				Logger: true,
			},
		})

		_, err := GetSwitcher("MY_SWITCHER").IsOn()
		assert.NoError(t, err)

		ClearLogger()

		logged := GetExecution(GetSwitcher("MY_SWITCHER"))
		assert.Equal(t, ExecutionEntry{}, logged)
	})

	t.Run("should log local executions when logger is enabled", func(t *testing.T) {
		BuildContext(Context{
			Domain:      "My Domain",
			Environment: "default",
			Options: ContextOptions{
				Local:            true,
				Logger:           true,
				SnapshotLocation: filepath.Join("testdata", "snapshots"),
			},
		})

		_, err := LoadSnapshot(nil)
		assert.NoError(t, err)

		got, evalErr := GetSwitcher("FF2FOR2022").IsOn()

		assert.NoError(t, evalErr)
		assert.True(t, got)

		logged := GetExecution(GetSwitcher("FF2FOR2022"))
		assert.Equal(t, "FF2FOR2022", logged.Key)
		assert.True(t, logged.Response.Result)
		assert.Equal(t, map[string]any{"cached": true}, logged.Response.Metadata)
	})
}
