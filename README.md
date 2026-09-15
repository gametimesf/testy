[![Go Reference](https://pkg.go.dev/badge/github.com/gametimesf/testy.svg)](https://pkg.go.dev/github.com/gametimesf/testy)

# Testy

A Go test running framework.

Please use the reference badge above to find the full documentation of this package.

We couldn't find a framework that addressed our API acceptance testing desires, so we're making one ourselves.
We had some specific design goals in mind when we started this project:
   - Maintain the ability to run tests via `go test` during development of tests, including its ability to run specific tests only.
   - Provide the ability to run tests as a service, so that they may be run on a schedule and as part of CI/CD.
   - The same tests should be able to be run both ways.
   - Tests should be written in a familiar manner.
   - Historical test results should be stored somewhere.

As this is intended for external API acceptance tests,
it is expected that the test code itself does not reside in the same repository as the code under test.
We have the tests for all of our external APIs in a single test repository, even though those tests are testing several different APIs.
This makes it easier to run tests over all external APIs at the same time,
to ensure a change to an internal service that is used by multiple APIs does not cause any such API to fail.

## Examples

Please see the [Example](./example) directory.

## Test-scoped resources

Use `testy.Cleanup(t, cleanup)` immediately after acquiring a resource in a
`Test` or `t.Run` callback. Callbacks run last-in, first-out after subtests, even
when the test fails fatally or panics. A failing or panicking cleanup does not
prevent earlier callbacks from running. Cleanup errors should fail the test;
logging and ignoring them reports a misleading pass.

`testy.Context(t)` returns a context canceled immediately before cleanup starts.
Subtests inherit parent cancellation; native runs also honor the Go test deadline.
Use a separate, bounded context for cleanup I/O. These helpers are optional
capabilities, so existing custom `TestingT` implementations still compile, but
must implement `Cleanup(func())` and `Context() context.Context` to use them.
Legacy Before/After hooks are not resource scopes: migrate resource acquisition
into a Test/Run callback before using the lifecycle helpers.

Use ordinary named `t.Run` cases (or `TestEach`) for repeated contracts. Hosted
execution keeps steps serial within each case; `Parallel` is only effective with
`RunAsTest`. Independent top-level cases can explicitly opt in as described below. Do not make acceptance correctness depend on
subtest scheduling, and do not use a parent's `defer` to release resources needed
by native parallel children. Cleanup waits for those children in native runs.
These callbacks cannot recover resources after process termination; persistent
fixture providers must retain ownership and support external reconciliation.

## Skipped prerequisites

`testy.Skipf(t, "reason: %s", detail)` stops a case whose prerequisites cannot
be established, while running defers and registered cleanup. Use only at explicit
prerequisite boundaries, not to suppress product assertion failures. The native
runner uses Go's skipped status; hosted results retain `SkipReason`, including
when a prior or later failure overrides the skip. `testy.Skipped(t)` lets cleanup
adapters identify an explicit skip; it does not mean that the test has not failed.
These optional capabilities do not add methods to `TestingT`.

Hosted skipped leaves are `skipped`; nonfailed containers with skipped coverage
are `incomplete`, never `passed`. `SumTestStatsWithSkipped` returns total, passed,
failed and skipped counts. The existing `SumTestStats` signature is preserved,
but its total includes skipped tests without counting them as passed. Result
consumers must handle incomplete coverage explicitly before enabling skips.
Any `Fail`, `Errorf`, fatal error or panic, including during cleanup, remains a
failure. A prerequisite-specific best-effort cleanup policy must report warnings
explicitly instead of weakening these generic failure semantics.

## Bounded independent cases

Register an independent case with `ConcurrentTest(name, tester)` instead of
`Test`. This is an author declaration that its fixture IDs, mutations and hooks
are safe to overlap with other opted-in cases. Never split dependent steps merely
to get concurrency: `t.Run` children still execute sequentially in hosted mode.

Create **one** `executor := testy.NewCaseExecutor(2)` per hosted worker and share
it in `RunOptions{CaseExecutor: executor}` across all `RunWithContext` or
`RunPackageWithOptions` calls. A case owns admission from `BeforeTest` through
body, children, cleanup and `AfterTest`. Non-opted-in cases and package hooks take
exclusive admission, preventing overlap with other cases in the same executor.
Package teardown waits for all of its admitted work. Hooks themselves are still
legacy non-resource scopes; migrate fixture acquisition into case callbacks.

A nil executor preserves existing serial-in-package behavior and existing package
concurrency. An executor of size one serializes all shared case lifecycles.
Native `RunAsTest` uses Go's `-parallel` limit for `ConcurrentTest`; existing manual
`Parallel` semantics are unchanged. Native `AfterPackage` now runs in enclosing
Go test cleanup, after parallel children and their cleanup, without changing test
names. A native bootstrap should contain one registered package, as before.

`RunPackageWithOptions(ctx, packageName, opts)` propagates caller cancellation,
then **waits** for admitted cases, their cleanup and hooks before returning
`ctx.Err()`. It cannot preempt non-cooperative callbacks or clean up after a killed
process. Cleanup I/O needs a fresh bounded context. Infra retries can still repeat
side effects; orchestration must not claim exactly-once execution.

The executor is not a fleet-wide limit. Deployment must bound simultaneous worker
replicas, surge and other test runners before increasing capacity. Keep a serial
rollback and drain old workers before enabling overlap. `ListCases` exposes a
copy of canonical case identities/indices and opt-in flags; reports retain this
order regardless of completion order. `TestResult.QueueDur` records delay from
case discovery after package setup to admission; `Dur` covers the admitted case
lifecycle, including its test hooks, separately from that queue delay.

A hosted caller may supply `RunOptions.CaseOrder[pkg]` as an immutable complete
permutation of current case names (for example, a duration-based snapshot recorded
by an external workflow). Testy does no history I/O and reads no scheduling clock.
This changes admission order only: returned cases retain canonical indices/names,
children remain ordered, and the shared executor still covers each entire lifecycle.
Missing hints use canonical order. Invalid/duplicate/stale hints are ignored as a
whole with a package diagnostic, never by dropping or duplicating selected tests.
