.PHONY: all
all: pico

.PHONY: pico
pico:
	mkdir -p build
	go build -o build/pico main.go

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	go vet ./...
	golangci-lint run
