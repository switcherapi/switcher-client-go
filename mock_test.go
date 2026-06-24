package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type cleanupRecorder struct {
	callbacks []func()
}

func (r *cleanupRecorder) Cleanup(callback func()) {
	r.callbacks = append(r.callbacks, callback)
}

func (r *cleanupRecorder) Helper() {}

func (r *cleanupRecorder) Run() {
	for _, callback := range r.callbacks {
		callback()
	}
}

func TestClientAssume(t *testing.T) {
	t.Run("should bypass evaluation with a forced true result", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").True()

		enabled, err := client.GetSwitcher("FEATURE01").IsOn()

		assert.NoError(t, err)
		assert.True(t, enabled)
	})

	t.Run("should return mocked details and metadata", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").False().WithMetadata(map[string]any{
			"message": "Feature is disabled",
		})

		response, err := client.GetSwitcher("FEATURE01").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, response.Result)
		assert.Equal(t, mockReasonFalse, response.Reason)
		assert.Equal(t, "Feature is disabled", response.Metadata["message"])
	})

	t.Run("should support conditional mocks based on multiple strategies", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").True().
			When(StrategyValue, []string{"guest", "admin"}).
			When(StrategyNetwork, "10.0.0.3")

		response, err := client.GetSwitcher("FEATURE01").
			CheckValue("guest").
			CheckNetwork("10.0.0.3").
			IsOnWithDetails()

		assert.NoError(t, err)
		assert.True(t, response.Result)
		assert.Equal(t, mockReasonTrue, response.Reason)
	})

	t.Run("should invert the forced result when a when condition does not match", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").True().When(StrategyValue, "Canada")

		response, err := client.GetSwitcher("FEATURE01").CheckValue("Brazil").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, response.Result)
		assert.Equal(t, "Forced to False when: [Canada] - input: Brazil", response.Reason)
	})

	t.Run("should report a true mismatch reason when a false assumption does not match", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").False().When(StrategyValue, "Canada")

		response, err := client.GetSwitcher("FEATURE01").CheckValue("Brazil").IsOnWithDetails()

		assert.NoError(t, err)
		assert.True(t, response.Result)
		assert.Equal(t, "Forced to True when: [Canada] - input: Brazil", response.Reason)
	})

	t.Run("should keep the forced result when the configured strategy input is not provided", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").True().When(StrategyNetwork, "10.0.0.3")

		response, err := client.GetSwitcher("FEATURE01").CheckValue("guest").IsOnWithDetails()

		assert.NoError(t, err)
		assert.True(t, response.Result)
		assert.Equal(t, mockReasonTrue, response.Reason)
	})

	t.Run("should accept non slice values in when and match them through string normalization", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").True().When(StrategyValue, 123)

		response, err := client.GetSwitcher("FEATURE01").CheckValue("123").IsOnWithDetails()

		assert.NoError(t, err)
		assert.True(t, response.Result)
		assert.Equal(t, mockReasonTrue, response.Reason)
	})

	t.Run("should replace a previous when condition for the same strategy", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").True().
			When(StrategyValue, "Canada").
			When(StrategyValue, "Brazil")

		response, err := client.GetSwitcher("FEATURE01").CheckValue("Canada").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, response.Result)
		assert.Equal(t, "Forced to False when: [Brazil] - input: Canada", response.Reason)
	})

	t.Run("should replace a previous assumption for the same key", func(t *testing.T) {
		client := NewClient(Context{})
		client.Assume("FEATURE01").True()
		client.Assume("FEATURE01").False()

		enabled, err := client.GetSwitcher("FEATURE01").IsOn()

		assert.NoError(t, err)
		assert.False(t, enabled)
	})

	t.Run("should forget the mocked result and restore normal evaluation", func(t *testing.T) {
		client := NewClient(Context{
			Domain:      "My Domain",
			Environment: "default",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotFixtureDir(),
			},
		})
		_, err := client.LoadSnapshot(nil)
		assert.NoError(t, err)

		client.Assume("FF2FOR2022").False()

		mocked, err := client.GetSwitcher("FF2FOR2022").IsOn()
		assert.NoError(t, err)
		assert.False(t, mocked)

		client.Forget("FF2FOR2022")

		enabled, err := client.GetSwitcher("FF2FOR2022").IsOn()
		assert.NoError(t, err)
		assert.True(t, enabled)
	})

	t.Run("should register cleanup through the Go test helper", func(t *testing.T) {
		client := NewClient(Context{
			Domain:      "My Domain",
			Environment: "default",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: snapshotFixtureDir(),
			},
		})
		_, err := client.LoadSnapshot(nil)
		assert.NoError(t, err)

		recorder := &cleanupRecorder{}
		client.Assume("FF2FOR2022").False().Cleanup(recorder)

		mocked, err := client.GetSwitcher("FF2FOR2022").IsOn()
		assert.NoError(t, err)
		assert.False(t, mocked)

		recorder.Run()

		enabled, err := client.GetSwitcher("FF2FOR2022").IsOn()
		assert.NoError(t, err)
		assert.True(t, enabled)
	})
}
