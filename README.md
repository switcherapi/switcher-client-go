***

<div align="center">
<b>Switcher Client SDK</b><br>
A Go SDK for Switcher API
</div>

<div align="center">

[![Master CI](https://github.com/switcherapi/switcher-client-go/actions/workflows/master.yml/badge.svg)](https://github.com/switcherapi/switcher-client-go/actions/workflows/master.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=switcherapi_switcher-client-go&metric=alert_status)](https://sonarcloud.io/dashboard?id=switcherapi_switcher-client-go)
![Go](https://img.shields.io/badge/go-1.25%2B-blue.svg)
![Status](https://img.shields.io/badge/status-under_development-orange.svg)
![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)
[![Slack: Switcher-HQ](https://img.shields.io/badge/slack-@switcher/hq-blue.svg?logo=slack)](https://switcher-hq.slack.com/)

</div>

***

## Table of Contents

- [Quick Start](#quick-start)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage Examples](#usage-examples)
- [Advanced Features](#advanced-features)
- [Snapshot Management](#snapshot-management)
- [Testing & Development](#testing--development)
- [Contributing](#contributing)

---

## About

The **Switcher Client SDK for Go** provides integration with [Switcher-API](https://github.com/switcherapi/switcher-api), enabling feature flag management in Go applications.

> Features marked as **Under development** are part of the current SDK roadmap and may not be available in the repository yet.

### Key Features

- **Clean & Maintainable**: Simple package-level access with an instance-based core
- **Local Mode**: Offline execution using snapshot files from your Switcher-API domain
- **Silent Mode**: Hybrid configuration with automatic fallback for connectivity issues
- **Built-in Testing Helpers (Under development)**: Test-oriented mocking support adapted for Go
- **Zero Latency**: Local snapshot execution for high-performance scenarios
- **Secure**: Regex protections and configurable remote transport settings
- **Monitoring**: Execution logging, caching, and error notification hooks

## Quick Start

Get up and running in just a few lines of code:

```go
package main

import (
	"fmt"

	"github.com/switcherapi/switcher-client-go"
)

func main() {
	client.BuildContext(client.Context{
		Domain:      "My Domain",
		URL:         "https://api.switcherapi.com",
		APIKey:      "[YOUR_API_KEY]",
		Component:   "MyApp",
		Environment: "default",
	})

	switcher := client.GetSwitcher("FEATURE_TOGGLE")
	if switcher.IsOn() {
		fmt.Println("Feature is enabled!")
	}
}
```

## Installation

Install the Switcher Client SDK:

```bash
go get github.com/switcherapi/switcher-client-go
```

### System Requirements
- **Go**: 1.25+ (targeting 1.25.x and 1.26.x)
- **Operating System**: Cross-platform (Windows, macOS, Linux)

## Configuration

### Basic Setup

Initialize the Switcher Client with your domain configuration:

```go
package main

import (
	"github.com/switcherapi/switcher-client-go"
)

func main() {
	client.BuildContext(client.Context{
		Domain:      "My Domain",                	// Your Switcher domain name
		URL:         "https://api.switcherapi.com", // Switcher-API endpoint (optional)
		APIKey:      "[YOUR_API_KEY]",           	// Your component's API key (optional)
		Component:   "MyApp",                    	// Your application name (optional)
		Environment: "default",                  	// Environment ("default" for production)
	})

	switcher := client.GetSwitcher("FEATURE_LOGIN_V2")
	_ = switcher
}
```

#### Configuration Parameters

| Parameter | Required | Description | Default |
|-----------|----------|-------------|---------|
| `Domain` | ✅ | Your Switcher domain name | - |
| `URL` |  | Switcher-API endpoint | `https://api.switcherapi.com` |
| `APIKey` |  | API key for your component | - |
| `Component` |  | Your application identifier | - |
| `Environment` |  | Target environment | `default` |

### Advanced Configuration

Enable additional features like local mode, silent mode, and transport options:

```go
package main

import (
	"time"

	"github.com/switcherapi/switcher-client-go"
)

func main() {
	client.BuildContext(client.Context{
		Domain:      "My Domain",
		URL:         "https://api.switcherapi.com",
		APIKey:      "[YOUR_API_KEY]",
		Component:   "MyApp",
		Environment: "default",
		Options: client.ContextOptions{
			Local:                      true,
			Logger:                     true,
			Freeze:                     true,
			SnapshotLocation:           "./snapshot/",
			SnapshotAutoUpdateInterval: 3 * time.Second,
			SilentMode:                 5 * time.Minute,
			RestrictRelay:              true,
			ThrottleMaxWorkers:         2,
			RegexMaxBlacklist:          10,
			RegexMaxTimeLimit:          100 * time.Millisecond,
			Remote: client.RemoteOptions{
				CertPath:       "./certs/ca.pem",
				ConnectTimeout: 300 * time.Millisecond,
				ReadTimeout:    5 * time.Second,
				WriteTimeout:   5 * time.Second,
				PoolTimeout:    5 * time.Second,
			},
		},
	})

	switcher := client.GetSwitcher("FEATURE_LOGIN_V2")
	_ = switcher
}
```

#### Advanced Options Reference

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `Local` | `bool` | Use local snapshot files only (zero latency) | `false` |
| `Logger` | `bool` | Enable logging/caching of feature flag evaluations | `false` |
| `Freeze` | `bool` | Enable cache-immutability responses for consistent results | `false` |
| `SnapshotLocation` | `string` | Directory for snapshot files | `""` |
| `SnapshotAutoUpdateInterval` | `time.Duration` | Auto-update interval for snapshots | `0` |
| `SilentMode` | `time.Duration` | Silent mode retry time before returning to remote mode | `0` |
| `RestrictRelay` | `bool` | Enable relay restrictions in local mode | `true` |
| `ThrottleMaxWorkers` | `int` | Max workers for throttling refresh tasks | runtime-defined |
| `RegexMaxBlacklist` | `int` | Max cached entries for failed regex | `100` |
| `RegexMaxTimeLimit` | `time.Duration` | Regex execution time limit | `3s` |
| `Remote` | `RemoteOptions` | Remote transport settings | `RemoteOptions{}` |

`RemoteOptions` fields:

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `CertPath` | `string` | Path to custom certificate for secure API connections | `""` |
| `ConnectTimeout` | `time.Duration` | Max time to establish a remote connection before failing fast | `300ms` |
| `ReadTimeout` | `time.Duration` | Max time to wait for remote response data | `5s` |
| `WriteTimeout` | `time.Duration` | Max time to send remote request data | `5s` |
| `PoolTimeout` | `time.Duration` | Max time to wait for a pooled HTTP connection | `5s` |

**Under development:** transport errors are normalized into typed SDK errors, and silent mode uses the configured remote timeouts to fail fast and switch back to local evaluation.

#### Security Features

- **ReDoS Protection (Under development)**: Regex safety features with bounded execution time
- **Time Limits**: Configurable timeouts for regex and remote operations
- **Certificate Support**: Custom certificates for secure API connections

## Usage Examples

### Basic Feature Flag Checking

The simplest way to check if a feature is enabled:

```go
switcher := client.GetSwitcher("FEATURE_LOGIN_V2")

if switcher.IsOn() {
	newLogin()
} else {
	legacyLogin()
}
```

### Detailed Response Information

Get comprehensive information about the feature flag evaluation:

```go
response := client.GetSwitcher("FEATURE_LOGIN_V2").IsOnWithDetails()

fmt.Printf("Feature enabled: %v\n", response.Result)
fmt.Printf("Reason: %s\n", response.Reason)
fmt.Printf("Metadata: %#v\n", response.Metadata)
```

### Strategy-Based Feature Flags

#### Method 1: Prepare and Execute

Load validation data separately, useful for complex applications:

```go
prepared := client.GetSwitcher("").
	CheckValue("USER_123")

prepared.Prepare("USER_FEATURE")

if prepared.IsOn() {
	enableUserFeature()
}
```

#### Method 2: All-in-One Execution

Chain multiple validation strategies for comprehensive feature control:

```go
isEnabled := client.GetSwitcher("PREMIUM_FEATURES").
	CheckValue("premium_user").
	CheckNetwork("192.168.1.0/24").
	DefaultResult(true).
	Throttle(time.Second).
	IsOn()

if isEnabled {
	showPremiumDashboard()
}
```

### Error Handling

Subscribe to error notifications for robust error management:

```go
client.SubscribeNotifyError(func(err error) {
	fmt.Printf("Switcher Error: %v\n", err)
})
```

## Advanced Features

#### Throttling
```go
client.GetSwitcher("FEATURE01").Throttle(time.Second).IsOn()
```

#### Hybrid Mode
```go
client.GetSwitcher("FEATURE01").Remote().IsOn()
```

## Snapshot Management

### Loading Snapshots

Load snapshots from the API or local files:

```go
version, err := client.LoadSnapshot(nil)
if err != nil {
	panic(err)
}

fmt.Println(version)
```

```go
version, err := client.LoadSnapshot(&client.LoadSnapshotOptions{
	FetchRemote: true,
})
if err != nil {
	panic(err)
}

fmt.Println(version)
```

```go
_, err := client.LoadSnapshot(&client.LoadSnapshotOptions{
	WatchSnapshot: true,
})
if err != nil {
	panic(err)
}
```

### Version Management

Check your current snapshot version:

```go
updated, err := client.CheckSnapshot()
if err != nil {
	panic(err)
}

fmt.Printf("Snapshot updated: %v\n", updated)
fmt.Printf("Current snapshot version: %d\n", client.SnapshotVersion())
```

### Automated Updates

Schedule automatic snapshot updates for zero-latency local mode:

```go
client.ScheduleSnapshotAutoUpdate(time.Minute, func(err error, updated bool) {
	if err != nil {
		fmt.Printf("snapshot update error: %v\n", err)
		return
	}

	if updated {
		fmt.Printf("Snapshot updated to version: %d\n", client.SnapshotVersion())
	}
})
```

### Snapshot Monitoring

```go
err := client.WatchSnapshot(client.WatchSnapshotCallback{
	Success: func() {
		fmt.Println("snapshot loaded successfully")
	},
	Reject: func(err error) {
		fmt.Printf("error loading snapshot: %v\n", err)
	},
})
if err != nil {
	panic(err)
}
```

## Testing & Development

### Built-in Mocking (Under development)

The Go SDK provides test-oriented mocking capabilities adapted to Go idioms and safer state ownership.

```go
sdk := client.NewClient(ctx)
sdk.Assume("FEATURE01").True()

assert.Equal(t, true, sdk.GetSwitcher("FEATURE01").IsOn())
```

```go
sdk.Assume("FEATURE01").True().
	When(client.StrategyValue, []string{"guest", "admin"}).
	When(client.StrategyNetwork, "10.0.0.3")

assert.Equal(t, true, sdk.GetSwitcher("FEATURE01").
	CheckValue("guest").
	CheckNetwork("10.0.0.3").
	IsOn())
```

```go
sdk.Forget("FEATURE01")
```

```go
sdk.Assume("FEATURE01").False().WithMetadata(map[string]any{
	"message": "Feature is disabled",
})

response := sdk.GetSwitcher("FEATURE01").IsOnWithDetails()
assert.Equal(t, false, response.Result)
assert.Equal(t, "Feature is disabled", response.Metadata["message"])
```

### Test Helpers (Under development)

This area is under active development. The helper surface focuses on:

- explicit test helpers instead of decorators
- automatic cleanup helpers for tests where useful
- mock isolation by client instance and, when needed, `context.Context`

### Configuration Validation

Validate your feature flag configuration before deployment:

```go
err := client.CheckSwitchers([]string{
	"FEATURE_LOGIN",
	"FEATURE_DASHBOARD",
	"FEATURE_PAYMENTS",
})
if err != nil {
	fmt.Printf("Configuration error: %v\n", err)
}
```

This validation helps prevent deployment issues by ensuring all required feature flags are properly set up in your Switcher domain.

## Contributing
We welcome contributions to the Switcher Client SDK for Go. If you have suggestions, improvements, or bug fixes, please follow these steps:

1. Fork the repository.
2. Create a new branch for your feature or bug fix.
3. Make your changes and commit them with clear messages.
4. Submit a pull request detailing your changes and the problem they solve.

Thank you for helping us improve the Switcher Client SDK for Go.

### Requirements
- Go 1.25 or higher
- A local Switcher API environment or test fixtures for development
- Standard Go tooling (`go test`, `gofmt`)
- `golangci-lint` for repository lint checks (`make lint-install`)

# AI Disclaimer

This project was ported from [switcherapi/switcher-client-py](https://github.com/switcherapi/switcher-client-py) and adapted for Go using AI-assisted tools. We have thoroughly reviewed and tested all AI-generated contributions to ensure they meet our quality standards and align with our project's goals. We are committed to transparency about our use of AI and will continue to disclose any significant AI contributions in the future. 

External contributions from the community are **equally valued and will be reviewed with the same standards, regardless of whether they were assisted by AI or not**. We encourage all contributors to disclose their use of AI tools in their contributions to maintain transparency and foster trust within our community.
