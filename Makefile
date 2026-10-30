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
	go test -race -covermode=atomic -coverpkg=./... -coverprofile=cover.out ./...

# Fails if any statement in the profile never ran, or if the profile has no
# blocks at all. It reads the profile's blocks rather than the total from
# `go tool cover -func`, because that total is rounded (99.95% prints as
# 100.0%) and leaves out function literals that sit outside named functions.
cover: test
	@awk 'NR > 1 { stmts[$$1] = $$2; runs[$$1] += $$3 } \
	END { \
		if (NR <= 1) { print "coverage: the profile has no blocks"; exit 1 } \
		for (b in stmts) if (stmts[b] > 0 && runs[b] == 0) { print "not covered: " b; bad = 1 } \
		if (!bad) print "coverage: 100% of statements"; \
		exit bad \
	}' cover.out
