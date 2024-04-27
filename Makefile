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

.PHONY: generate
generate:
	protoc --go_out=. --go_opt=paths=source_relative api/rpc.proto

.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out
