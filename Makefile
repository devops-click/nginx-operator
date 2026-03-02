# ============================================================================
# NGINX Operator - Makefile
# ============================================================================
# This Makefile provides targets for building, testing, linting, and releasing
# the NGINX Kubernetes Operator. It follows kubebuilder conventions.
#
# Usage:
#   make help          - Show all available targets
#   make build         - Build the operator binary
#   make test          - Run unit tests
#   make lint          - Run linters
#   make docker-build  - Build Docker image
#   make generate      - Generate CRD manifests and deepcopy
#   make install       - Install CRDs into cluster
# ============================================================================

# --- Project Configuration ---
PROJECT_NAME := nginx-operator
ORG := devops-click
MODULE := github.com/$(ORG)/$(PROJECT_NAME)
IMG ?= ghcr.io/$(ORG)/$(PROJECT_NAME)
RELOADER_IMG ?= ghcr.io/$(ORG)/$(PROJECT_NAME)-reloader
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# --- Go Configuration ---
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# --- Tool Versions ---
CONTROLLER_TOOLS_VERSION ?= v0.16.4
ENVTEST_VERSION ?= release-0.19
GOLANGCI_LINT_VERSION ?= v1.61.0
HELM_VERSION ?= v3.16.0

# --- Build Flags ---
LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
	-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

# --- Directories ---
BIN_DIR := bin
TOOLS_DIR := $(BIN_DIR)/tools
CRD_DIR := config/crd/bases
CHART_CRD_DIR := charts/nginx-operator/crds
COVERAGE_DIR := coverage

# ============================================================================
# General Targets
# ============================================================================

.PHONY: help
help: ## Display this help message
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*##/ { printf "  %-25s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: all
all: generate fmt lint test build ## Run full build pipeline

# ============================================================================
# Development Targets
# ============================================================================

.PHONY: build
build: ## Build the operator binary
	@echo "==> Building operator binary..."
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/operator ./cmd/operator/

.PHONY: build-reloader
build-reloader: ## Build the config reloader binary
	@echo "==> Building reloader binary..."
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/reloader ./cmd/reloader/

.PHONY: build-all
build-all: build build-reloader ## Build all binaries

.PHONY: run
run: generate ## Run the operator locally against the current cluster
	go run ./cmd/operator/ --debug

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

# ============================================================================
# Code Generation Targets
# ============================================================================

.PHONY: generate
generate: controller-gen ## Generate code (deepcopy, CRDs)
	@echo "==> Generating deepcopy..."
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."
	@echo "==> Generating CRD manifests..."
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=$(CRD_DIR)

.PHONY: manifests
manifests: controller-gen ## Generate RBAC and CRD manifests
	$(CONTROLLER_GEN) rbac:roleName=nginx-operator-role crd paths="./..." output:crd:artifacts:config=$(CRD_DIR)

.PHONY: sync-crds
sync-crds: generate ## Sync generated CRDs to Helm chart
	@echo "==> Syncing CRDs to Helm chart..."
	@mkdir -p $(CHART_CRD_DIR)
	cp $(CRD_DIR)/*.yaml $(CHART_CRD_DIR)/

# ============================================================================
# Test Targets
# ============================================================================

.PHONY: test
test: generate fmt vet ## Run unit tests
	@echo "==> Running unit tests..."
	@mkdir -p $(COVERAGE_DIR)
	go test ./... \
		-coverprofile=$(COVERAGE_DIR)/coverage.out \
		-covermode=atomic \
		-race \
		-count=1 \
		-timeout=300s
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1

.PHONY: test-short
test-short: ## Run unit tests (short mode, skip slow tests)
	go test ./... -short -race -count=1

.PHONY: test-integration
test-integration: generate envtest ## Run integration tests with envtest
	@echo "==> Running integration tests..."
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_VERSION) --bin-dir $(TOOLS_DIR) -p path)" \
		go test ./internal/controller/... \
		-coverprofile=$(COVERAGE_DIR)/integration-coverage.out \
		-covermode=atomic \
		-race \
		-count=1 \
		-timeout=600s \
		-tags=integration

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests (requires running cluster)
	@echo "==> Running e2e tests..."
	go test ./test/e2e/... \
		-count=1 \
		-timeout=1200s \
		-tags=e2e \
		-v

.PHONY: test-all
test-all: test test-integration test-e2e ## Run all tests

.PHONY: coverage
coverage: test ## Generate and open coverage report
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "==> Coverage report: $(COVERAGE_DIR)/coverage.html"

# ============================================================================
# Lint Targets
# ============================================================================

.PHONY: lint
lint: golangci-lint ## Run golangci-lint
	@echo "==> Running linters..."
	$(GOLANGCI_LINT) run --timeout=5m

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint with auto-fix
	$(GOLANGCI_LINT) run --fix --timeout=5m

.PHONY: lint-helm
lint-helm: ## Lint Helm chart
	helm lint charts/nginx-operator

# ============================================================================
# Docker Targets
# ============================================================================

.PHONY: docker-build
docker-build: ## Build operator Docker image
	@echo "==> Building operator Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMG):$(VERSION) \
		-t $(IMG):latest \
		-f Dockerfile .

.PHONY: docker-build-reloader
docker-build-reloader: ## Build reloader Docker image
	@echo "==> Building reloader Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(RELOADER_IMG):$(VERSION) \
		-t $(RELOADER_IMG):latest \
		-f Dockerfile.reloader .

.PHONY: docker-build-all
docker-build-all: docker-build docker-build-reloader ## Build all Docker images

.PHONY: docker-push
docker-push: ## Push operator Docker image
	docker push $(IMG):$(VERSION)
	docker push $(IMG):latest

.PHONY: docker-push-reloader
docker-push-reloader: ## Push reloader Docker image
	docker push $(RELOADER_IMG):$(VERSION)
	docker push $(RELOADER_IMG):latest

.PHONY: docker-push-all
docker-push-all: docker-push docker-push-reloader ## Push all Docker images

# ============================================================================
# Deployment Targets
# ============================================================================

.PHONY: install
install: sync-crds ## Install CRDs into the cluster
	kubectl apply -f $(CRD_DIR)/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the cluster
	kubectl delete -f $(CRD_DIR)/ --ignore-not-found

.PHONY: deploy
deploy: sync-crds ## Deploy operator to cluster using Helm
	helm upgrade --install nginx-operator charts/nginx-operator \
		--namespace nginx-operator-system \
		--create-namespace \
		--set image.tag=$(VERSION)

.PHONY: undeploy
undeploy: ## Undeploy operator from cluster
	helm uninstall nginx-operator --namespace nginx-operator-system

# ============================================================================
# Release Targets
# ============================================================================

.PHONY: release-chart
release-chart: sync-crds lint-helm ## Package Helm chart for release
	@echo "==> Packaging Helm chart..."
	helm package charts/nginx-operator -d $(BIN_DIR)/

.PHONY: changelog
changelog: ## Generate changelog
	@echo "==> Generating changelog..."
	git-cliff --config cliff.toml -o CHANGELOG.md

# ============================================================================
# Security Targets
# ============================================================================

.PHONY: security-scan
security-scan: ## Run security scans
	@echo "==> Running govulncheck..."
	govulncheck ./...

.PHONY: trivy-scan
trivy-scan: docker-build ## Scan Docker image with Trivy
	trivy image --severity HIGH,CRITICAL $(IMG):$(VERSION)

# ============================================================================
# Tool Dependencies
# ============================================================================

CONTROLLER_GEN = $(TOOLS_DIR)/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen
	@test -s $(CONTROLLER_GEN) || \
		GOBIN=$(abspath $(TOOLS_DIR)) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

ENVTEST = $(TOOLS_DIR)/setup-envtest
.PHONY: envtest
envtest: ## Download envtest-setup
	@test -s $(ENVTEST) || \
		GOBIN=$(abspath $(TOOLS_DIR)) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

GOLANGCI_LINT = $(TOOLS_DIR)/golangci-lint
.PHONY: golangci-lint
golangci-lint: ## Download golangci-lint
	@test -s $(GOLANGCI_LINT) || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(abspath $(TOOLS_DIR)) $(GOLANGCI_LINT_VERSION)

# ============================================================================
# Clean Targets
# ============================================================================

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf $(BIN_DIR) $(COVERAGE_DIR)
	go clean -cache -testcache

.PHONY: clean-tools
clean-tools: ## Clean downloaded tools
	rm -rf $(TOOLS_DIR)

.PHONY: clean-all
clean-all: clean clean-tools ## Clean everything
