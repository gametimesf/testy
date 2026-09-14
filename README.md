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
execution can run packages concurrently and remains serial inside each package; `Parallel` is
only effective with `RunAsTest`. Do not make acceptance correctness depend on
subtest scheduling, and do not use a parent's `defer` to release resources needed
by native parallel children. Cleanup waits for those children in native runs.
These callbacks cannot recover resources after process termination; persistent
fixture providers must retain ownership and support external reconciliation.
