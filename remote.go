package client

import (
	"bytes"
	"cmp"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	StrategyValue = "VALUE_VALIDATION"
)

type criteriaEntry struct {
	Strategy string `json:"strategy"`
	Input    string `json:"input"`
}

type authResponse struct {
	Token *string     `json:"token"`
	Exp   json.Number `json:"exp"`
}

type criteriaResponse struct {
	Result   bool           `json:"result"`
	Reason   string         `json:"reason"`
	Metadata map[string]any `json:"metadata"`
}

func (c *Client) ensureToken() (string, error) {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	if strings.TrimSpace(c.authToken) != "" && !tokenExpired(c.authTokenExp) {
		return c.authToken, nil
	}

	ctx := c.Context()
	endpoint := strings.TrimRight(ctx.URL, "/") + "/criteria/auth"

	response, err := c.doJSONRequest(
		http.MethodPost,
		endpoint,
		map[string]any{
			"domain":      ctx.Domain,
			"component":   ctx.Component,
			"environment": ctx.Environment,
		},
		map[string]string{
			"switcher-api-key": ctx.APIKey,
			"Content-Type":     "application/json",
		},
	)
	if err != nil {
		return "", newRemoteAuthError("[auth] remote unavailable")
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return "", newRemoteAuthError("invalid API key")
	}

	var payload authResponse
	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", err
	}

	if payload.Token == nil {
		c.authToken = ""
		c.authTokenExp = parseTokenExpiration(payload.Exp)
		return "", nil
	}

	c.authToken = *payload.Token
	c.authTokenExp = parseTokenExpiration(payload.Exp)
	return c.authToken, nil
}

func (c *Client) checkCriteria(token string, switcher *Switcher, showDetails bool) (ResultDetail, error) {
	ctx := c.Context()
	endpoint := strings.TrimRight(ctx.URL, "/") + "/criteria"

	query := make(url.Values)
	query.Set("showReason", strings.ToLower(strconvFormatBool(showDetails)))
	query.Set("key", switcher.key)

	entries := switcher.entries
	if entries == nil {
		entries = []criteriaEntry{}
	}

	response, err := c.doJSONRequest(
		http.MethodPost,
		endpoint+"?"+query.Encode(),
		map[string]any{
			"entry": entries,
		},
		map[string]string{
			"Authorization": "Bearer " + token,
			"Content-Type":  "application/json",
		},
	)
	if err != nil {
		return ResultDetail{}, newRemoteCriteriaError("[check_criteria] remote unavailable")
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return ResultDetail{}, newRemoteCriteriaError("[check_criteria] failed with status: %d", response.StatusCode)
	}

	var payload criteriaResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return ResultDetail{}, err
	}

	if payload.Metadata == nil {
		payload.Metadata = map[string]any{}
	}

	return ResultDetail(payload), nil
}

func (c *Client) doJSONRequest(method, endpoint string, payload any, headers map[string]string) (*http.Response, error) {
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequestWithContext(context.Background(), method, endpoint, bytes.NewReader(body))

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	return c.httpClient().Do(request)
}

func (c *Client) httpClient() *http.Client {
	c.httpClientMu.Lock()
	defer c.httpClientMu.Unlock()

	if c.httpClient_ != nil {
		return c.httpClient_
	}

	ctx := c.Context()
	dialer := &net.Dialer{
		Timeout: ctx.Options.Remote.ConnectTimeout,
	}

	transport := &http.Transport{
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: ctx.Options.Remote.ConnectTimeout,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	c.httpClient_ = &http.Client{
		Transport: transport,
		Timeout:   cmp.Or(ctx.Options.Remote.Timeout, DefaultRemoteTimeout),
	}

	return c.httpClient_
}

func missingTokenError(token string) error {
	if strings.TrimSpace(token) != "" {
		return nil
	}

	return errors.New("something went wrong: missing token field")
}

func parseTokenExpiration(value json.Number) int64 {
	parsed, _ := value.Int64()
	return parsed
}

func tokenExpired(expiration int64) bool {
	if expiration == 0 {
		return true
	}

	if expiration > 1_000_000_000_000 {
		return time.Now().UnixMilli() >= expiration
	}

	return time.Now().Unix() >= expiration
}

func strconvFormatBool(value bool) string {
	if value {
		return "true"
	}

	return "false"
}
