GO ?= go

.PHONY: fmt vet test build check

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

build:
	mkdir -p bin
	$(GO) build -o bin/docktor ./cmd/docktor

check:
	@files=$$(gofmt -l cmd internal); if [ -n "$$files" ]; then printf 'Files need gofmt:\n%s\n' "$$files"; exit 1; fi
	$(GO) vet ./...
	$(GO) test ./...
	$(MAKE) build
