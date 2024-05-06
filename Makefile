.PHONY: all
all: pico

.PHONY: pico
pico:
	mkdir -p bin
	go build -o bin/pico main.go

.PHONY: unit-test
unit-test:
	go test ./... -v

.PHONY: integration-test
integration-test:
	go test ./... -tags integration -v

.PHONY: system-test
system-test:
	go test ./tests -tags system -v

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	go vet ./...
	golangci-lint run

.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out -tags integration
	go tool cover -html=coverage.out

.PHONY: image
image:
	docker build . -f build/Dockerfile -t pico:$(shell git rev-parse HEAD)
	docker tag pico:$(shell git rev-parse HEAD) pico:latest
