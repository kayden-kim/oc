.PHONY: build test build-all clean release

VERSION ?= v0.1.1
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags="$(LDFLAGS)" -o oc ./cmd/oc

test:
	go test ./...

build-all: clean
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/oc-darwin-arm64 ./cmd/oc
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/oc-darwin-amd64 ./cmd/oc
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/oc-windows-amd64.exe ./cmd/oc

clean:
	rm -rf dist/

release: build-all
	@echo "Creating GitHub release $(VERSION)..."
	gh release create $(VERSION) dist/* --title "$(VERSION)" --generate-notes
