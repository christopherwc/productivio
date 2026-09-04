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

.PHONY: help build run release test vet fmt fmt-check tidy tidy-check verify vulncheck check clean

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build:  ## Build ./bin/pomodoro-gui
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

run: build  ## Build and launch the GUI
	./bin/$(BINARY)

# Unlike the CLI's release target, this cannot cross-compile from one
# machine: Fyne needs cgo, and cgo needs a C toolchain for the actual
# target platform, not just a GOOS/GOARCH env var. Each host builds
# only what it can natively — Linux builds linux/amd64, and Apple
# Silicon macOS builds both darwin/arm64 and darwin/amd64 via clang's
# built-in multi-arch support. linux/arm64 is a known gap: it needs
# either a native arm64 runner or a cross-gcc + sysroot, neither of
# which is set up yet.
release:  ## Build for every target this host's toolchain can reach
	@mkdir -p dist
	@os=$$(go env GOOS); \
	case "$$os" in \
		linux) \
			echo "  building linux/amd64"; \
			GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" \
				-o dist/$(BINARY)-linux-amd64 ./cmd/$(BINARY) || exit 1 ;; \
		darwin) \
			for arch in arm64 amd64; do \
				echo "  building darwin/$$arch"; \
				GOOS=darwin GOARCH=$$arch go build -ldflags "$(LDFLAGS)" \
					-o dist/$(BINARY)-darwin-$$arch ./cmd/$(BINARY) || exit 1; \
			done ;; \
		*) \
			echo "release: no target configured for host OS $$os"; exit 1 ;; \
	esac
	@ls -lh dist/

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
