// Package template_library is a template repository for building a new Golang library.
//
// Overview
//
// Please go through the following files and make the following changes after copying this code into your new
// repository:
// 1. doc.go
//   - Change the package name in both locations and _actually write documentation_.
// 2. go.mod
//   - Change the package import path.
// 3. Makefile
//   - Change the package import path for the fmt target.
// 4. .golangci.yml
//   - Change the package import path for the gci.local-prefixes setting.
// 5. .travis.yml
//   - remove the docker login step, if you don't need it; otherwise, add DOCKERHUB_TOKEN to travis pipeline
//
// Documentation
//
// Automatically updating README.md requires having Go 1.16 installed, and installing godoc2md.
//
//  go install github.com/morganhein/godoc2md
//  go generate
package template_library

//go:generate bash -c "godoc2md github.com/gametimesf/template_library > README.md"
