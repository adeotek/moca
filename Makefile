# moca — build, test and release.
#
# `go` is not on the PATH on every machine that works on this repo (the Go
# version is pinned in go.mod's `toolchain` directive, which mise reads);
# GORUN falls back to `mise x go -- go` when no go binary is found, so
# `make build` works either way.
#
# The targets below are mirrored as mise tasks in mise.toml (`mise run
# <task>`) — keep the two in sync.
GORUN := $(shell command -v go >/dev/null 2>&1 && echo go || echo "mise x go -- go")
GOFMT := $(shell command -v gofmt >/dev/null 2>&1 && echo gofmt || echo "mise x go -- gofmt")
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS := -s -w -X github.com/adeotek/moca/internal/config.Version=$(VERSION)

.PHONY: build test vet fmt release clean

build:
	$(GORUN) build -trimpath -ldflags "$(LDFLAGS)" -o bin/moca ./cmd/moca

test:
	$(GORUN) test ./... -race -count=1

vet:
	$(GORUN) vet ./...
	@out="$$($(GOFMT) -l .)"; if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi

fmt:
	$(GORUN) fmt ./...

# Cross-compiled release binaries (no cgo): dist/moca-<version>-<os>-<arch>[.exe]
release:
	@mkdir -p dist
	@for p in linux/amd64 linux/arm64 darwin/arm64 darwin/amd64 windows/amd64; do \
	  os=$${p%/*}; arch=$${p#*/}; ext=$$( [ $$os = windows ] && echo .exe ); \
	  GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GORUN) build -trimpath -ldflags "$(LDFLAGS)" \
	    -o dist/moca-$(VERSION)-$$os-$$arch$$ext ./cmd/moca || exit 1; \
	done

clean:
	rm -rf bin dist
