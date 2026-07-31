package client

import (
	"sync"
	"time"
)

// autoRenewBuffer is subtracted from the token's remaining lifetime before scheduling
// the next background renewal, so the renewal fires slightly ahead of expiration.
const autoRenewBuffer = 5 * time.Second

// autoRenewMinDelay is the minimum delay used for a scheduled renewal, preventing
// tight refresh loops when a token's remaining lifetime is very short or already past.
const autoRenewMinDelay = 1 * time.Second

// tokenAutoRenewer manages a background timer that proactively refreshes the client's
// auth token ahead of its expiration when RemoteOptions.AutoRenewToken is enabled.
//
// A generation counter guards against stale renewals: any scheduled or in-flight
// renewal from a previous generation is discarded rather than overwriting a newer
// token.
type tokenAutoRenewer struct {
	mu         sync.Mutex
	timer      *time.Timer
	generation int
}

func newTokenAutoRenewer() *tokenAutoRenewer {
	return &tokenAutoRenewer{}
}

// schedule arranges for a background renewal of client's auth token ahead of exp
// (a Unix timestamp in seconds or milliseconds, consistent with tokenExpired).
// Any previously scheduled renewal is cancelled.
func (r *tokenAutoRenewer) schedule(client *Client, exp int64) {
	delay := autoRenewDelay(exp)

	r.mu.Lock()
	r.generation++
	generation := r.generation
	previous := r.timer

	timer := time.AfterFunc(delay, func() {
		r.renew(client, generation)
	})
	r.timer = timer
	r.mu.Unlock()

	if previous != nil {
		previous.Stop()
	}
}

// renew performs a background token refresh for the given generation. If the
// renewer has moved on to a newer generation (e.g. due to stop() or a newer
// schedule()) either before or after the network call, the result is discarded.
func (r *tokenAutoRenewer) renew(client *Client, generation int) {
	if !r.isCurrentGeneration(generation) {
		return
	}

	token, exp, err := client.authenticate()

	if !r.isCurrentGeneration(generation) {
		return
	}

	if err != nil || token == "" {
		r.stop()
		return
	}

	client.authMu.Lock()
	client.authToken = token
	client.authTokenExp = exp
	client.authMu.Unlock()

	r.schedule(client, exp)
}

// stop cancels any pending or future renewal, invalidating in-flight callbacks by
// bumping the generation counter.
func (r *tokenAutoRenewer) stop() {
	r.mu.Lock()
	r.generation++
	timer := r.timer
	r.timer = nil
	r.mu.Unlock()

	if timer != nil {
		timer.Stop()
	}
}

func (r *tokenAutoRenewer) isCurrentGeneration(generation int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return generation == r.generation
}

// autoRenewDelay computes how long to wait before renewing a token expiring at exp
// (a Unix timestamp in seconds or milliseconds), buffered so the renewal fires
// slightly ahead of expiration, floored at autoRenewMinDelay.
func autoRenewDelay(exp int64) time.Duration {
	var expiration time.Time
	if exp > 1_000_000_000_000 {
		expiration = time.UnixMilli(exp)
	} else {
		expiration = time.Unix(exp, 0)
	}

	delay := time.Until(expiration) - autoRenewBuffer
	if delay < autoRenewMinDelay {
		return autoRenewMinDelay
	}

	return delay
}
