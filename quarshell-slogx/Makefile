.PHONY: test lint fmt examples pre-release coverage

test:
	go test ./... -race -count=1

lint:
	golangci-lint run --timeout=5m

fmt:
	gofmt -w .

examples:
	@for d in examples/*/; do echo "==> $$d"; go run "./$$d"; done

coverage:
	go list ./... | grep -v '/examples/' | \
	  xargs go test -count=1 -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1

pre-release:
	bash scripts/pre-release-check.sh

pre-release-quick:
	bash scripts/pre-release-check.sh --quick
