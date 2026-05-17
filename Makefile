.PHONY: test fmt cover cover-html lint lint-install godoc-install docs

GOLANGCI_LINT_VERSION=v2.12.2

test-clean:
	go clean -testcache
	
test:
	go test -p 1 -v ./...

fmt:
	gofmt -s -w .

lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint:
	golangci-lint run

docs:
	go doc -http

cover:
	go test -p 1 -coverprofile="coverage.out" ./...

cover-html:
	go tool cover -html="coverage.out"
