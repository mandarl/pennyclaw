# PennyClaw Makefile
# Version: 0.5.0

VERSION ?= 0.5.0
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(DATE)

.PHONY: build run clean test docker deploy logs

## build: Compile the PennyClaw binary
build:
	@echo "Building PennyClaw $(VERSION)..."
	CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -o bin/pennyclaw ./cmd/pennyclaw
	@echo "Built: bin/pennyclaw ($(shell du -h bin/pennyclaw | cut -f1))"

## run: Build and run PennyClaw locally
run: build
	./bin/pennyclaw --config config.example.json

## clean: Remove build artifacts
clean:
	rm -rf bin/ /tmp/pennyclaw-sandbox

## test: Run all tests
test:
	go test -v -race ./...

## docker: Build Docker image
docker:
	docker build -t pennyclaw:$(VERSION) .
	@echo "Image size: $(shell docker image inspect pennyclaw:$(VERSION) --format='{{.Size}}' | numfmt --to=iec)"

## docker-run: Run PennyClaw in Docker
docker-run: docker
	docker run -p 3000:3000 -e OPENAI_API_KEY=$${OPENAI_API_KEY} pennyclaw:$(VERSION)

## deploy: Deploy to GCP free tier (interactive)
deploy:
	@bash scripts/deploy.sh

## teardown: Remove PennyClaw from GCP
teardown:
	@bash scripts/teardown.sh

## preflight: Run pre-flight checks only (no deployment)
preflight:
	@bash scripts/deploy.sh --preflight-only

## logs: View PennyClaw service logs (on GCP VM)
logs:
	@journalctl -u pennyclaw -f --no-pager

## help: Show this help
help:
	@echo "PennyClaw v$(VERSION) — Your \$$0/month AI agent"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
