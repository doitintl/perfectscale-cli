OPENAPI_CODEGEN_VERSION ?= v2.5.0
BINARY_NAME ?= pscli
BUILD_DIR ?= dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build openapi generate test

build:
	mkdir -p $(BUILD_DIR)
	go build \
		-trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) \
		.

openapi: internal/publicapi/client.gen.go

internal/publicapi/client.gen.go: public-api.yaml
	mkdir -p internal/publicapi
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OPENAPI_CODEGEN_VERSION) \
		-generate types,client \
		-package publicapi \
		-o internal/publicapi/client.gen.go \
		public-api.yaml

generate: openapi

test:
	go test ./...
