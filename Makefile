include func.mk

PROJECT_NAME:=odh-platform
PACKAGE_NAME:=github.com/opendatahub-io/$(PROJECT_NAME)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail

.PHONY: all
all: tools lint test build

##@ General
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Purges build artifacts
	@rm -rf $(LOCALBIN)

##@ Development

.PHONY: generate
generate: tools ## Generates required resources for the controller to work properly (see config/ folder)
	$(LOCALBIN)/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(call fetch-external-crds,github.com/kuadrant/authorino,api/v1beta1)

SRC_DIRS:=./controllers ./test
SRCS:=$(shell find ${SRC_DIRS} -name "*.go")

.PHONY: format
format: $(SRCS) ## Removes unneeded imports and formats source code
	$(call header,"Formatting code")
	$(LOCALBIN)/goimports -l -w -e $(SRC_DIRS) $(TEST_DIRS)

.PHONY: lint
lint: tools format ## Concurrently runs a whole bunch of static analysis tools
	$(call header,"Running a whole bunch of static analysis tools")
	$(LOCALBIN)/golangci-lint run --fix --sort-results

.PHONY: test
test: generate
test: test-unit+kube-envtest ## Run all tests. You can also select a category by running e.g. make test-unit or make test-kube-envtest

ENVTEST_K8S_VERSION = 1.26 # refers to the version of kubebuilder assets to be downloaded by envtest binary.
test-%:
	$(eval test-type:=$(subst +,||,$(subst test-,,$@)))
	KUBEBUILDER_ASSETS="$(shell $(LOCALBIN)/setup-envtest use $(ENVTEST_K8S_VERSION) -p path)" \
	$(LOCALBIN)/ginkgo -r --label-filter="$(test-type)" -vet=off \
	-coverprofile cover.out --junit-report=ginkgo-test-results.xml ${args}

##@ Build
GOOS?=$(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH?=$(shell uname -m | tr '[:upper:]' '[:lower:]' | sed 's/x86_64/amd64/')
GOBUILD:=GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0

# Version values
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GITUNTRACKEDCHANGES:=$(shell git status --porcelain --untracked-files=no)
COMMIT:=$(shell git rev-parse --short HEAD)
ifneq ($(GITUNTRACKEDCHANGES),)
	COMMIT:=$(COMMIT)-dirty
endif

VERSION?=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
GIT_TAG?=$(shell git describe --tags --abbrev=0 --exact-match > /dev/null 2>&1; echo $$?)
ifneq ($(GIT_TAG),0)
	ifeq ($(origin VERSION),file)
		VERSION:=$(VERSION)-next
	endif
endif

LDFLAGS?=-w -X ${PACKAGE_NAME}/version.Version=${VERSION} -X ${PACKAGE_NAME}/version.Commit=${COMMIT} -X ${PACKAGE_NAME}/version.BuildTime=${BUILD_TIME}

.PHONY: deps
deps:
	go mod download && go mod tidy

.PHONY: build
build: tools format generate go-build ## Build manager binary.

.PHONY: go-build
go-build:
	${GOBUILD} go build -ldflags "${LDFLAGS}" -o bin/manager main.go

.PHONY: run
run: format generate ## Run a controller from your host.
	go run ./main.go

##@ Container images
# Prefer to use podman if not explicitly set
CONTAINER_ENGINE ?= docker
ifneq (, $(shell which podman))
	CONTAINER_ENGINE = podman
endif

IMG ?= quay.io/maistra-dev/$(PROJECT_NAME)
# If the commit is not tagged, use "latest", otherwise use the tag name
ifeq ($(GIT_TAG), 0)
	TAG ?= $(VERSION)
else
	TAG ?= latest
endif

.PHONY: image-build
image-build: ## Build container image
	${CONTAINER_ENGINE} build --build-arg LDFLAGS="$(LDFLAGS)" . -t ${IMG}:${TAG} ${DOCKER_ARGS}

.PHONY: image-push
image-push: ## Push container image
	${CONTAINER_ENGINE} tag ${IMG}:${TAG} ${IMG}:latest
	${CONTAINER_ENGINE} push ${IMG}:${TAG}
	${CONTAINER_ENGINE} push ${IMG}:latest

.PHONY: image
image: image-build image-push ## Build and push docker image with the manager.

##@ Release
define update-manifests
	sed -i 's/newTag: .*/newTag: $(1)/' config/manager/kustomization.yaml
endef

.PHONY: release
release: ## Create a new release
	$(call header,"Releasing version $(VERSION)")
	@$(call update-manifests,$(VERSION)); \
	git add config/ && \
	git commit -m "release: $(VERSION)" && \
	git tag -a "$(VERSION)" -m "Release $(VERSION)"; \
	$(call update-manifests,latest); \
	git add config/ && \
	git commit -m "release: next iteration"
	@echo "Done, don't forget to push. When pushing, add --tags to your call."


##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: deploy
deploy: generate ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(LOCALBIN)/kustomize edit set image controller=${IMG}:${TAG}
	kubectl apply -k config/base

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	kubectl delete --ignore-not-found=$(ignore-not-found) -k config/base

##@ Build Dependencies
LOCALBIN ?= $(shell pwd)/bin
$(shell	mkdir -p $(LOCALBIN))

.PHONY: tools
tools: deps
tools: $(LOCALBIN)/controller-gen $(LOCALBIN)/kustomize ## Installs required tools in local ./bin folder
tools: $(LOCALBIN)/setup-envtest $(LOCALBIN)/ginkgo
tools: $(LOCALBIN)/goimports $(LOCALBIN)/golangci-lint

KUSTOMIZE_VERSION ?= v5.0.1
$(LOCALBIN)/kustomize:
	$(call header,"Installing $(notdir $@)")
	wget -q -c https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F$(KUSTOMIZE_VERSION)/kustomize_$(KUSTOMIZE_VERSION)_$(GOOS)_$(GOARCH).tar.gz -O /tmp/kustomize.tar.gz
	tar xzvf /tmp/kustomize.tar.gz -C $(LOCALBIN)
	chmod +x $(LOCALBIN)/kustomize

CONTROLLER_TOOLS_VERSION?=$(call go-mod-version,'controller-tools')
$(LOCALBIN)/controller-gen:
	$(call header,"Installing $(notdir $@)")
	$(call go-get-tool,controller-gen,sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION))

ENVTEST_PACKAGE_VERSION = v0.0.0-20240320141353-395cfc7486e6
$(LOCALBIN)/setup-envtest:
	$(call header,"Installing $(notdir $@)")
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_PACKAGE_VERSION)

$(LOCALBIN)/ginkgo:
	$(call header,"Installing $(notdir $@)")
	GOBIN=$(LOCALBIN) go install -mod=readonly github.com/onsi/ginkgo/v2/ginkgo

$(LOCALBIN)/goimports:
	$(call header,"Installing $(notdir $@)")
	GOBIN=$(LOCALBIN) go install -mod=readonly golang.org/x/tools/cmd/goimports

LINT_VERSION=v1.59.1
$(LOCALBIN)/golangci-lint:
	$(call header,"Installing $(notdir $@)")
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(LINT_VERSION)
