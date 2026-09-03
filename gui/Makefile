# Development tasks for the GUI module. Run `make help` for the list.
#
# This module is cgo-based (Fyne needs it), unlike the root CLI
# module — see the root README's "How the GUI works" section.

BINARY  := pomodoro-gui
PKG     := github.com/christopherwc/productivio/gui
VERSION := $(shell git -C .. describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w

# Pinned, not @latest — see the vulncheck target below.
GOVULNCHECK_VERSION := v1.7.0

export CGO_ENABLED := 1

.PHONY: help build run test vet fmt fmt-check tidy tidy-check verify vulncheck check clean

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build:  ## Build ./bin/pomodoro-gui
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

run: build  ## Build and launch the GUI
	./bin/$(BINARY)

test:  ## Run the tests (headless — see internal/ui's use of fyne.io/fyne/v2/test)
	go test ./...

vet:  ## Run go vet
	go vet ./...

fmt:  ## Format the source
	gofmt -s -w .

fmt-check:  ## Fail if anything is unformatted
	@unformatted=$$(gofmt -s -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt -s -w:"; echo "$$unformatted"; exit 1; \
	fi

tidy:  ## Tidy go.mod
	go mod tidy

tidy-check: tidy  ## Fail if go.mod/go.sum are not tidy
	@if [ -n "$$(git status --porcelain -- go.mod go.sum)" ]; then \
		echo "go.mod is not tidy; run 'make tidy'"; exit 1; \
	fi

verify:  ## Verify downloaded modules match go.sum
	go mod verify

vulncheck:  ## Scan for known vulnerabilities (pinned version, not @latest)
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

check: fmt-check vet test tidy-check verify vulncheck  ## Everything CI runs

clean:  ## Remove build and test artefacts
	rm -rf bin
