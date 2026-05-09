package client

import "time"

const (
	DefaultEnvironment          = "default"
	DefaultLocal                = false
	DefaultLogger               = false
	DefaultFreeze               = false
	DefaultRestrictRelay        = true
	DefaultRegexMaxBlacklist    = 100
	DefaultRegexMaxTimeLimit    = 3 * time.Second
	DefaultRemoteConnectTimeout = 300 * time.Millisecond
	DefaultRemoteReadTimeout    = 5 * time.Second
	DefaultRemoteWriteTimeout   = 5 * time.Second
	DefaultRemotePoolTimeout    = 5 * time.Second
)

type RemoteOptions struct {
	CertPath       string
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	PoolTimeout    time.Duration
}

type ContextOptions struct {
	Local                      bool
	Logger                     bool
	Freeze                     bool
	SnapshotLocation           string
	SnapshotAutoUpdateInterval time.Duration
	SilentMode                 time.Duration
	RestrictRelay              bool
	ThrottleMaxWorkers         int
	RegexMaxBlacklist          int
	RegexMaxTimeLimit          time.Duration
	Remote                     RemoteOptions
}

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

	if o.RegexMaxBlacklist == 0 {
		o.RegexMaxBlacklist = DefaultRegexMaxBlacklist
	}

	if o.RegexMaxTimeLimit == 0 {
		o.RegexMaxTimeLimit = DefaultRegexMaxTimeLimit
	}

	o.Remote = o.Remote.withDefaults()
	return o
}

func (o RemoteOptions) withDefaults() RemoteOptions {
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = DefaultRemoteConnectTimeout
	}

	if o.ReadTimeout == 0 {
		o.ReadTimeout = DefaultRemoteReadTimeout
	}

	if o.WriteTimeout == 0 {
		o.WriteTimeout = DefaultRemoteWriteTimeout
	}

	if o.PoolTimeout == 0 {
		o.PoolTimeout = DefaultRemotePoolTimeout
	}

	return o
}
