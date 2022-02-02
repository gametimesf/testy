.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test: lint
	go vet ./...
	go test --race --cover $$(go list ./...)

.PHONY: docs
docs:
	go generate $$(go list ./...)

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: test-coverage
test-coverage: lint
	go vet ./...
	COVEROUT=$$(mktemp)
	go test -coverprofile="$COVEROUT" $$(go list ./...)
	go tool cover -html="$COVEROUT"
	rm -f "$COVEROUT"

.PHONY: fmt
fmt:
	go fmt ./...
	find . -name '*.go' -exec gci -w -local github.com/gametimesf/testy {} \; > /dev/null
