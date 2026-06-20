package client

import (
	"bytes"
	"cmp"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
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

type switchersCheckResponse struct {
	NotFound []string `json:"not_found"`
}

type snapshotCheckResponse struct {
	Status bool `json:"status"`
}

type resolveSnapshotResponse struct {
	Data struct {
		Domain SnapshotDomain `json:"domain"`
	} `json:"data"`
}

const contentTypeJSON = "application/json"

func (c *Client) authHeaders(token string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  contentTypeJSON,
	}
}

func (c *Client) ensureToken() (string, error) {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	if strings.TrimSpace(c.authToken) != "" && c.authToken != silentModeAuthToken && !tokenExpired(c.authTokenExp) {
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

	response, err := c.doJSONRequest(
		http.MethodPost,
		endpoint+"?"+query.Encode(),
		map[string]any{
			"entry": switcher.entries,
		},
		c.authHeaders(token),
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

func (c *Client) checkSwitchers(token string, switcherKeys []string) error {
	ctx := c.Context()
	endpoint := strings.TrimRight(ctx.URL, "/") + "/criteria/switchers_check"

	response, err := c.doJSONRequest(
		http.MethodPost,
		endpoint,
		map[string]any{
			"switchers": switcherKeys,
		},
		c.authHeaders(token),
	)
	if err != nil {
		return newRemoteError("[check_switchers] remote unavailable")
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return newRemoteError("[check_switchers] failed with status: %d", response.StatusCode)
	}

	var payload switchersCheckResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return err
	}

	if err := newRemoteSwitcherError(payload.NotFound); err != nil {
		return err
	}

	return nil
}

func (c *Client) checkSnapshotVersion(token string, snapshotVersion int) (bool, error) {
	ctx := c.Context()
	endpoint := fmt.Sprintf("%s/criteria/snapshot_check/%d", strings.TrimRight(ctx.URL, "/"), snapshotVersion)

	response, err := c.doJSONRequest(
		http.MethodGet,
		endpoint,
		nil,
		c.authHeaders(token),
	)
	if err != nil {
		return false, newRemoteSnapshotError("[check_snapshot_version] remote unavailable")
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return false, newRemoteSnapshotError("[check_snapshot_version] failed with status: %d", response.StatusCode)
	}

	var payload snapshotCheckResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return false, newRemoteSnapshotError("[check_snapshot_version] failed to decode response: %v", err)
	}

	return payload.Status, nil
}

func (c *Client) resolveSnapshot(token string) (*Snapshot, error) {
	ctx := c.Context()
	endpoint := strings.TrimRight(ctx.URL, "/") + "/graphql"

	response, err := c.doJSONRequest(
		http.MethodPost,
		endpoint,
		map[string]string{
			"query": fmt.Sprintf(`
				query domain {
					domain(name: %q, environment: %q, _component: %q) {
						name version activated
						group { name activated
							config { key activated
								strategies { strategy activated operation values }
								relay { type activated }
							}
						}
					}
				}
			`, ctx.Domain, ctx.Environment, ctx.Component),
		},
		c.authHeaders(token),
	)
	if err != nil {
		return nil, newRemoteSnapshotError("[resolve_snapshot] remote unavailable")
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return nil, newRemoteSnapshotError("[resolve_snapshot] failed with status: %d", response.StatusCode)
	}

	var payload resolveSnapshotResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, newRemoteSnapshotError("[resolve_snapshot] failed to decode response: %v", err)
	}

	return &Snapshot{Domain: payload.Data.Domain}, nil
}

func (c *Client) checkAPIHealth() bool {
	ctx := c.Context()
	endpoint := strings.TrimRight(ctx.URL, "/") + "/check"

	response, err := c.doJSONRequest(
		http.MethodGet,
		endpoint,
		nil,
		map[string]string{},
	)
	if err != nil {
		return false
	}
	defer func() {
		_ = response.Body.Close()
	}()

	return response.StatusCode == http.StatusOK
}

func (c *Client) doJSONRequest(method, endpoint string, payload any, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if payload != nil {
		body, _ := json.Marshal(payload)
		bodyReader = bytes.NewReader(body)
	}

	request, _ := http.NewRequestWithContext(context.Background(), method, endpoint, bodyReader)

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	client, err := c.httpClient()
	if err != nil {
		return nil, err
	}

	return client.Do(request)
}

func (c *Client) httpClient() (*http.Client, error) {
	c.httpClientMu.Lock()
	defer c.httpClientMu.Unlock()

	if c.httpClient_ != nil {
		return c.httpClient_, nil
	}

	ctx := c.Context()
	transport, err := newRemoteTransport(ctx)
	if err != nil {
		return nil, err
	}

	c.httpClient_ = &http.Client{
		Transport: transport,
		Timeout:   cmp.Or(ctx.Options.Remote.Timeout, DefaultRemoteTimeout),
	}

	return c.httpClient_, nil
}

func newRemoteTransport(ctx Context) (*http.Transport, error) {
	dialer := &net.Dialer{
		Timeout: ctx.Options.Remote.ConnectTimeout,
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	certPath := strings.TrimSpace(ctx.Options.Remote.CertPath)
	if certPath != "" {
		certificate, err := tls.LoadX509KeyPair(certPath, certPath)
		if err != nil {
			return nil, fmt.Errorf("loading remote certificate %q: %w", certPath, err)
		}

		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	return &http.Transport{
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: ctx.Options.Remote.ConnectTimeout,
		TLSClientConfig:     tlsConfig,
	}, nil
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
