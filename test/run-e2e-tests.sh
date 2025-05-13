#!/usr/bin/env bash

# Copyright 2020 The Kubernetes Authors.
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

set -x
set -o errexit
set -o nounset

# Set to empty if unbound/empty
SKIP_INSTALL=${SKIP_INSTALL:-}
KIND_E2E=${KIND_E2E:-}

# This just runs e2e tests.
if [ -n "$KIND_E2E" ]; then
    # If we did not set SKIP_INSTALL
    if [ -z "$SKIP_INSTALL" ]; then
        K8S_VERSION=${KUBERNETES_VERSION:-v1.33.0}
        curl -Lo kubectl https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/amd64/kubectl && chmod +x kubectl && mv kubectl /usr/local/bin/
        wget https://github.com/kubernetes-sigs/kind/releases/download/v0.27.0/kind-linux-amd64
        chmod +x kind-linux-amd64
        mv kind-linux-amd64 kind
        export PATH=$PATH:$PWD
        kind create cluster --image kindest/node:${K8S_VERSION} --config=./hack/kind_config.yaml
    fi
    ${CONTAINER_ENGINE:-docker} pull registry.k8s.io/pause
    kind load docker-image registry.k8s.io/pause
    kind get kubeconfig > /tmp/admin.conf
    export KUBECONFIG="/tmp/admin.conf"
    mkdir -p ~/gopath/src/sigs.k8s.io/
fi

PRJ_PREFIX="sigs.k8s.io/cluster-capacity"
go test ${PRJ_PREFIX}/test/e2e/ -v

# Just test the binary works with the default example pod spec
# See https://github.com/kubernetes-sigs/cluster-capacity/pull/127 for more detail
GO111MODULE=auto go build -o hypercc sigs.k8s.io/cluster-capacity/cmd/hypercc
ln -sf hypercc cluster-capacity

./cluster-capacity --kubeconfig ~/.kube/config  --podspec examples/pod.yaml --verbose
