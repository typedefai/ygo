SERVICE		?= $(shell basename `go list`)
VERSION		?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || cat $(PWD)/.version 2> /dev/null || echo v0)
PACKAGE		?= $(shell go list)
PACKAGES	?= $(shell go list ./...)
FILES		?= $(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: help clean fmt lint vet test gen conformance

# Regenerate the reference fixtures/vectors from yjs + lib0 (requires bun).
gen:
	bun run testutil/gen_lib0_vectors.js
	bun run testutil/gen_fixtures.js
	bun run testutil/gen_merge_fixtures.js
	bun run testutil/gen_diff_fixtures.js
	bun run testutil/gen_ops_fixtures.js
	bun run testutil/gen_protocol_fixtures.js

# Byte/semantic conformance against the yjs reference implementation.
conformance: gen
	go test ./internal/lib0/ ./pkg/ygo/ -run 'Conformance|TestYjs' -v
	bun run testutil/verify_go_updates.js

clean:
	go clean -cache

fmt:
	go fmt ./...
	goimports -w $(FILES)

lint:
	golint $(PACKAGES)

vet:
	go vet ./...

test:
	go run gotest.tools/gotestsum@latest
