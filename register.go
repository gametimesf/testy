package testy

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/gametimesf/testy/internal/orderedmap"
)

// It is assumed that all funcs in this file are called during package initialization,
// and as such there is a language guarantee that only one such func will be running at a time,
// so we don't need to synchronize access to the maps containing packages or tests.
//
// For more details, see https://go.dev/ref/spec#Package_initialization

// Test registers a new test to be run.
// Tests are run in lexicographical order within a package.
//
// The return value may be discarded (and is always nil); it is provided to simplify writing test code, like so:
//
//	var _ = testy.Test("my test", func(t testy.TestingT){})
func Test(name string, tester Tester) any {
	if tester == nil {
		panic(fmt.Sprintf("test %s has nil test function", name))
	}

	name = strings.Map(stripName, name)
	pkg := getCallerPackage()
	pkgTests := getPackageTests(pkg)

	if _, exists := pkgTests.tests[name]; exists {
		panic(fmt.Sprintf("test %s already exists in package %s", name, pkg))
	}

	pkgTests.tests[name] = testCase{
		Package: pkg,
		Name:    name,
		tester:  tester,
	}

	return nil
}

// BeforePackage registers a function to be run once before any tests in the given package are run.
// A package may only have one BeforePackage function.
//
// If BeforePackage panics, no tests in the package will be run and the reporting behavior depends on if RunAsTest or Run was called.
// AfterPackage will still be run.
//
// If RunAsTest was called, the panic will be reported as the top-level bootstrap test for the package.
// If Run was called, every registered test in the package will be marked as failed with the panic's message.
//
// When run via Run, any logging output to the provided Tester will only be visible if BeforePackage panics or the Tester is marked as failed.
// When run via RunAsTest, the standard `go test` output rules apply.
// Notably, if a test fails, the output is not visible via Run but is via RunAsTest.
//
// NOTE: Do not call TestingT.Fail, TestingT.FailNow, TestingT.Fatal, or TestingT.Fatalf in BeforePackage to report errors; always panic.
// It will work as expected in RunAsTest mode, but not Run mode.
// TODO parity here.
//
// The return value may be discarded (and is always nil); it is provided to simplify writing test code, like so:
//
//	var _ = testy.BeforePackage(func(){})
func BeforePackage(f Tester) any {
	pkg := getCallerPackage()
	pkgTests := getPackageTests(pkg)

	if pkgTests.BeforePackage != nil {
		panic(fmt.Sprintf("package %s already has a BeforePackage", pkg))
	}

	pkgTests.BeforePackage = f
	return nil
}

// AfterPackage registers a function to be run once after all tests in the given package have finished.
// A package may only have one AfterPackage function.
//
// If AfterPackage panics, the reporting behavior depends on if RunAsTest or Run was called.
// If AfterPackage panics and BeforePackage had already panicked, then the panic for BeforePackage takes priority.
//
// If RunAsTest was called, the panic will be reported as the top-level bootstrap test for the package.
// If Run was called, every test in the package will be marked as failed,
// and the panic's message will be appended to every test's own messages or the BeforePackage panic that replaced every test.
//
// When run via Run, any logging output to the provided Tester will only be visible if AfterPackage panics or the Tester is marked as failed.
// When run via RunAsTest, the standard `go test` output rules apply.
// Notably, if a test fails, the output is not visible via Run but is via RunAsTest.
//
// NOTE: Do not call TestingT.Fail, TestingT.FailNow, TestingT.Fatal, or TestingT.Fatalf in AfterPackage to report errors; always panic.
// It will work as expected in RunAsTest mode, but not Run mode.
// TODO parity here.
//
// The return value may be discarded (and is always nil); it is provided to simplify writing test code, like so:
//
//	var _ = testy.AfterPackage(func(){})
func AfterPackage(f Tester) any {
	pkg := getCallerPackage()
	pkgTests := getPackageTests(pkg)

	if pkgTests.AfterPackage != nil {
		panic(fmt.Sprintf("package %s already has an AfterPackage", pkg))
	}

	pkgTests.AfterPackage = f
	return nil
}

// BeforeTest registers a function to be run before every top level registered test in the given package is run.
// It is not run before subtests created by `t.Run`.
// A package may only have one BeforeTest function.
//
// If BeforeTest panics, the specific registered test that was about to be invoked will be marked as failed with the panic's message.
// AfterTest will still be run.
//
// When run via Run, any logging output to the provided Tester will only be visible if BeforeTest panics or the Tester is marked as failed.
// When run via RunAsTest, the standard `go test` output rules apply.
// Notably, if a test fails, the output is not visible via Run but is via RunAsTest.
//
// NOTE: Do not call TestingT.Fail, TestingT.FailNow, TestingT.Fatal, or TestingT.Fatalf in BeforeTest to report errors; always panic.
// It will work as expected in RunAsTest mode, but not Run mode.
// TODO parity here.
//
// The return value may be discarded (and is always nil); it is provided to simplify writing test code, like so:
//
//	var _ = testy.BeforeTest(func(){})
func BeforeTest(f Tester) any {
	pkg := getCallerPackage()
	pkgTests := getPackageTests(pkg)

	if pkgTests.BeforeTest != nil {
		panic(fmt.Sprintf("package %s already has a BeforeTest", pkg))
	}

	pkgTests.BeforeTest = f
	return nil
}

// AfterTest registers a function to be run once after every top level registered test in the given package has finished.
// It is not run after subtests created by `t.Run`.
// A package may only have one AfterTest function.
//
// If AfterTest panics, the specific test that was just run will be marked as failed,
// and the panic's message will be appended to the test's own messages.
// Any subtests of the test will not be modified.
//
// When run via Run, any logging output to the provided Tester will only be visible if AfterTest panics or the Tester is marked as failed.
// When run via RunAsTest, the standard `go test` output rules apply.
// Notably, if a test fails, the output is not visible via Run but is via RunAsTest.
//
// NOTE: Do not call TestingT.Fail, TestingT.FailNow, TestingT.Fatal, or TestingT.Fatalf in AfterTest to report errors; always panic.
// It will work as expected in RunAsTest mode, but not Run mode.
// TODO parity here.
//
// The return value may be discarded (and is always nil); it is provided to simplify writing test code, like so:
//
//	var _ = testy.AfterTest(func(){})
func AfterTest(f Tester) any {
	pkg := getCallerPackage()
	pkgTests := getPackageTests(pkg)

	if pkgTests.AfterTest != nil {
		panic(fmt.Sprintf("package %s already has an AfterTest", pkg))
	}

	pkgTests.AfterTest = f
	return nil
}

func getCallerPackage() string {
	// we only care about our immediate caller's immediate caller
	callers := make([]uintptr, 1)
	// skip over Callers, ourselves, and our caller
	n := runtime.Callers(3, callers[:])
	pkg := "<unknown>"
	// make sure it was able to actually get our caller
	if n > 0 {
		frames := runtime.CallersFrames(callers)
		frame, _ := frames.Next()
		pkg = packageAndFuncNameToPackage(frame.Function)
	}
	return pkg
}

func packageAndFuncNameToPackage(full string) string {
	// skip to after the package.
	i := strings.LastIndex(full, "/")
	// remove the function name (which is almost certainly "init") and leave just the package name.
	// as an example, this function is `github.com/gametimesf/testy.RegisterTest`.
	// we cannot just do LastIndex for . because anonymous functions will end up with multiple . in their name
	j := strings.Index(full[i:], ".")
	return full[:i+j]
}

// getPackageTests ensures that a testPkg instance exists for the provided package and returns it
func getPackageTests(pkg string) *testPkg {
	if instance.tests == nil {
		instance.tests = make(orderedmap.OrderedMap[string, *testPkg])
	}

	if instance.tests[pkg] == nil {
		instance.tests[pkg] = &testPkg{
			name:  pkg,
			tests: make(orderedmap.OrderedMap[string, testCase]),
		}
	}

	return instance.tests[pkg]
}
