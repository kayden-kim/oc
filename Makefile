.PHONY: build test release-check snapshot clean

VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)
GORELEASER_VERSION ?= v2.14.3
GORELEASER ?= go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

ifeq ($(OS),Windows_NT)
  EXT := .exe
else
  EXT :=
endif

build:
	go build -ldflags="$(LDFLAGS)" -o oc$(EXT) ./cmd/oc

test:
	go test ./...

release-check:
	$(GORELEASER) check

snapshot: clean
	$(GORELEASER) release --snapshot --clean

clean:
	rm -rf dist/
