# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

export CONTAINER_ENGINE ?= docker

# VERSION is based on a date stamp plus the last commit
VERSION?=v$(shell date +%Y%m%d)-$(shell git describe --tags)
BRANCH?=$(shell git branch --show-current)
SHA1?=$(shell git rev-parse HEAD)
BUILD=$(shell date +%FT%T%z)
LDFLAG_LOCATION=sigs.k8s.io/cluster-capacity/pkg/version
ARCHS = amd64 arm arm64

LDFLAGS=-ldflags "-X ${LDFLAG_LOCATION}.version=${VERSION} -X ${LDFLAG_LOCATION}.buildDate=${BUILD} -X ${LDFLAG_LOCATION}.gitbranch=${BRANCH} -X ${LDFLAG_LOCATION}.gitsha1=${SHA1}"

# REGISTRY is the container registry to push
# into. The default is to push to the staging
# registry, not production.
REGISTRY?=gcr.io/k8s-staging-cluster-capacity

# IMAGE is the image name of cluster-capacity
IMAGE:=cluster-capacity:$(VERSION)

# IMAGE_GCLOUD is the image name of cluster-capacity in the remote registry
IMAGE_GCLOUD:=$(REGISTRY)/cluster-capacity:$(VERSION)

run:
	@./cluster-capacity --kubeconfig ~/.kube/config --podspec=examples/pod.yaml --verbose

verify-gofmt:
	./hack/verify-gofmt.sh

test-unit:
	./hack/unit-test.sh

test-integration:
	./integration-tests.sh

test-e2e:
	./test/run-e2e-tests.sh

install-golint: ## check golint if not exist install golint tools
ifeq (, $(shell which golangci-lint))
	@{ \
	set -e ;\
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.31.0 ;\
	}
GOLINT_BIN=$(shell go env GOPATH)/bin/golangci-lint
else
GOLINT_BIN=$(shell which golangci-lint)
endif

lint: install-golint ## Run go lint against code.
	$(GOLINT_BIN) run -v

build:
	CGO_ENABLED=0 go build ${LDFLAGS} -o _output/bin/cluster-capacity sigs.k8s.io/cluster-capacity/cmd/cluster-capacity
	CGO_ENABLED=0 go build ${LDFLAGS} -o _output/bin/genpod sigs.k8s.io/cluster-capacity/cmd/genpod

build.amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o _output/bin/cluster-capacity sigs.k8s.io/cluster-capacity/cmd/cluster-capacity
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o _output/bin/genpod sigs.k8s.io/cluster-capacity/cmd/genpod

build.arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build ${LDFLAGS} -o _output/bin/cluster-capacity sigs.k8s.io/cluster-capacity/cmd/cluster-capacity
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build ${LDFLAGS} -o _output/bin/genpod sigs.k8s.io/cluster-capacity/cmd/genpod

build.arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o _output/bin/cluster-capacity sigs.k8s.io/cluster-capacity/cmd/cluster-capacity
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o _output/bin/genpod sigs.k8s.io/cluster-capacity/cmd/genpod

image:
	$(CONTAINER_ENGINE) build --build-arg VERSION="$(VERSION)" --build-arg ARCH="amd64" -t $(IMAGE) .

image.amd64:
	$(CONTAINER_ENGINE) build --build-arg VERSION="$(VERSION)" --build-arg ARCH="amd64" -t $(IMAGE)-amd64 .

image.arm:
	$(CONTAINER_ENGINE) build --build-arg VERSION="$(VERSION)" --build-arg ARCH="arm" -t $(IMAGE)-arm .

image.arm64:
	$(CONTAINER_ENGINE) build --build-arg VERSION="$(VERSION)" --build-arg ARCH="arm64" -t $(IMAGE)-arm64 .

push-all: image.amd64 image.arm image.arm64
	gcloud auth configure-docker
	for arch in $(ARCHS); do \
		$(CONTAINER_ENGINE) tag $(IMAGE)-$${arch} $(IMAGE_GCLOUD)-$${arch} ;\
		$(CONTAINER_ENGINE) push $(IMAGE_GCLOUD)-$${arch} ;\
	done
	DOCKER_CLI_EXPERIMENTAL=enabled $(CONTAINER_ENGINE) manifest create $(IMAGE_GCLOUD) $(addprefix --amend $(IMAGE_GCLOUD)-, $(ARCHS))
	for arch in $(ARCHS); do \
		DOCKER_CLI_EXPERIMENTAL=enabled $(CONTAINER_ENGINE) manifest annotate --arch $${arch} $(IMAGE_GCLOUD) $(IMAGE_GCLOUD)-$${arch} ;\
	done
	DOCKER_CLI_EXPERIMENTAL=enabled $(CONTAINER_ENGINE) manifest push $(IMAGE_GCLOUD) ;\

clean:
	rm -f cluster-capacity genpod hypercc
