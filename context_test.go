package client

import "testing"

func TestContextWithOptionals(t *testing.T) {
	BuildContext(Context{
		Domain: "My Domain",
		Options: ContextOptions{
			Local:            true,
			SnapshotLocation: "./tests/snapshots",
		},
	})

	options := defaultClient().context.Options

	if !options.Local {
		t.Fatalf("expected local option to be true")
	}

	if options.SnapshotLocation != "./tests/snapshots" {
		t.Fatalf("expected snapshot location to be preserved, got %q", options.SnapshotLocation)
	}
}

func TestBuildContextCreatesFreshDefaultOptions(t *testing.T) {
	BuildContext(Context{Domain: "First Domain"})
	firstClient := defaultClient()

	firstClient.mu.Lock()
	firstClient.context.Options.Local = true
	firstClient.mu.Unlock()

	BuildContext(Context{Domain: "Second Domain"})
	secondClient := defaultClient()

	if firstClient == secondClient {
		t.Fatalf("expected BuildContext to replace the default client")
	}

	if secondClient.context.Options.Local {
		t.Fatalf("expected second client to receive fresh default options")
	}
}

func TestBuildContextAppliesDefaults(t *testing.T) {
	BuildContext(Context{
		Domain: "My Domain",
	})

	ctx := defaultClient().context

	if ctx.Environment != DefaultEnvironment {
		t.Fatalf("expected default environment %q, got %q", DefaultEnvironment, ctx.Environment)
	}

	if !ctx.Options.RestrictRelay {
		t.Fatalf("expected restrict relay default to be true")
	}

	if ctx.Options.RegexMaxBlacklist != DefaultRegexMaxBlacklist {
		t.Fatalf("expected regex max blacklist default %d, got %d", DefaultRegexMaxBlacklist, ctx.Options.RegexMaxBlacklist)
	}

	if ctx.Options.RegexMaxTimeLimit != DefaultRegexMaxTimeLimit {
		t.Fatalf("expected regex max time limit default %v, got %v", DefaultRegexMaxTimeLimit, ctx.Options.RegexMaxTimeLimit)
	}

	if ctx.Options.Remote.ConnectTimeout != DefaultRemoteConnectTimeout {
		t.Fatalf("expected default connect timeout %v, got %v", DefaultRemoteConnectTimeout, ctx.Options.Remote.ConnectTimeout)
	}

	if ctx.Options.Remote.ReadTimeout != DefaultRemoteReadTimeout {
		t.Fatalf("expected default read timeout %v, got %v", DefaultRemoteReadTimeout, ctx.Options.Remote.ReadTimeout)
	}

	if ctx.Options.Remote.WriteTimeout != DefaultRemoteWriteTimeout {
		t.Fatalf("expected default write timeout %v, got %v", DefaultRemoteWriteTimeout, ctx.Options.Remote.WriteTimeout)
	}

	if ctx.Options.Remote.PoolTimeout != DefaultRemotePoolTimeout {
		t.Fatalf("expected default pool timeout %v, got %v", DefaultRemotePoolTimeout, ctx.Options.Remote.PoolTimeout)
	}
}
