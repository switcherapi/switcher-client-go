# Architecture

This document describes the design of the **Switcher Client SDK for Go**, from the
public API surface down to the individual files that implement it. The package is
intentionally flat (`package client`, no internal sub-packages), so this document
exists to make the relationships between files explicit.

## Table of Contents

- [1. Design Goals](#1-design-goals)
- [2. API Surface Map (Feature → Code)](#2-api-surface-map-feature--code)
- [3. High-Level Architecture](#3-high-level-architecture)
- [4. Core Components](#4-core-components)
- [5. Execution Flow (`IsOn` / `IsOnWithDetails`)](#5-execution-flow-ison--isonwithdetails)
- [6. Execution Modes: Local, Remote, Silent](#6-execution-modes-local-remote-silent)
- [7. Snapshot Lifecycle](#7-snapshot-lifecycle)
- [8. Authentication & Token Auto-Renewal](#8-authentication--token-auto-renewal)
- [9. Throttling (Stale-While-Revalidate)](#9-throttling-stale-while-revalidate)
- [10. Mocking / Test Support](#10-mocking--test-support)
- [11. Error Model](#11-error-model)
- [12. Concurrency Model](#12-concurrency-model)
- [13. File-by-File Reference](#13-file-by-file-reference)
- [14. Design Patterns Used](#14-design-patterns-used)
- [15. Extension Points](#15-extension-points)

---

## 1. Design Goals

- **Package-level ergonomics with instance-based core**: `client.GetSwitcher(...)` and
  friends are thin wrappers around a `*Client` instance, so simple apps can use the
  package API directly while tests/multi-tenant apps can create isolated `*Client`
  instances via `NewClient`.
- **Zero-latency option**: full feature evaluation against an in-memory/on-disk
  snapshot, no network required (`Options.Local`).
- **Resilience by default**: remote failures can automatically fall back to a local
  snapshot ("Silent Mode" / circuit breaker) instead of failing requests.
- **Safe concurrency**: the `Client` is shared, mutable state (snapshot, auth token,
  switcher cache, mocks, execution log) is guarded by targeted mutexes rather than one
  global lock.
- **Testability**: SDK behavior can be mocked per-client without global state leakage
  between tests.

## 2. API Surface Map (Feature → Code)

This maps README.md features to the files that implement them.

| README Feature | Public Entry Points | Primary Files |
|---|---|---|
| Client bootstrap (`BuildContext`, `NewClient`) | `client.BuildContext`, `client.NewClient` | `client.go`, `context.go` |
| Basic feature flag check | `GetSwitcher(key).IsOn()` | `client.go`, `switcher.go` |
| Detailed response (`IsOnWithDetails`) | `Switcher.IsOnWithDetails` | `switcher.go`, `result.go` |
| Must-variant with default (`IsOnOrDefault`) | `Switcher.IsOnOrDefault`, `IsOnWithDetailsOrDefault` | `switcher.go` |
| Strategy-based flags (`Check*` fluent chain) | `Switcher.Check`, `CheckValue`, `CheckNumeric`, `CheckDate`, `CheckTime`, `CheckPayload`, `CheckNetwork`, `CheckRegex` | `switcher.go` (API), `local_strategies.go` (evaluation) |
| Prepare/Execute pattern | `Switcher.Prepare` | `switcher.go` |
| Error notifications | `SubscribeNotifyError` | `client.go` |
| Throttling (stale-while-revalidate) | `Switcher.Throttle` | `switcher.go`, `execution_logger.go` |
| Hybrid mode (force remote) | `Switcher.Remote` | `switcher.go` |
| Circuit breaker / Silent Mode | `Options.SilentMode` | `client_silent_mode.go` |
| Snapshot loading | `LoadSnapshot` | `client.go`, `snapshot.go` |
| Snapshot version check | `CheckSnapshot`, `SnapshotVersion` | `client.go`, `remote.go` |
| Snapshot auto-update | `ScheduleSnapshotAutoUpdate`, `TerminateSnapshotAutoUpdate` | `snapshot_auto_updater.go` |
| Snapshot file watch | `WatchSnapshot`, `UnwatchSnapshot` | `snapshot_watcher.go` |
| Switcher configuration validation | `CheckSwitchers` | `client.go`, `remote.go` (remote path), `resolver.go` (local path) |
| Execution log inspection | `GetExecution`, `ClearLogger` | `client.go`, `execution_logger.go` |
| Built-in mocking | `Client.Assume`, `Client.Forget` | `mock.go` |
| Remote transport / certs | `Options.Remote` | `remote.go` |
| Auth token auto-renewal | `Options.Remote.AutoRenewToken` | `client_auto_renew.go`, `remote.go` |

## 3. High-Level Architecture

```
                          ┌─────────────────────────────┐
                          │   Application Code (user)   │
                          └──────────────┬──────────────┘
                                         │ package-level API
                                         ▼
                          ┌─────────────────────────────┐
                          │   client.go (facade layer)  │  BuildContext / GetSwitcher /
                          │   defaultClient() singleton │  LoadSnapshot / CheckSnapshot ...
                          └──────────────┬──────────────┘
                                         │ delegates to
                                         ▼
                          ┌─────────────────────────────────┐
                          │        *Client (core)           │
                          │  context.go   – configuration   │
                          │  client.go    – switcher cache, │
                          │                 execution log,  │
                          │                 throttle tokens │
                          └───┬─────────┬─────────┬─────────┘
                              │         │         │
             ┌────────────────┘         │         └────────────────┐
             ▼                          ▼                          ▼
   ┌───────────────────┐     ┌───────────────────┐      ┌────────────────────┐
   │   switcher.go     │     │   remote.go       │      │  snapshot.go /     │
   │  Switcher (fluent │     │  HTTP transport,  │      │  resolver.go       │
   │  API + execution  │◄────┤  auth, criteria,  │      │  snapshot state,   │
   │  orchestration)   │     │  snapshot fetch   │      │  local evaluation  │
   └─────────┬─────────┘     └─────────┬─────────┘      └──────────┬─────────┘
             │                         │                           │
             ▼                         ▼                           ▼
   ┌─────────────────────┐     ┌───────────────────┐      ┌────────────────────┐
   │ execution_logger.go │     │ client_silent_    │      │ local_strategies.go│
   │ (throttle cache/    │     │ mode.go           │      │ (per-strategy      │
   │  logging)           │     │ client_auto_      │      │  criteria engine)  │
   │                     │     │ renew.go          │      │                    │
   └─────────────────────┘     └───────────────────┘      └────────────────────┘

   Cross-cutting: mock.go (test overrides), errors.go (typed errors),
   result.go (ResultDetail), snapshot_watcher.go / snapshot_auto_updater.go (background jobs)
```

Everything funnels through **`*Client`**, which is the aggregate root holding
configuration (`Context`), cached `*Switcher` instances, the current `*Snapshot`,
auth token state, the execution/throttle log, and background workers (snapshot
watcher, snapshot auto-updater, token auto-renewer).

## 4. Core Components

| Type | File | Responsibility |
|---|---|---|
| `Client` | `client.go` | Aggregate root: owns context, switcher cache, snapshot, mocks, execution logger, throttle token pool, auth state, background workers, HTTP client. |
| `Context` / `ContextOptions` / `RemoteOptions` | `context.go` | Immutable-ish configuration value objects with `withDefaults()` normalization. |
| `Switcher` | `switcher.go` | Fluent, per-key evaluator. Holds the strategy `entries` for one evaluation, throttle state, and orchestrates `submit → resolveExecutionMode → executeMode`. |
| `Snapshot` / `SnapshotDomain` / `SnapshotGroup` / `SnapshotConfig` / `SnapshotStrategy` / `SnapshotRelay` | `snapshot.go` | Value objects mirroring the Switcher-API domain data model, (de)serialized to/from JSON snapshot files. |
| `resolver.go` (unexported functions) | `resolver.go` | Pure local evaluation engine: walks `Snapshot → Domain → Group → Config → Strategies` and returns a `ResultDetail`. |
| `local_strategies.go` | `local_strategies.go` | Per-strategy-type predicate evaluation (VALUE, NUMERIC, DATE, TIME, PAYLOAD, NETWORK, REGEX) plus `Operation` constants. |
| `remote.go` | `remote.go` | All outbound HTTP calls: `/criteria/auth`, `/criteria`, `/criteria/switchers_check`, `/criteria/snapshot_check/{version}`, `/graphql`, `/check`. Builds the shared `*http.Client`/`*http.Transport` (TLS, timeouts, cert). |
| `client_silent_mode.go` | `client_silent_mode.go` | Circuit-breaker: decides when to force local evaluation after remote failures and manages the sentinel `SILENT` auth token. |
| `client_auto_renew.go` (`tokenAutoRenewer`) | `client_auto_renew.go` | Background timer that refreshes the auth token slightly before expiry. |
| `snapshot_watcher.go` (`snapshotWatcher`) | `snapshot_watcher.go` | Polls the snapshot file on disk for changes and hot-reloads it. |
| `snapshot_auto_updater.go` (`snapshotAutoUpdater`) | `snapshot_auto_updater.go` | Periodically calls `CheckSnapshot` on a ticker. |
| `execution_logger.go` (`executionLogger`) | `execution_logger.go` | Stores the last `ResultDetail` per (key, inputs) pair; backs both `GetExecution` and throttling's cached-result path. |
| `mock.go` (`MockAssumption`, `mockDefinition`) | `mock.go` | Per-client test mocking (`Assume`/`Forget`), with optional per-strategy `When(...)` conditions. |
| `errors.go` | `errors.go` | Typed error hierarchy (`RemoteError` family, `LocalCriteriaError`, `LocalSwitcherError`) so callers can `errors.As` on failure category. |
| `result.go` | `result.go` | `ResultDetail` — the canonical evaluation outcome (`Result`, `Reason`, `Metadata`). |
| `client_snapshot.go` | `client_snapshot.go` | Small `Client` snapshot-state helpers (`snapshotState`, `setSnapshot`, `shouldCheckSnapshot`, `loadSnapshotFromCurrentFile`, `stopBackgroundTasks`). |

## 5. Execution Flow (`IsOn` / `IsOnWithDetails`)

Both public methods funnel into `Switcher.submit(showDetails bool)` in `switcher.go`:

```
Switcher.IsOn() / IsOnWithDetails()
        │
        ▼
   Switcher.submit(showDetails)
        │
        ├─ 1. snapshotForExecution()          // clone key+entries+throttle state (race-safe snapshot of *this* call)
        │
        ├─ 2. client.mockedResult(execution)  // mock.go — short-circuit if Client.Assume(key) was set
        │        └─ found → log + return
        │
        ├─ 3. tryCachedResult(execution)      // only when Throttle(...) was set — execution_logger.go lookup
        │        └─ hit → optionally scheduleBackgroundRefresh(), return cached ResultDetail
        │
        └─ 4. execute(execution, showDetails)
                 │
                 ├─ resolveExecutionMode()     // Local | SilentLocal | Remote  (see §6)
                 ├─ executeMode(mode, ...)     // dispatch to executeLocal() or executeRemote()
                 └─ logResult(result)          // execution_logger.go, only if Logger option or Throttle is set
```

`snapshotForExecution` matters: a `*Switcher` returned by `GetSwitcher` is a
long-lived, cached, mutable object (its `entries`/`throttlePeriod` can be changed by
further `Check*`/`Throttle` calls). Each evaluation call takes an immutable clone so
concurrent calls to the same cached `Switcher` don't race on `entries`.

## 6. Execution Modes: Local, Remote, Silent

`Switcher.resolveExecutionMode()` picks one of three modes:

1. **`executionModeLocal`** — `Context.Options.Local == true`. Skips remote
   validation entirely (no URL/APIKey/Component required) and evaluates directly
   against the in-memory `*Snapshot` via `resolver.go`.
2. **`executionModeRemote`** — default when not in local mode: validates the
   `Switcher`/`Context` (`Switcher.Validate`), obtains an auth token
   (`Client.ensureToken`, `remote.go`), and calls the `/criteria` endpoint.
3. **`executionModeSilentLocal`** — entered automatically when `client_silent_mode.go`
   detects the client is already in a silent-mode window (`shouldUseLocalSilentMode`)
   — i.e., a prior remote failure caused a fallback and the `SilentMode` cool-down
   period has not elapsed. Evaluates locally exactly like local mode.

**Fallback on remote failure** (`executeRemote` → `fallbackToSilentMode` in
`client_silent_mode.go`): if `ensureToken` or `checkCriteria` fails and
`Options.SilentMode > 0`, the client:
   - notifies the error callback (`notifyError`),
   - stamps `authToken = "SILENT"` with an expiry `now + SilentMode`
     (`updateSilentToken`),
   - immediately serves the request from the local snapshot.

While in that window, `shouldUseLocalSilentMode()` periodically probes
`/check` (`checkAPIHealth`) once the silent window has technically expired; if the
API is healthy again it clears the silent token and resumes remote mode, otherwise it
re-arms another silent window.

This makes Silent Mode a **circuit breaker with local-snapshot fallback** rather than
a simple retry: local snapshot data must be self-sufficient (no Relay dependency) for
switchers evaluated in this mode — see the `RestrictRelay` check in `resolver.go`.

**Hybrid Mode override** (`Switcher.Remote(force ...bool)`, `switcher.go`): a per-call
opt-out from local execution. Setting `forceRemote = true` on a `Switcher` (the
default when calling `Remote()` with no arguments; `Remote(false)` clears it) changes
`resolveExecutionMode`'s first branch from `Options.Local` to
`Options.Local && !forceRemote`, so a forced switcher always falls through to the
`Validate` → token → `executionModeRemote` (or `executionModeSilentLocal`, if a
silent-mode window is active) path even while the client is otherwise configured for
Local Mode. `Validate()` requires `Options.Local == true` whenever `forceRemote` is
set — calling `Remote()` while local mode is disabled has no effect on mode
selection (the client is already remote-only) but does yield a
`"something went wrong: local mode is not enabled"` error from `Validate`, `Prepare`,
`IsOn`, and `IsOnWithDetails`.

## 7. Snapshot Lifecycle

```
LoadSnapshot(options)                                    (client.go)
   │
   ├─ loadSnapshotFromCurrentFile()  ─── snapshot.go: loadSnapshotFromFile()
   │      reads <SnapshotLocation>/<Environment>.json, or creates an empty
   │      default snapshot (version 0) if the file/location doesn't exist yet
   │
   ├─ shouldCheckSnapshot(FetchRemote)?  (client_snapshot.go)
   │      true when version == 0 AND (FetchRemote requested OR not in Local mode)
   │      └─ CheckSnapshot()                              (client.go / remote.go)
   │             ensureToken() → checkSnapshotVersion(token, currentVersion)
   │             │  GET /criteria/snapshot_check/{version}
   │             └─ if stale → resolveSnapshot(token)      POST /graphql (domain query)
   │                    → saveSnapshotToFile()  (snapshot.go, only if SnapshotLocation set)
   │                    → setSnapshot(...)       (client_snapshot.go, in-memory swap)
   │
   └─ WatchSnapshot requested? → snapshot_watcher.go starts polling the file
```

Two independent, composable mechanisms keep a local snapshot fresh:

- **`ScheduleSnapshotAutoUpdate`** (`snapshot_auto_updater.go`): a ticker goroutine
  that calls `Client.CheckSnapshot()` (pull-based, talks to the remote API).
- **`WatchSnapshot`** (`snapshot_watcher.go`): a ticker goroutine that `os.Stat`s the
  snapshot *file* every 100ms and reloads it in-memory on change (push-based from an
  external process writing the file, e.g. another SDK instance or a sidecar).

Both are started/stopped independently and both are stopped together via
`Client.stopBackgroundTasks()` when `BuildContext` replaces the global client.

## 8. Authentication & Token Auto-Renewal

- `Client.ensureToken()` (`remote.go`) is the single lazy-auth entry point used by
  every remote call. It returns the cached token if non-empty, non-`"SILENT"`, and
  unexpired; otherwise it calls `authenticate()` (`POST /criteria/auth`) and caches
  the result (`authToken`, `authTokenExp`, guarded by `authMu`).
- When `Options.Remote.AutoRenewToken` is true, a successful `ensureToken()` schedules
  `tokenAutoRenewer.schedule()` (`client_auto_renew.go`): a `time.AfterFunc` fires
  `autoRenewBuffer` (5s) before expiry and calls `authenticate()` again in the
  background, re-scheduling itself — so foreground requests never pay synchronous
  re-auth latency once warmed up.
- A **generation counter** in `tokenAutoRenewer` invalidates in-flight/scheduled
  renewals when `stop()` is called (e.g., entering Silent Mode or replacing the
  client), preventing a stale background renewal from clobbering newer state.

## 9. Throttling (Stale-While-Revalidate)

`Switcher.Throttle(period)` (`switcher.go`) opts a switcher into SWR semantics:

1. First call has no cache entry → executes normally, then `logResult` unconditionally
   stores the result in `executionLogger` (throttled switchers log even when
   `Options.Logger` is false — see `Switcher.canLog()`).
2. Subsequent calls within the throttle window: `tryCachedResult` finds the entry in
   `executionLogger` and returns it immediately.
3. Once `nextRefreshAt` has passed (`shouldScheduleRefresh`) and `Options.Freeze` is
   not set, a background refresh is fired via `Client.runBackgroundTask` (bounded by
   `throttleTokens`, a buffered channel sized by `Options.ThrottleMaxWorkers`) which
   re-executes and re-logs, updating the cache for the *next* caller.
4. If `Options.Freeze` is true, the cached value is pinned until an explicit
   `client.ClearLogger()`.

Cached responses are tagged with `Metadata["cached"] = true`
(`execution_logger.go: cachedResultDetail`) so callers can distinguish fresh vs.
stale-served results (see README's `GetExecution(...).Response.Metadata["cached"]`
example).

## 10. Mocking / Test Support

`mock.go` implements client-scoped mocking so tests don't need a real snapshot or
remote API:

- `Client.Assume(key)` creates a `mockDefinition` in `Client.mocks` (guarded by
  `mockMu`, separate from the main `mu` used for context/snapshot/switchers) and
  returns a `*MockAssumption` fluent builder (`True()`, `False()`, `When(strategy,
  input)`, `WithMetadata(...)`, `Cleanup(t)`).
- `Switcher.submit()` checks `Client.mockedResult(execution)` **before** cache lookup
  or real evaluation — mocks take precedence over everything, including Throttle.
- `mockDefinition.responseFor(entries)` supports conditional mocks: if a `When(...)`
  condition's strategy input doesn't match what was actually supplied via
  `Check*(...)`, the mock's boolean result is **inverted** and a descriptive mismatch
  reason is generated (`mismatchMockReason`). This lets tests assert both the
  "matching" and "non-matching" branches from a single `Assume(...).When(...)` setup.
- Mocks are strictly per-`Client` instance — no process-global mock registry — so
  parallel tests using `client.NewClient(...)` don't interfere with each other.

## 11. Error Model

`errors.go` defines two error families surfaced by the SDK:

- **Remote errors** — all embed `RemoteError` (base `message string` + `Error()`):
  `RemoteAuthError`, `RemoteCriteriaError`, `RemoteSnapshotError`,
  `RemoteSwitcherError`. Constructed via `newRemote*Error(...)` helpers in
  `remote.go`/`errors.go`. Callers can `errors.As(err, &client.RemoteAuthError{})`
  to branch on failure category (e.g., to distinguish "bad API key" from "network
  unavailable").
- **Local errors** — `LocalCriteriaError` (snapshot missing / config or key not
  found during local evaluation, from `resolver.go`) and `LocalSwitcherError`
  (`CheckSwitchers` found missing keys locally).
- `missingTokenError(token)` (`remote.go`) is a plain `errors.New` guard used after
  every `ensureToken()` call site to convert an empty token into an explicit error
  without a network round-trip.

## 12. Concurrency Model

`Client` uses **multiple fine-grained mutexes** instead of one lock, to avoid
serializing unrelated operations:

| Mutex | Guards |
|---|---|
| `Client.mu` (`RWMutex`) | `context`, `switchers` map, `snapshot` |
| `Client.mockMu` (`RWMutex`) | `mocks` map |
| `Client.authMu` (`Mutex`) | `authToken`, `authTokenExp` |
| `Client.httpClientMu` (`Mutex`) | lazily-built `httpClient_` |
| `Client.notifyErrorMu` (`RWMutex`) | `notifyErrorCallback` |
| `Switcher.mu` (`RWMutex`) | `entries`, `throttlePeriod`, `nextRefreshAt` on a cached `*Switcher` |
| `tokenAutoRenewer.mu`, `snapshotWatcher.mu`, `snapshotAutoUpdater.mu` | each background worker's own timer/stop/done/generation state |

Background work (throttle refresh) is dispatched through
`Client.runBackgroundTask`, which — when `Options.ThrottleMaxWorkers > 0` — bounds
concurrency using a buffered channel (`throttleTokens`) acting as a worker-pool
semaphore; with the default (`0`), refreshes just spawn an unbounded goroutine.

The package-level API is backed by a single global `*Client` stored in an
`atomic.Pointer[Client]` (`globalClient` in `client.go`), swapped atomically by
`BuildContext` (old client's background tasks are stopped before being discarded).

## 13. File-by-File Reference

| File | Role |
|---|---|
| `client.go` | Public facade + `Client` struct + switcher cache + snapshot/switcher-check orchestration + global default-client singleton. |
| `context.go` | `Context`/`ContextOptions`/`RemoteOptions` config types and defaulting. |
| `switcher.go` | `Switcher` fluent API + evaluation orchestration (mock → cache → execute). |
| `resolver.go` | Pure local criteria engine (Snapshot → Domain → Group → Config → Strategies). |
| `local_strategies.go` | Strategy/Operation constants + per-strategy predicate implementations. |
| `remote.go` | HTTP client construction, auth, criteria/snapshot/switchers remote calls. |
| `remote_transport_test.go` | Tests for the TLS/transport configuration in `remote.go`. |
| `snapshot.go` | `Snapshot` data model + file (de)serialization. |
| `client_snapshot.go` | `Client` snapshot-state accessors used by `client.go`/watchers. |
| `client_silent_mode.go` | Circuit-breaker fallback logic + silent-mode token sentinel. |
| `client_auto_renew.go` | Background token auto-renewal (`tokenAutoRenewer`). |
| `snapshot_watcher.go` | File-polling snapshot hot-reload worker. |
| `snapshot_auto_updater.go` | Ticker-based periodic `CheckSnapshot` worker. |
| `execution_logger.go` | Cache of last `ResultDetail` per (key, inputs); backs throttling + `GetExecution`. |
| `mock.go` | Per-client test mocking (`Assume`/`Forget`/`MockAssumption`). |
| `result.go` | `ResultDetail` value type. |
| `errors.go` | Typed error hierarchy for remote/local failures. |
| `client_check_switchers_test.go`, `client_test.go`, `switcher_test.go`, `switcher_throttle_test.go`, `local_test.go`, `snapshot_test.go`, `snapshot_checker_test.go`, `snapshot_watcher_test.go`, `client_auto_renew_test.go`, `client_silent_mode_test.go`, `execution_logger_test.go`, `context_test.go`, `mock_test.go`, `remote_test.go` | Corresponding unit tests, generally one test file per production file above. |
| `testdata/` | Fixture snapshot JSON files used by tests. |

## 14. Design Patterns Used

- **Facade**: package-level functions in `client.go` (`GetSwitcher`, `LoadSnapshot`,
  `CheckSnapshot`, ...) are thin facades over `*Client` methods, backed by a lazily
  initialized singleton (`defaultClient()`).
- **Builder / Fluent Interface**: `Switcher.Check*` methods and `MockAssumption`
  (`True/False/When/WithMetadata`) both return `*Switcher`/`*MockAssumption` for
  chaining, mirroring the README's chained-strategy examples.
- **Strategy Pattern**: `local_strategies.go` dispatches evaluation by
  `SnapshotStrategy.Strategy` to independent `process*Strategy` predicate functions,
  keyed by `Operation` sub-dispatch.
- **State / Circuit Breaker**: `client_silent_mode.go` models three effective client
  states (healthy-remote, silent/local-fallback, recovering) driven by
  `authToken == "SILENT"` + expiry, transitioning via `checkAPIHealth`.
- **Decorator-like execution wrapping**: `Switcher.submit` layers mocking →
  throttle-cache → real execution → logging around the same core "evaluate"
  operation without those concerns knowing about each other.
- **Observer**: `SubscribeNotifyError`/`notifyError` is a single-callback observer
  used to surface async/background errors (silent-mode fallback, throttle
  background-refresh failures) to the caller without requiring a return value.
- **Object Pool via buffered channel**: `throttleTokens` implements a bounded
  worker-pool for background throttle refreshes.
- **Value Objects with immutable-copy semantics**: `Switcher.snapshotForExecution`,
  `cloneMetadata`, `cloneResultDetail`, `mockDefinition.clone` all defensively copy
  mutable maps/slices before crossing a concurrency boundary, avoiding shared mutable
  state across goroutines.

## 15. Extension Points

- **New strategy type**: add a `Strategy*` constant, a `process*Strategy` function in
  `local_strategies.go`, and wire it into `processLocalStrategy`'s switch — plus a
  `Check*` convenience method in `switcher.go` if it should be part of the fluent
  chain.
- **New remote endpoint**: add a request/response struct and method in `remote.go`
  following the existing `doJSONRequest` + typed-error pattern, then expose it via a
  `Client` method (and optional package-level facade) in `client.go`.
- **New client-wide background worker**: follow the `stop`/`done` channel pattern used
  by `snapshotWatcher`/`snapshotAutoUpdater` (start/stop idempotent, `Stop()` blocks
  until the goroutine exits) and register it in `Client.stopBackgroundTasks()`.
- **New error category**: add a type embedding `RemoteError` (or a standalone local
  error type) plus a `new*Error` constructor in `errors.go`, so callers can
  `errors.As` on it.
