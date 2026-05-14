package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalStrategyOperations(t *testing.T) {
	makeStrategy := func(strategy, operation string, values []string) SnapshotStrategy {
		return SnapshotStrategy{
			Strategy:  strategy,
			Operation: operation,
			Values:    values,
			Activated: true,
		}
	}

	t.Run("should evaluate value strategy", func(t *testing.T) {
		values := []string{"USER_1", "USER_2"}

		assert.True(t, processLocalStrategy(makeStrategy(StrategyValue, "EXIST", values), "USER_1"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyValue, "EXIST", values), "USER_123"))
		assert.True(t, processLocalStrategy(makeStrategy(StrategyValue, "NOT_EXIST", values), "USER_123"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyValue, "NOT_EXIST", values), "USER_1"))
		assert.True(t, processLocalStrategy(makeStrategy(StrategyValue, "EQUAL", values), "USER_1"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyValue, "EQUAL", values), "USER_2"))
		assert.True(t, processLocalStrategy(makeStrategy(StrategyValue, "NOT_EQUAL", values), "USER_123"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyValue, "NOT_EQUAL", values), "USER_2"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyValue, "EXIST", []string{}), "USER_1"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyValue, "INVALID_OP", values), "USER_1"))
	})

	t.Run("should evaluate numeric strategy", func(t *testing.T) {
		assert.True(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "EXIST", []string{"1", "3"}), "3"))
		assert.False(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "NOT_EXIST", []string{"1", "3"}), "1"))
		assert.True(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "NOT_EXIST", []string{"1", "3"}), "2"))
		assert.True(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "EQUAL", []string{"1"}), "1"))
		assert.False(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "EQUAL", []string{"1"}), "2"))
		assert.True(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "NOT_EQUAL", []string{"1"}), "2"))
		assert.True(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "GREATER", []string{"1"}), "2"))
		assert.False(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "GREATER", []string{"1"}), "0"))
		assert.True(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "LOWER", []string{"1"}), "0"))
		assert.True(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "BETWEEN", []string{"1", "3"}), "2"))
		assert.False(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "GREATER", []string{"1", "3"}), "ABC"))
		assert.False(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "BETWEEN", []string{"1"}), "1"))
		assert.False(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "BETWEEN", []string{"ABC", "3"}), "2"))
		assert.False(t, processLocalStrategy(makeStrategy("NUMERIC_VALIDATION", "INVALID_OP", []string{"1", "3"}), "2"))
	})

	t.Run("should evaluate date strategy", func(t *testing.T) {
		assert.True(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "LOWER", []string{"2019-12-01"}), "2019-11-26"))
		assert.True(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "LOWER", []string{"2019-12-01"}), "2019-12-01"))
		assert.False(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "LOWER", []string{"2019-12-01"}), "2019-12-02"))
		assert.True(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "GREATER", []string{"2019-12-01"}), "2019-12-02"))
		assert.True(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "GREATER", []string{"2019-12-01"}), "2019-12-01"))
		assert.False(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "GREATER", []string{"2019-12-01"}), "2019-11-10"))
		assert.True(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "BETWEEN", []string{"2019-12-01", "2019-12-05"}), "2019-12-03"))
		assert.False(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "BETWEEN", []string{"2019-12-01", "2019-12-05"}), "2019-12-12"))
		assert.False(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "BETWEEN", []string{"2019-12-01"}), "2019-12-03"))
		assert.True(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "LOWER", []string{"2019-12-01T08:30"}), "2019-12-01T07:00"))
		assert.False(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "LOWER", []string{"2019-12-01"}), "invalid-date"))
		assert.False(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "LOWER", []string{"invalid-date"}), "2019-12-01"))
		assert.False(t, processLocalStrategy(makeStrategy("DATE_VALIDATION", "INVALID_OP", []string{"2019-12-01"}), "2019-12-01"))
	})

	t.Run("should evaluate time strategy", func(t *testing.T) {
		assert.True(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "LOWER", []string{"08:00"}), "06:00"))
		assert.True(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "LOWER", []string{"08:00"}), "08:00"))
		assert.False(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "LOWER", []string{"08:00"}), "10:00"))
		assert.True(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "GREATER", []string{"08:00"}), "10:00"))
		assert.True(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "GREATER", []string{"08:00"}), "08:00"))
		assert.False(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "GREATER", []string{"08:00"}), "06:00"))
		assert.True(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "BETWEEN", []string{"08:00", "10:00"}), "09:00"))
		assert.False(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "BETWEEN", []string{"08:00", "10:00"}), "07:00"))
		assert.False(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "BETWEEN", []string{"08:00"}), "09:00"))
		assert.False(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "GREATER", []string{"08:00"}), "invalid-time"))
		assert.False(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "GREATER", []string{"invalid-time"}), "08:00"))
		assert.False(t, processLocalStrategy(makeStrategy("TIME_VALIDATION", "INVALID_OP", []string{"08:00"}), "08:00"))
	})

	t.Run("should evaluate payload strategy", func(t *testing.T) {
		payload := `{"id":"1","order":{"qty":1,"deliver":{"expect":"2019-12-10","tracking":[{"date":"2019-12-09","status":"sent"},{"date":"2019-12-10","status":"delivered","comments":"comments"}]}}}`

		assert.True(t, processLocalStrategy(makeStrategy("PAYLOAD_VALIDATION", "HAS_ONE", []string{"login", "order.qty"}), payload))
		assert.False(t, processLocalStrategy(makeStrategy("PAYLOAD_VALIDATION", "HAS_ONE", []string{"user"}), payload))
		assert.True(t, processLocalStrategy(makeStrategy("PAYLOAD_VALIDATION", "HAS_ALL", []string{"id", "order", "order.qty"}), payload))
		assert.False(t, processLocalStrategy(makeStrategy("PAYLOAD_VALIDATION", "HAS_ALL", []string{"id", "missing"}), payload))
		assert.False(t, processLocalStrategy(makeStrategy("PAYLOAD_VALIDATION", "HAS_ALL", []string{}), "NOT_JSON"))
		assert.False(t, processLocalStrategy(makeStrategy("PAYLOAD_VALIDATION", "INVALID_OP", []string{"id"}), payload))
	})

	t.Run("should evaluate network strategy", func(t *testing.T) {
		assert.True(t, processLocalStrategy(makeStrategy(StrategyNetwork, "EXIST", []string{"10.0.0.0/30"}), "10.0.0.3"))
		assert.True(t, processLocalStrategy(makeStrategy(StrategyNetwork, "EXIST", []string{"10.0.0.3/24"}), "10.0.0.3"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyNetwork, "EXIST", []string{"10.0.0.0/30"}), "10.0.0.4"))
		assert.True(t, processLocalStrategy(makeStrategy(StrategyNetwork, "NOT_EXIST", []string{"10.0.0.0/30"}), "10.0.0.4"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyNetwork, "NOT_EXIST", []string{"10.0.0.0/30"}), "10.0.0.3"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyNetwork, "EXIST", []string{"10.0.0.0/30"}), "not-an-ip"))
		assert.False(t, processLocalStrategy(makeStrategy(StrategyNetwork, "INVALID_OP", []string{"10.0.0.0/30"}), "10.0.0.3"))
	})

	t.Run("should evaluate regex strategy", func(t *testing.T) {
		assert.True(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "EXIST", []string{`\bUSER_[0-9]{1,2}\b`}), "USER_1"))
		assert.False(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "EXIST", []string{`\bUSER_[0-9]{1,2}\b`}), "USER_123"))
		assert.True(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "NOT_EXIST", []string{`\bUSER_[0-9]{1,2}\b`}), "USER_123"))
		assert.True(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "EQUAL", []string{`USER_[0-9]{1,2}`}), "USER_11"))
		assert.False(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "EQUAL", []string{`USER_[0-9]{1,2}`}), "USER_123"))
		assert.True(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "NOT_EQUAL", []string{`USER_[0-9]{1,2}`}), "USER_123"))
		assert.False(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "NOT_EQUAL", []string{`USER_[0-9]{1,2}`}), "USER_1"))
		assert.False(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "EQUAL", []string{"["}), "USER_11"))
		assert.False(t, processLocalStrategy(makeStrategy("REGEX_VALIDATION", "INVALID_OP", []string{`USER_[0-9]{1,2}`}), "USER_11"))
	})

	t.Run("should return false for unknown strategy", func(t *testing.T) {
		assert.False(t, processLocalStrategy(makeStrategy("UNKNOWN", "EQUAL", []string{"USER_1"}), "USER_1"))
	})
}
