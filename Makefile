IMAGE_TAG ?= $(shell git rev-parse HEAD)
VERSION ?= $(shell git describe)

.PHONY: all
all: piko

.PHONY: piko
piko:
	mkdir -p bin
	go build -ldflags="-X github.com/andydunstall/piko/pkg/build.Version=$(VERSION)" -o bin/piko main.go

.PHONY: inline-test
inline-test:
	go test ./... -v

.PHONY: system-test
system-test:
	go test ./tests/... -tags system -v -count 1

.PHONY: system-test-short
system-test-short:
	go test ./tests/... -tags system -v -count 1 -test.short

.PHONY: test
test:
	$(MAKE) inline-test
	$(MAKE) system-test

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	go vet ./...
	golangci-lint run

.PHONY: import
import:
	goimports -w -local github.com/andydunstall/piko .

.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

.PHONY: image
image:
	docker build --build-arg version=$(VERSION) . -f build/Dockerfile -t piko:$(IMAGE_TAG)
	docker tag piko:$(IMAGE_TAG) piko:latest
