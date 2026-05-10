package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildContext(t *testing.T) {
	t.Run("should preserve optional context options", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: "./tests/snapshots",
			},
		})

		options := defaultClient().context.Options

		assert.True(t, options.Local, "expected Local option to be true")
		assert.Equal(t, "./tests/snapshots", options.SnapshotLocation, "expected SnapshotLocation to be './tests/snapshots'")
	})

	t.Run("should create fresh default options on rebuild", func(t *testing.T) {
		BuildContext(Context{Domain: "First Domain"})
		firstClient := defaultClient()

		firstClient.mu.Lock()
		firstClient.context.Options.Local = true
		firstClient.mu.Unlock()

		BuildContext(Context{Domain: "Second Domain"})
		secondClient := defaultClient()

		assert.NotSame(t, firstClient, secondClient, "expected different clients for different contexts")
		assert.False(t, secondClient.context.Options.Local, "expected Local option to be false for the new client")
	})

	t.Run("should apply default values when omitted", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
		})

		ctx := defaultClient().context

		assert.Equal(t, DefaultEnvironment, ctx.Environment)
		assert.True(t, ctx.Options.RestrictRelay)
		assert.Equal(t, DefaultRegexMaxBlacklist, ctx.Options.RegexMaxBlacklist)
		assert.Equal(t, DefaultRegexMaxTimeLimit, ctx.Options.RegexMaxTimeLimit)
		assert.Equal(t, DefaultRemoteConnectTimeout, ctx.Options.Remote.ConnectTimeout)
		assert.Equal(t, DefaultRemoteReadTimeout, ctx.Options.Remote.ReadTimeout)
		assert.Equal(t, DefaultRemoteWriteTimeout, ctx.Options.Remote.WriteTimeout)
		assert.Equal(t, DefaultRemotePoolTimeout, ctx.Options.Remote.PoolTimeout)
	})
}
