package client

import (
	"encoding/json"
	"net"
	"regexp"
	"slices"
	"strconv"
	"time"
)

const (
	StrategyValue   = "VALUE_VALIDATION"
	StrategyNumeric = "NUMERIC_VALIDATION"
	StrategyDate    = "DATE_VALIDATION"
	StrategyTime    = "TIME_VALIDATION"
	StrategyPayload = "PAYLOAD_VALIDATION"
	StrategyNetwork = "NETWORK_VALIDATION"
	StrategyRegex   = "REGEX_VALIDATION"
)

const (
	OperationExist    = "EXIST"
	OperationNotExist = "NOT_EXIST"
	OperationEqual    = "EQUAL"
	OperationNotEqual = "NOT_EQUAL"
	OperationGreater  = "GREATER"
	OperationLower    = "LOWER"
	OperationBetween  = "BETWEEN"
	OperationHasOne   = "HAS_ONE"
	OperationHasAll   = "HAS_ALL"
)

func processLocalStrategy(strategy SnapshotStrategy, input string) bool {
	switch strategy.Strategy {
	case StrategyValue:
		return processValueStrategy(strategy.Operation, strategy.Values, input)
	case StrategyNumeric:
		return processNumericStrategy(strategy.Operation, strategy.Values, input)
	case StrategyDate:
		return processDateStrategy(strategy.Operation, strategy.Values, input)
	case StrategyTime:
		return processTimeStrategy(strategy.Operation, strategy.Values, input)
	case StrategyPayload:
		return processPayloadStrategy(strategy.Operation, strategy.Values, input)
	case StrategyNetwork:
		return processNetworkStrategy(strategy.Operation, strategy.Values, input)
	case StrategyRegex:
		return processRegexStrategy(strategy.Operation, strategy.Values, input)
	default:
		return false
	}
}

func processValueStrategy(operation string, values []string, input string) bool {
	switch operation {
	case OperationExist, OperationEqual:
		if len(values) == 0 {
			return false
		}
		if operation == OperationEqual {
			return input == values[0]
		}
		return containsString(values, input)
	case OperationNotExist, OperationNotEqual:
		return !containsString(values, input)
	default:
		return false
	}
}

func processNumericStrategy(operation string, values []string, input string) bool {
	numericInput, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return false
	}

	numericValues := make([]float64, 0, len(values))
	for _, value := range values {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		numericValues = append(numericValues, parsed)
	}

	switch operation {
	case OperationExist, OperationEqual:
		return containsFloat(numericValues, numericInput)
	case OperationNotExist, OperationNotEqual:
		return !containsFloat(numericValues, numericInput)
	case OperationGreater:
		return anyFloat(numericValues, func(candidate float64) bool { return numericInput > candidate })
	case OperationLower:
		return anyFloat(numericValues, func(candidate float64) bool { return numericInput < candidate })
	case OperationBetween:
		if len(numericValues) < 2 {
			return false
		}
		return numericValues[0] <= numericInput && numericInput <= numericValues[1]
	default:
		return false
	}
}

func processDateStrategy(operation string, values []string, input string) bool {
	dateInput, ok := parseDateValue(input)
	if !ok {
		return false
	}

	dateValues := make([]time.Time, 0, len(values))
	for _, value := range values {
		parsed, ok := parseDateValue(value)
		if !ok {
			return false
		}
		dateValues = append(dateValues, parsed)
	}

	switch operation {
	case OperationLower:
		return anyDate(dateValues, func(candidate time.Time) bool { return dateInput.Before(candidate) || dateInput.Equal(candidate) })
	case OperationGreater:
		return anyDate(dateValues, func(candidate time.Time) bool { return dateInput.After(candidate) || dateInput.Equal(candidate) })
	case OperationBetween:
		if len(dateValues) < 2 {
			return false
		}
		return !dateInput.Before(dateValues[0]) && !dateInput.After(dateValues[1])
	default:
		return false
	}
}

func processTimeStrategy(operation string, values []string, input string) bool {
	timeInput, ok := parseClockValue(input)
	if !ok {
		return false
	}

	timeValues := make([]time.Time, 0, len(values))
	for _, value := range values {
		parsed, ok := parseClockValue(value)
		if !ok {
			return false
		}
		timeValues = append(timeValues, parsed)
	}

	switch operation {
	case OperationLower:
		return anyDate(timeValues, func(candidate time.Time) bool { return timeInput.Before(candidate) || timeInput.Equal(candidate) })
	case OperationGreater:
		return anyDate(timeValues, func(candidate time.Time) bool { return timeInput.After(candidate) || timeInput.Equal(candidate) })
	case OperationBetween:
		if len(timeValues) < 2 {
			return false
		}
		return !timeInput.Before(timeValues[0]) && !timeInput.After(timeValues[1])
	default:
		return false
	}
}

func processPayloadStrategy(operation string, values []string, input string) bool {
	var payload any
	if err := json.Unmarshal([]byte(input), &payload); err != nil {
		return false
	}

	keys := flattenPayloadKeys(payload)
	switch operation {
	case OperationHasOne:
		return anyString(values, func(value string) bool { return slices.Contains(keys, value) })
	case OperationHasAll:
		return allString(values, func(value string) bool { return slices.Contains(keys, value) })
	default:
		return false
	}
}

func processNetworkStrategy(operation string, values []string, input string) bool {
	ip := net.ParseIP(input)
	if ip == nil {
		return false
	}

	switch operation {
	case OperationExist:
		return networkExists(values, ip)
	case OperationNotExist:
		return !networkExists(values, ip)
	default:
		return false
	}
}

func processRegexStrategy(operation string, values []string, input string) bool {
	switch operation {
	case OperationExist:
		return anyString(values, func(pattern string) bool { return regexMatch(pattern, input, false) })
	case OperationNotExist:
		return !anyString(values, func(pattern string) bool { return regexMatch(pattern, input, false) })
	case OperationEqual:
		return anyString(values, func(pattern string) bool { return regexMatch(pattern, input, true) })
	case OperationNotEqual:
		return !anyString(values, func(pattern string) bool { return regexMatch(pattern, input, true) })
	default:
		return false
	}
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}

func containsFloat(values []float64, target float64) bool {
	return slices.Contains(values, target)
}

func anyFloat(values []float64, predicate func(float64) bool) bool {
	return slices.ContainsFunc(values, predicate)
}

func anyDate(values []time.Time, predicate func(time.Time) bool) bool {
	return slices.ContainsFunc(values, predicate)
}

func anyString(values []string, predicate func(string) bool) bool {
	return slices.ContainsFunc(values, predicate)
}

func allString(values []string, predicate func(string) bool) bool {
	for _, value := range values {
		if !predicate(value) {
			return false
		}
	}

	return true
}

func flattenPayloadKeys(value any) []string {
	keys := make([]string, 0)
	seen := make(map[string]struct{})
	flattenPayloadKeysWithPrefix(value, "", seen)

	for key := range seen {
		keys = append(keys, key)
	}

	return keys
}

func flattenPayloadKeysWithPrefix(value any, prefix string, seen map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			seen[next] = struct{}{}
			flattenPayloadKeysWithPrefix(nested, next, seen)
		}
	case []any:
		if prefix != "" {
			seen[prefix] = struct{}{}
		}
		for _, nested := range typed {
			flattenPayloadKeysWithPrefix(nested, prefix, seen)
		}
	default:
		if prefix != "" {
			seen[prefix] = struct{}{}
		}
	}
}

func parseDateValue(value string) (time.Time, bool) {
	layouts := []string{time.DateOnly, "2006-01-02T15:04"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
}

func parseClockValue(value string) (time.Time, bool) {
	if parsed, err := time.Parse("15:04", value); err == nil {
		return parsed, true
	}

	return time.Time{}, false
}

func networkExists(values []string, input net.IP) bool {
	for _, value := range values {
		if _, network, err := net.ParseCIDR(value); err == nil {
			if network.Contains(input) {
				return true
			}
			continue
		}

		if parsed := net.ParseIP(value); parsed != nil && parsed.Equal(input) {
			return true
		}
	}

	return false
}

func regexMatch(pattern, input string, fullMatch bool) bool {
	if fullMatch {
		pattern = "^(?:" + pattern + ")$"
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return compiled.MatchString(input)
}
