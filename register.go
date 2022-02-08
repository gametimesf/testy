package testy

import (
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/gametimesf/testy/orderedmap"
)

var regLock sync.Mutex

// RegisterTest registers a new test to be run. Tests are run in lexicographical order within a package. The return
// value may be discarded (and is always nil); it is provided to simplify writing test code, like so:
//
//    var _ = testy.RegisterTest("my test", func(t testy.TestingT){})
func RegisterTest(name string, tester Tester) any {
	// we only care about our immediate caller
	callers := make([]uintptr, 1)
	// skip over Callers and ourselves
	n := runtime.Callers(2, callers[:])
	pkg := "<unknown>"
	// make sure it was able to actually get our caller
	if n > 0 {
		frames := runtime.CallersFrames(callers)
		frame, _ := frames.Next()
		// remove the function name (which is almost certainly "init") and leave just the package name.
		// as an example, this function is `github.com/gametimesf/testy.RegisterTest`.
		i := strings.LastIndex(frame.Function, ".")
		pkg = frame.Function[:i]
	}

	regLock.Lock()
	defer regLock.Unlock()
	if instance.tests == nil {
		instance.tests = make(orderedmap.OrderedMap[string, orderedmap.OrderedMap[string, testCase]])
	}
	if instance.tests[pkg] == nil {
		instance.tests[pkg] = make(orderedmap.OrderedMap[string, testCase])
	}

	if _, exists := instance.tests[pkg][name]; exists {
		panic(fmt.Sprintf("test %s already exists in package %s", name, pkg))
	}

	instance.tests[pkg][name] = testCase{
		Package: pkg,
		Name:    name,
		tester:  tester,
	}

	return nil
}
