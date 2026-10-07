# moca — build, test and release.
#
# `go` is not on the PATH on every machine that works on this repo (mise pins
# Go in mise.toml); GORUN falls back to `mise x go -- go` when no go binary is
# found, so `make build` works either way.
GORUN := $(shell command -v go >/dev/null 2>&1 && echo go || echo "mise x go -- go")
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS := -s -w -X github.com/adeotek/moca/internal/config.Version=$(VERSION)

.PHONY: build test vet fmt release clean

build:
	$(GORUN) build -trimpath -ldflags "$(LDFLAGS)" -o bin/moca ./cmd/moca

test:
	$(GORUN) test ./... -race -count=1

vet:
	$(GORUN) vet ./...

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
