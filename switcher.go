package client

import (
	"fmt"
	"strings"
)

type Switcher struct {
	client *Client
	key    string
}

func (s *Switcher) Validate() error {
	if s == nil || s.client == nil {
		return fmt.Errorf("something went wrong: client is not configured")
	}

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
