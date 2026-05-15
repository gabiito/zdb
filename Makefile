.PHONY: build test test-integration lint fmt run release release-clean

build:
	CGO_ENABLED=0 go build -o bin/zdb ./cmd/zdb

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

run:
	go run ./cmd/zdb

# Cross-compile release binaries for every supported platform. Output lands
# in dist/ (gitignored) alongside a SHA256SUMS file. Run from a clean
# tag-pointed main checkout so --version reports the tag, not a pseudo-version.
release:
	@mkdir -p dist
	@echo "-> Cross-compiling release binaries..."
	@for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		os=$$(echo $$target | cut -d/ -f1); \
		arch=$$(echo $$target | cut -d/ -f2); \
		printf "  -> %s/%s\n" $$os $$arch; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build \
			-ldflags="-s -w" \
			-o dist/zdb-$$os-$$arch \
			./cmd/zdb; \
	done
	@cd dist && sha256sum zdb-* > SHA256SUMS
	@echo "-> Done. Artifacts in dist/:"
	@ls -lh dist/

release-clean:
	rm -rf dist/
