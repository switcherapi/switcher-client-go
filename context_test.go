package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildContext(t *testing.T) {
	t.Run("should preserve optional context options", func(t *testing.T) {
		client := NewClient(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: "./tests/snapshots",
			},
		})

		options := client.Context().Options

		assert.True(t, options.Local, "expected Local option to be true")
		assert.Equal(t, "./tests/snapshots", options.SnapshotLocation, "expected SnapshotLocation to be './tests/snapshots'")
	})

	t.Run("should create fresh default options on rebuild", func(t *testing.T) {
		BuildContext(Context{
			Domain: "First Domain",
			Options: ContextOptions{
				Local: true,
			},
		})
		firstClient := defaultClient()

		BuildContext(Context{Domain: "Second Domain"})
		secondClient := defaultClient()

		assert.NotSame(t, firstClient, secondClient, "expected different clients for different contexts")
		assert.True(t, firstClient.Context().Options.Local, "expected Local option to remain true for the first client")
		assert.False(t, secondClient.Context().Options.Local, "expected Local option to be false for the new client")
	})

	t.Run("should apply default values when omitted", func(t *testing.T) {
		client := NewClient(Context{
			Domain: "My Domain",
		})

		ctx := client.Context()

		assert.Equal(t, DefaultEnvironment, ctx.Environment)
		assert.True(t, ctx.Options.RestrictRelay)
		assert.Equal(t, DefaultRegexMaxBlacklist, ctx.Options.RegexMaxBlacklist)
		assert.Equal(t, DefaultRegexMaxTimeLimit, ctx.Options.RegexMaxTimeLimit)
		assert.Equal(t, DefaultRemoteConnectTimeout, ctx.Options.Remote.ConnectTimeout)
		assert.Equal(t, DefaultRemoteTimeout, ctx.Options.Remote.Timeout)
	})
}
