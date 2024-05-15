.PHONY: all
all: piko

.PHONY: piko
piko:
	mkdir -p bin
	go build -o bin/piko main.go

.PHONY: unit-test
unit-test:
	go test ./... -v

.PHONY: integration-test
integration-test:
	go test ./... -tags integration -v

.PHONY: system-test
system-test:
	go test ./tests -tags system -v

.PHONY: test-all
test-all:
	$(MAKE) unit-test
	$(MAKE) integration-test
	$(MAKE) system-test

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
	docker build . -f build/Dockerfile -t piko:$(shell git rev-parse HEAD)
	docker tag piko:$(shell git rev-parse HEAD) piko:latest
