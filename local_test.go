package client

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwitcherLocalEvaluationCommonBehavior(t *testing.T) {
	t.Run("should use local snapshot to evaluate a switcher without strategies", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("FF2FOR2022").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should use local snapshot to evaluate a switcher with value validation", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("FF2FOR2020").CheckValue("Japan").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when the domain is deactivated", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_disabled")

		got, err := GetSwitcher("FEATURE").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Domain is disabled", got.Reason)
	})

	t.Run("should return disabled when the group is deactivated", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("FF2FOR2040").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Group disabled", got.Reason)
	})

	t.Run("should return disabled when the config is deactivated", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("FF2FOR2031").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Config disabled", got.Reason)
	})

	t.Run("should return enabled when the strategy is deactivated", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("FF2FOR2021").CheckNetwork("10.0.0.3").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when relay is enabled and relay restriction is active", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		got, err := GetSwitcher("USECASE103").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Config has relay enabled", got.Reason)
	})
}

func TestSwitcherLocalEvaluationValueStrategies(t *testing.T) {
	t.Run("should use local snapshot to evaluate a switcher with the generic check API", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("FF2FOR2020").Check(StrategyValue, "Japan").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when a value strategy does not receive any input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("FF2FOR2020").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'VALUE_VALIDATION' did not receive any input", got.Reason)
	})

	t.Run("should return enabled when value EXIST matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("VALUE_EXIST").CheckValue("guest").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when value EXIST does not match the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("VALUE_EXIST").CheckValue("anonymous").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'VALUE_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return disabled when operation is invalid for the strategy", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("HAS_ALL").CheckValue("anonymous").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'VALUE_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return disabled when strategy input does not match snapshot settings", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("FF2FOR2020").CheckNetwork("10.0.0.3").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'VALUE_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return enabled when value EQUAL matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_value_only")

		got, err := GetSwitcher("VALUE_EQUAL").CheckValue("pro-user").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})
}

func TestSwitcherLocalEvaluationNetworkStrategies(t *testing.T) {
	t.Run("should return enabled when the network input is inside a CIDR range", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_EXIST_CIDR").CheckNetwork("10.0.0.3").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when the network input is outside a CIDR range", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_EXIST_CIDR").CheckNetwork("192.168.1.2").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'NETWORK_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return enabled when the network input exactly matches an IP value", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_EXIST_IP").CheckNetwork("10.0.0.3").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when the network input is invalid", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_EXIST_CIDR").CheckNetwork("not-an-ip").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'NETWORK_VALIDATION' does not agree", got.Reason)
	})

	t.Run("should return enabled when NOT_EXIST network strategy does not match the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_NOT_EXIST_CIDR").CheckNetwork("192.168.1.10").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return disabled when NOT_EXIST network strategy matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_network_only")

		got, err := GetSwitcher("NET_NOT_EXIST_CIDR").CheckNetwork("10.0.0.3").IsOnWithDetails()

		assert.NoError(t, err)
		assert.False(t, got.Result)
		assert.Equal(t, "Strategy 'NETWORK_VALIDATION' does not agree", got.Reason)
	})
}

func TestSwitcherLocalEvaluationNumericStrategies(t *testing.T) {
	t.Run("should return enabled when numeric EXIST matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_numeric_only")

		got, err := GetSwitcher("NUMERIC_EXIST").CheckNumeric("3").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when numeric NOT_EXIST does not match the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_numeric_only")

		got, err := GetSwitcher("NUMERIC_NOT_EXIST").CheckNumeric("2").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when numeric EQUAL matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_numeric_only")

		got, err := GetSwitcher("NUMERIC_EQUAL").CheckNumeric("7").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when numeric NOT_EQUAL differs from the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_numeric_only")

		got, err := GetSwitcher("NUMERIC_NOT_EQUAL").CheckNumeric("8").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when numeric GREATER matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_numeric_only")

		got, err := GetSwitcher("NUMERIC_GREATER").CheckNumeric("11").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when numeric LOWER matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_numeric_only")

		got, err := GetSwitcher("NUMERIC_LOWER").CheckNumeric("9").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when numeric BETWEEN matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_numeric_only")

		got, err := GetSwitcher("NUMERIC_BETWEEN").CheckNumeric("15").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})
}

func TestSwitcherLocalEvaluationDateStrategies(t *testing.T) {
	t.Run("should return enabled when date LOWER matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_date_only")

		got, err := GetSwitcher("DATE_LOWER").CheckDate("2019-11-30").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when date GREATER matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_date_only")

		got, err := GetSwitcher("DATE_GREATER").CheckDate("2019-12-02").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when date BETWEEN matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_date_only")

		got, err := GetSwitcher("DATE_BETWEEN").CheckDate("2019-12-03").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})
}

func TestSwitcherLocalEvaluationTimeStrategies(t *testing.T) {
	t.Run("should return enabled when time LOWER matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_time_only")

		got, err := GetSwitcher("TIME_LOWER").CheckTime("06:00").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when time GREATER matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_time_only")

		got, err := GetSwitcher("TIME_GREATER").CheckTime("10:00").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when time BETWEEN matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_time_only")

		got, err := GetSwitcher("TIME_BETWEEN").CheckTime("09:00").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})
}

func TestSwitcherLocalEvaluationPayloadStrategies(t *testing.T) {
	t.Run("should return enabled when payload HAS_ONE matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_payload_only")

		got, err := GetSwitcher("PAYLOAD_HAS_ONE").CheckPayload(`{"id":"1","order":{"qty":2}}`).IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when payload HAS_ALL matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_payload_only")

		got, err := GetSwitcher("PAYLOAD_HAS_ALL").CheckPayload(`{"id":"1","user":{"role":"admin"}}`).IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})
}

func TestSwitcherLocalEvaluationRegexStrategies(t *testing.T) {
	t.Run("should return enabled when regex EXIST matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_regex_only")

		got, err := GetSwitcher("REGEX_EXIST").CheckRegex("USER_11").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when regex NOT_EXIST does not match the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_regex_only")

		got, err := GetSwitcher("REGEX_NOT_EXIST").CheckRegex("USER_123").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when regex EQUAL matches the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_regex_only")

		got, err := GetSwitcher("REGEX_EQUAL").CheckRegex("USER_11").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should return enabled when regex NOT_EQUAL differs from the input", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_regex_only")

		got, err := GetSwitcher("REGEX_NOT_EQUAL").CheckRegex("USER_123").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("should use the generic check API with non-value strategies", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default_regex_only")

		got, err := GetSwitcher("REGEX_EQUAL").Check(StrategyRegex, "USER_11").IsOn()

		assert.NoError(t, err)
		assert.True(t, got)
	})
}

func TestSwitcherLocalEvaluationErrors(t *testing.T) {
	t.Run("should return an error when the key is not found in the snapshot", func(t *testing.T) {
		useLocalSnapshotFixture(t, "default")

		_, err := GetSwitcher("INVALID_KEY").IsOn()

		assert.Error(t, err)
		var localCriteriaErr *LocalCriteriaError
		assert.ErrorAs(t, err, &localCriteriaErr)
		assert.EqualError(t, err, "Config with key 'INVALID_KEY' not found in the snapshot")
	})

	t.Run("should return an error when no snapshot has been loaded", func(t *testing.T) {
		BuildContext(Context{
			Domain: "My Domain",
			Options: ContextOptions{
				Local:            true,
				SnapshotLocation: filepath.Join(t.TempDir(), "missing"),
			},
		})

		_, err := GetSwitcher("FF2FOR2022").IsOn()

		assert.Error(t, err)
		var localCriteriaErr *LocalCriteriaError
		assert.ErrorAs(t, err, &localCriteriaErr)
		assert.EqualError(t, err, "Snapshot not loaded. Try to use 'Client.load_snapshot()'")
	})
}

func useLocalSnapshotFixture(t *testing.T, environment string) {
	t.Helper()

	BuildContext(Context{
		Domain:      "My Domain",
		Environment: environment,
		Options: ContextOptions{
			Local:            true,
			SnapshotLocation: snapshotFixtureDir(),
		},
	})

	_, err := LoadSnapshot(nil)
	assert.NoError(t, err)
}

func snapshotFixtureDir() string {
	return filepath.Join("testdata", "snapshots")
}
