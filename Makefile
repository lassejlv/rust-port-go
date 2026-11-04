GO ?= go

.PHONY: all build test check demo clean
all: build
build:
	mkdir -p bin
	$(GO) build -o bin/ferrugo ./cmd/ferrugo
	$(GO) build -o bin/ferrugo-cargo ./cmd/ferrugo-cargo

test:
	$(GO) test ./...

check:
	$(GO) vet ./...
	$(GO) test ./...

demo: build
	./bin/ferrugo-cargo run --manifest-path examples/hello/Cargo.toml
	./bin/ferrugo-cargo run --manifest-path examples/local-packages/app/Cargo.toml

clean:
	rm -f bin/ferrugo bin/ferrugo-cargo
