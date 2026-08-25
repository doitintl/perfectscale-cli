OPENAPI_CODEGEN_VERSION ?= v2.5.0
BINARY_NAME ?= pscli
BUILD_DIR ?= dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

SKILL_DIR ?= plugins
SKILL_NAME ?= perfectscale-skill
SKILL_ZIP ?= $(BUILD_DIR)/$(SKILL_NAME).zip

.PHONY: build openapi generate test skill

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

# Package skill/perfectscale/ as perfectscale-skill.zip with the layout:
#   perfectscale-skill.zip
#   └── perfectscale/
#       ├── SKILL.md
#       ├── agents/openai.yaml
#       ├── references/
#       └── scripts/
skill:
	mkdir -p $(BUILD_DIR)
	rm -f $(SKILL_ZIP)
	cd $(SKILL_DIR) && zip -qr $(abspath $(SKILL_ZIP)) perfectscale -x '*/.DS_Store'
	@echo "Built $(SKILL_ZIP)"
