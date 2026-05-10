package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwitcherValidate(t *testing.T) {
	t.Run("should return an error when the client is not configured", func(t *testing.T) {
		var switcher *Switcher

		err := switcher.Validate()
		assert.EqualError(t, err, "something went wrong: client is not configured")
	})

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
