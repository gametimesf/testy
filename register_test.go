package testy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCallerPackage(t *testing.T) {
	instance = testy{}

	var pkg string
	// we have to do this in a nested func since it skips an extra caller level
	func() {
		pkg = getCallerPackage()
	}()
	assert.Equal(t, "github.com/gametimesf/testy", pkg)

	// now test with another nested level since anonymous functions have an extra dot in their name
	func() {
		func() {
			pkg = getCallerPackage()
		}()
	}()
	assert.Equal(t, "github.com/gametimesf/testy", pkg)
}

func TestPackageAndFuncNameToPackage(t *testing.T) {
	actual := packageAndFuncNameToPackage("github.com/gametimesf/privaterepo/path/to/packagename.init.1.1")
	assert.Equal(t, "github.com/gametimesf/privaterepo/path/to/packagename", actual)
}

func TestTest(t *testing.T) {
	instance = testy{}

	Test("test", func(t TestingT) {})

	assert.Contains(t, instance.tests, "github.com/gametimesf/testy")
	pkgTests := instance.tests["github.com/gametimesf/testy"]
	assert.Contains(t, pkgTests.tests, "test")

	assert.Panics(t, func() {
		Test("test", func(t TestingT) {})
	})

	assert.Panics(t, func() {
		Test("nil tester", nil)
	})
}

func TestBeforePackage(t *testing.T) {
	instance = testy{}

	BeforePackage(func(t TestingT) {})

	assert.Contains(t, instance.tests, "github.com/gametimesf/testy")
	pkgTests := instance.tests["github.com/gametimesf/testy"]
	// funcs can only be compared to nil, not each other, so even using a known func doesn't help
	assert.NotNil(t, pkgTests.BeforePackage)

	assert.Panics(t, func() {
		BeforePackage(func(t TestingT) {})
	})
}

func TestAfterPackage(t *testing.T) {
	instance = testy{}

	AfterPackage(func(t TestingT) {})

	assert.Contains(t, instance.tests, "github.com/gametimesf/testy")
	pkgTests := instance.tests["github.com/gametimesf/testy"]
	// funcs can only be compared to nil, not each other, so even using a known func doesn't help
	assert.NotNil(t, pkgTests.AfterPackage)

	assert.Panics(t, func() {
		AfterPackage(func(t TestingT) {})
	})
}

func TestBeforeTest(t *testing.T) {
	instance = testy{}

	BeforeTest(func(t TestingT) {})

	assert.Contains(t, instance.tests, "github.com/gametimesf/testy")
	pkgTests := instance.tests["github.com/gametimesf/testy"]
	// funcs can only be compared to nil, not each other, so even using a known func doesn't help
	assert.NotNil(t, pkgTests.BeforeTest)

	assert.Panics(t, func() {
		BeforeTest(func(t TestingT) {})
	})
}

func TestAfterTest(t *testing.T) {
	instance = testy{}

	AfterTest(func(t TestingT) {})

	assert.Contains(t, instance.tests, "github.com/gametimesf/testy")
	pkgTests := instance.tests["github.com/gametimesf/testy"]
	// funcs can only be compared to nil, not each other, so even using a known func doesn't help
	assert.NotNil(t, pkgTests.AfterTest)

	assert.Panics(t, func() {
		AfterTest(func(t TestingT) {})
	})
}
