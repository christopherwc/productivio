# Common development tasks. Run `make help` for the list.

BINARY  := pomodoro
PKG     := github.com/your-username/pomodoro
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/internal/cli.Version=$(VERSION)

# No cgo anywhere: it is what keeps the binary static and makes
# cross-compilation a matter of two environment variables.
export CGO_ENABLED := 0

.PHONY: help build install run test race cover cover-html vet lint fmt fmt-check tidy check release clean

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build:  ## Build ./bin/pomodoro
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

install:  ## Install into $GOBIN
	go install -ldflags "$(LDFLAGS)" ./cmd/$(BINARY)

run: build  ## Build and show today's status
	./bin/$(BINARY) status

test:  ## Run the tests
	go test ./...

race:  ## Run the tests under the race detector
	go test -race ./...

cover:  ## Report test coverage
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out | tail -1

cover-html: cover  ## Open a line-by-line coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html"

vet:  ## Run go vet
	go vet ./...

lint:  ## Run staticcheck, if installed
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./... ; \
	else \
		echo "staticcheck not installed; skipping."; \
		echo "  go install honnef.co/go/tools/cmd/staticcheck@latest"; \
	fi

fmt:  ## Format the source
	gofmt -s -w .

fmt-check:  ## Fail if anything is unformatted
	@unformatted=$$(gofmt -s -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt -s -w:"; echo "$$unformatted"; exit 1; \
	fi

tidy:  ## Tidy go.mod
	go mod tidy

check: fmt-check vet race cover lint  ## Everything CI runs

release:  ## Cross-compile for every supported platform
	@mkdir -p dist
	@for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		echo "  building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" \
			-o dist/$(BINARY)-$$os-$$arch ./cmd/$(BINARY) || exit 1; \
	done
	@ls -lh dist/

clean:  ## Remove build and test artefacts
	rm -rf bin dist coverage.out coverage.html
