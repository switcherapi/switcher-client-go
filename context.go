package client

import "time"

// Default configuration constants used when fields are not provided in Context or options.
const (
	DefaultEnvironment          = "default"
	DefaultLocal                = false
	DefaultLogger               = false
	DefaultFreeze               = false
	DefaultRestrictRelay        = true
	DefaultRemoteConnectTimeout = 300 * time.Millisecond
	DefaultRemoteTimeout        = 5 * time.Second
)

// RemoteOptions configures remote transport behavior, timeouts and certificate path.
type RemoteOptions struct {
	CertPath       string
	AutoRenewToken bool
	ConnectTimeout time.Duration
	Timeout        time.Duration
}

// ContextOptions exposes advanced SDK behaviors such as local mode, snapshot management,
// throttling. See README advanced configuration for details.
type ContextOptions struct {
	Local                      bool
	Logger                     bool
	Freeze                     bool
	SnapshotLocation           string
	SnapshotAutoUpdateInterval time.Duration
	SilentMode                 time.Duration
	RestrictRelay              bool
	ThrottleMaxWorkers         int
	Remote                     RemoteOptions
}

// Context contains domain and environment-specific configuration for a Client.
// BuildContext accepts this value to configure the package default client.
type Context struct {
	Domain      string
	URL         string
	APIKey      string
	Component   string
	Environment string
	Options     ContextOptions
}

func (c Context) withDefaults() Context {
	if c.Environment == "" {
		c.Environment = DefaultEnvironment
	}

	c.Options = c.Options.withDefaults()
	return c
}

func (o ContextOptions) withDefaults() ContextOptions {
	if !o.RestrictRelay {
		o.RestrictRelay = DefaultRestrictRelay
	}

	o.Remote = o.Remote.withDefaults()
	return o
}

func (o RemoteOptions) withDefaults() RemoteOptions {
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = DefaultRemoteConnectTimeout
	}

	if o.Timeout == 0 {
		o.Timeout = DefaultRemoteTimeout
	}

	return o
}
