PREFIX ?= $(HOME)/.local

.PHONY: build install test bench clean

build:
	go build -trimpath -ldflags "-s -w" -o termo ./cmd/termo

install: build
	install -Dm755 termo $(PREFIX)/bin/termo

test:
	go vet ./...
	go test ./...

bench:
	go test ./internal/engine -run XXX -bench . -benchtime 300x

clean:
	rm -f termo
