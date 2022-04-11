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

build:
	GO111MODULE=auto go build -o hypercc sigs.k8s.io/cluster-capacity/cmd/hypercc
	ln -sf hypercc cluster-capacity
	ln -sf hypercc genpod

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

image:
	docker build -t cluster-capacity .

clean:
	rm -f cluster-capacity genpod hypercc


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