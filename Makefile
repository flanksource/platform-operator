
# make sure sub-commands don't use eg. fish shell
export SHELL := /bin/bash

# Image URL to use all building/pushing image targets
IMG ?= controller
TAG ?= latest
REGISTRY ?= docker.io

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq ($(shell command -v controller-gen),)
	@(cd /tmp; GO111MODULE=on go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.4)
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

all: manager

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out -v

e2e: fmt vet
	TEST_E2E=true go test ./... -coverprofile cover.out -v

# Build manager binary
manager: fmt vet
	go build -o bin/manager cmd/manager/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run cmd/manager/main.go

# Deploy CRDS, webhoook and controller in the configured Kubernetes cluster in ~/.kube/config
deploy: generate
	cd config/operator/manager && kustomize edit set image controller=${IMG}:${TAG}
	kubectl apply -f config/deploy/manifests.yaml

generate: controller-gen
	# Generate webhook
	$(CONTROLLER_GEN) webhook object:headerFile=./hack/boilerplate.go.txt paths=./pkg/... output:webhook:artifacts:config=config/operator/webhook
	# Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager paths="./pkg/..." output:crd:artifacts:config=config/crds/bases output:rbac:artifacts:config=config/operator/rbac
	# set image name and tag
	# Generate an all-in-one version including the operator manifests
	kubectl kustomize config/operator/default > config/deploy/manifests.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

#####################################
##  --       Release           --  ##
#####################################

# Build the docker image
docker-build: test
	docker build . -t $(REGISTRY)/$(IMG):$(TAG)

# Login to docker registry
docker-login:
	@ echo $(DOCKER_USER)
	docker login -u $(DOCKER_USER) -p $(DOCKER_PASS)

# Push the docker image
docker-push:
	docker push $(REGISTRY)/$(IMG):$(TAG)

ci-release: docker-build docker-login docker-push
	@ echo $(REGISTRY)/$(IMG):$(TAG) was pushed!
