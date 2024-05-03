.PHONY: all
all: pico

.PHONY: pico
pico:
	mkdir -p build
	go build -o build/pico main.go

.PHONY: unit-test
unit-test:
	go test ./... -v

.PHONY: integration-test
integration-test:
	go test ./... -tags integration -v

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	go vet ./...
	golangci-lint run

.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out
