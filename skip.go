package testy

// Skipf records why this test cannot execute and stops it with runtime.Goexit,
// running its defers and registered cleanup. A prior or later failure always
// overrides the skipped outcome. Call only from the test's goroutine.
//
// Skipping is an optional capability, preserving existing TestingT implementations.
// Custom runners must implement Skipf(string, ...interface{}); unsupported runners
// panic rather than silently report unexecuted assertions as passed. Legacy
// Before/After hooks are not test scopes and cannot skip.
func Skipf(t TestingT, format string, args ...interface{}) {
	t.Helper()
	skipper, ok := t.(interface{ Skipf(string, ...interface{}) })
	if !ok {
		panic("testy: TestingT does not support Skipf")
	}
	skipper.Skipf(format, args...)
}

// Skipped reports whether the test requested a skip, including during cleanup.
// This does not mean it passed or cannot also have failed. Custom runners must
// implement Skipped() bool to use this optional capability.
func Skipped(t TestingT) bool {
	skipper, ok := t.(interface{ Skipped() bool })
	if !ok {
		panic("testy: TestingT does not support Skipped")
	}
	return skipper.Skipped()
}
