# Every check CI runs. `make check` before pushing.

STATICCHECK := honnef.co/go/tools/cmd/staticcheck@v0.8.2
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0

.PHONY: check lint fmt tidy vet staticcheck govulncheck test cover

check: lint cover

lint: fmt tidy vet staticcheck govulncheck

fmt:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

tidy:
	go mod tidy -diff

vet:
	go vet ./...

staticcheck:
	go run $(STATICCHECK) ./...

govulncheck:
	go run $(GOVULNCHECK) ./...

test:
	go test -race -covermode=atomic -coverprofile=cover.out ./...

# Fails unless every statement is covered. A profile with no blocks means the
# module has no statements yet, which is the only way to pass without 100.0%.
cover: test
	@if [ "$$(grep -vc '^mode:' cover.out)" -eq 0 ]; then echo "coverage: no statements"; exit 0; fi; \
	go tool cover -func=cover.out | tail -n 1; \
	go tool cover -func=cover.out | grep -Eq '^total:[[:space:]]+\(statements\)[[:space:]]+100\.0%$$' || \
	{ echo "coverage is below 100%:"; go tool cover -func=cover.out | grep -v '100\.0%$$'; exit 1; }
