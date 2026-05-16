package client

import "time"

const silentModeAuthToken = "SILENT"

func (c *Client) shouldUseLocalSilentMode() bool {
	if !c.hasSilentMode() {
		return false
	}

	token, expiration := c.authState()
	if token != silentModeAuthToken {
		return false
	}

	if !tokenExpired(expiration) {
		return true
	}

	c.updateSilentToken()
	if c.checkAPIHealth() {
		c.clearSilentToken()
		return false
	}

	return true
}

func (c *Client) fallbackToSilentMode(switcher *Switcher, err error) (ResultDetail, error) {
	if !c.hasSilentMode() {
		return ResultDetail{}, err
	}

	c.notifyError(err)
	c.updateSilentToken()
	return checkLocalCriteria(c.snapshotState(), switcher)
}

func (c *Client) hasSilentMode() bool {
	return c.Context().Options.SilentMode > 0
}

func (c *Client) authState() (string, int64) {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	return c.authToken, c.authTokenExp
}

func (c *Client) updateSilentToken() {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	c.authToken = silentModeAuthToken
	c.authTokenExp = time.Now().Add(c.Context().Options.SilentMode).UnixMilli()
}

func (c *Client) clearSilentToken() {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	c.authToken = ""
	c.authTokenExp = 0
}
