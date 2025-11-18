# Copyright 2023 The cert-manager Authors.
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


BINDIR ?= $(CURDIR)/_bin

MAKEFLAGS += --warn-undefined-variables --no-builtin-rules
SHELL := /usr/bin/env bash
.SHELLFLAGS := -uo pipefail -c
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:

GO_FILES := $(shell find . -name "*.go")
TEMPLATE_FILES := $(shell find boilerplate-templates -name "*.boilertmpl")

GOLANGCI_LINT_VERSION := v1.52.2

GOFLAGS := -trimpath

RELEASE_VERSION := $(shell git describe --tags --match='v*' --abbrev=14)
GITCOMMIT := $(shell git rev-parse HEAD)

GOLDFLAGS := -w -s \
	-X 'github.com/cert-manager/boilersuite/internal/version.AppVersion=$(RELEASE_VERSION)' \
    -X 'github.com/cert-manager/boilersuite/internal/version.AppGitCommit=$(GITCOMMIT)'

.PHONY: build
build: $(BINDIR)/boilersuite

$(BINDIR)/boilersuite: $(GO_FILES) $(TEMPLATE_FILES) | $(BINDIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(GOLDFLAGS)" -o $@ main.go

.PHONY: test
test:
	go test ./...

.PHONY: smoke-test
smoke-test: $(BINDIR)/boilersuite
	./hack/smoke_test.sh $< ./fixtures

.PHONY: validate-local-boilerplate
validate-local-boilerplate: $(BINDIR)/boilersuite
	$< --skip fixtures .

.PHONY: lint
lint: | $(BINDIR)/golangci-lint
	$(BINDIR)/golangci-lint run

.PHONY: test-all
test-all: test smoke-test validate-local-boilerplate lint

$(BINDIR)/golangci-lint: $(BINDIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)/golangci-lint
	ln -fs $< $@

$(BINDIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)/golangci-lint: | $(BINDIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)
	GOBIN=$(dir $@) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(BINDIR) $(BINDIR)/golangci-lint-$(GOLANGCI_LINT_VERSION):
	@mkdir -p $@
