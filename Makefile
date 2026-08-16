CRAP_SOURCES := . internal/app internal/commit internal/git internal/provider

.PHONY: check ci workflow-check fmt-check lint vet test build race coverage mutation crap vuln no-cgo cross-build release-check snapshot clean

check: workflow-check lint vet test build release-check
	go mod tidy -diff
	go mod verify

ci: check race crap no-cgo cross-build vuln snapshot

workflow-check:
	@command -v actionlint >/dev/null 2>&1 || { echo "actionlint is required"; exit 1; }
	actionlint

fmt-check:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint v2 is required"; exit 1; }
	@golangci-lint version | grep -Eq 'version 2\.' || { echo "golangci-lint v2 is required"; exit 1; }
	golangci-lint fmt --diff

lint: fmt-check
	golangci-lint run ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	go build ./...

race:
	go test -race ./...

coverage:
	go test -race -coverprofile=coverage.out ./...

mutation:
	./scripts/mutation.sh
	@if [ -f mutation-results.json ] && \
		grep -Eq '"status":"(LIVED|TIMED OUT)"' mutation-results.json; then \
		echo "Mutation testing left lived or timed-out mutants."; \
		exit 1; \
	fi

crap:
	@report="$$(go run github.com/unclebob/crap4go/cmd/crap4go@v0.0.0-20260521190544-bee16dbdadb4 $(CRAP_SOURCES))" || { \
		exit_code="$$?"; \
		printf '%s\n' "$$report"; \
		exit "$$exit_code"; \
	}; \
	printf '%s\n' "$$report"; \
	printf '%s\n' "$$report" | awk \
		'$$(NF - 1) ~ /%$$/ && $$NF ~ /^[0-9]+([.][0-9]+)?$$/ && ($$NF + 0) >= 15 { failed = 1 } END { exit failed }' || { \
		echo "Every CRAP score must be below 15."; \
		exit 1; \
	}

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...

no-cgo:
	CGO_ENABLED=0 go test ./...
	CGO_ENABLED=0 go build ./...

cross-build:
	@set -eu; \
	for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64; do \
		os="$${target%/*}"; \
		arch="$${target#*/}"; \
		CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" go build ./...; \
	done

release-check:
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser is required"; exit 1; }
	goreleaser check

snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser is required"; exit 1; }
	goreleaser build --snapshot --clean

clean:
	rm -rf dist target coverage.out mutation-results.json
