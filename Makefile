IMAGE := mcp-monday-projects:local
COMPOSE := docker compose -f compose.yaml
TOOL ?= server_info
ARGS ?= {}
SMOKE_BOARD ?=
SMOKE_WORKSPACE ?=
SMOKE_USER ?=
SMOKE_REPORT ?= smoke-report.md

.PHONY: help build up run down logs test race fmt fmt-check vet validate clean probe tools smoke smoke-provision docs-tools

help: ## Show available Docker-first targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

build: ## Build the production image
	docker build -t $(IMAGE) .

up: build ## Start the Compose MCP service
	$(COMPOSE) up -d mcp

run: build ## Run the stdio MCP server interactively
	$(COMPOSE) run --rm mcp

down: ## Stop Compose services
	$(COMPOSE) down

logs: ## Follow Compose logs
	$(COMPOSE) logs -f mcp

test: ## Run all Go tests inside the builder container
	$(COMPOSE) --profile test run --rm test

race: ## Run race-enabled tests inside the Docker race builder
	docker build --target race-builder -t $(IMAGE)-race-builder .
	docker run --rm $(IMAGE)-race-builder sh -c 'CGO_ENABLED=1 go test -race ./...'

fmt-check: ## Fail if any Go file needs gofmt
	docker build --target builder -t $(IMAGE)-builder .
	docker run --rm $(IMAGE)-builder sh -c 'test -z "$$(gofmt -l .)"'

fmt: ## Rewrite Go files with gofmt (runs in Docker, no host Go needed)
	docker build --target builder -t $(IMAGE)-builder .
	@for f in $$(docker run --rm $(IMAGE)-builder gofmt -l .); do \
		docker run --rm $(IMAGE)-builder gofmt "$$f" > "$$f.fmt" && mv "$$f.fmt" "$$f" && echo "formatted $$f"; \
	done

probe: build ## Call one tool against the real account: make probe TOOL=get_me ARGS='{}'
	python3 scripts/mcp_probe.py $(TOOL) '$(ARGS)'

tools: build ## List registered tools with read/write mode
	python3 scripts/mcp_probe.py --list

smoke: build ## Real-account smoke suite: make smoke SMOKE_BOARD=<sandbox board> [SMOKE_USER=<user id>]
	python3 scripts/smoke.py $(if $(SMOKE_BOARD),--board $(SMOKE_BOARD)) $(if $(SMOKE_USER),--assign-user $(SMOKE_USER)) --report $(SMOKE_REPORT)

smoke-provision: build ## Create a devops template board: make smoke-provision SMOKE_WORKSPACE=<id>
	python3 scripts/smoke.py --provision --workspace $(SMOKE_WORKSPACE) $(if $(SMOKE_USER),--assign-user $(SMOKE_USER)) --report $(SMOKE_REPORT)

docs-tools: build ## Regenerate the tool catalog in docs/TOOLS.md
	python3 scripts/gen_tool_docs.py

vet: ## Run go vet inside Docker
	docker build --target builder -t $(IMAGE)-builder .
	docker run --rm $(IMAGE)-builder go vet ./...

validate: build test race fmt-check vet ## Run the complete local validation suite
	@echo "Validation passed"

clean: ## Remove local images and Compose resources
	$(COMPOSE) down --remove-orphans
	docker image rm $(IMAGE) $(IMAGE)-builder $(IMAGE)-race-builder 2>/dev/null || true
