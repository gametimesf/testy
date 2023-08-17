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
