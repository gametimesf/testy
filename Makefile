GO := go1.18beta2

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test: # lint # TODO add lint back when golangci-lint supports type parameters
	${GO} vet ./...
	${GO} test --race --cover $$(${GO} list ./...)

.PHONY: docs
docs:
	${GO} generate $$(${GO} list ./...)

.PHONY: tidy
tidy:
	${GO} mod tidy

.PHONY: test-coverage
test-coverage: lint
	${GO} vet ./...
	COVEROUT=$$(mktemp)
	${GO} test -coverprofile="$COVEROUT" $$(go list ./...)
	${GO} tool cover -html="$COVEROUT"
	rm -f "$COVEROUT"

.PHONY: fmt
fmt:
	${GO} fmt ./...
	find . -name '*.go' -exec gci -w -local github.com/gametimesf/testy {} \; > /dev/null
