.PHONY: build clean

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o build/ralph ./cmd/ralph

clean:
	rm -rf build/
