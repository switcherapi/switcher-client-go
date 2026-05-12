package client

import (
	"fmt"
	"strings"
)

type Switcher struct {
	client  *Client
	key     string
	entries []criteriaEntry
}

func (s *Switcher) Validate() error {
	ctx := s.client.Context()
	missingFields := make([]string, 0, 3)

	if strings.TrimSpace(ctx.URL) == "" {
		missingFields = append(missingFields, "url")
	}

	if strings.TrimSpace(ctx.Component) == "" {
		missingFields = append(missingFields, "component")
	}

	if strings.TrimSpace(ctx.APIKey) == "" {
		missingFields = append(missingFields, "api_key")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf(
			"something went wrong: missing or empty required fields (%s)",
			strings.Join(missingFields, ", "),
		)
	}

	if strings.TrimSpace(s.key) == "" {
		return fmt.Errorf("something went wrong: missing key field")
	}

	return nil
}

func (s *Switcher) CheckValue(input string) *Switcher {
	s.entries = []criteriaEntry{
		{
			Strategy: StrategyValue,
			Input:    input,
		},
	}

	return s
}

func (s *Switcher) Prepare(key string) error {
	if strings.TrimSpace(key) != "" {
		s.key = key
	}

	if err := s.Validate(); err != nil {
		return err
	}

	token, err := s.client.ensureToken()
	if err != nil {
		return err
	}

	return missingTokenError(token)
}

func (s *Switcher) IsOn() (bool, error) {
	result, err := s.submit(false)
	if err != nil {
		return false, err
	}

	return result.Result, nil
}

func (s *Switcher) IsOnWithDetails() (ResultDetail, error) {
	return s.submit(true)
}

func (s *Switcher) submit(showDetails bool) (ResultDetail, error) {
	if err := s.Validate(); err != nil {
		return ResultDetail{}, err
	}

	token, err := s.client.ensureToken()
	if err != nil {
		return ResultDetail{}, err
	}

	if err := missingTokenError(token); err != nil {
		return ResultDetail{}, err
	}

	return s.client.checkCriteria(token, s, showDetails)
}
