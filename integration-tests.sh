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

#! /bin/sh

# Assumptions:
# - cluster provisioned
# - KUBE_MASTER_API: master api url
# - KUBE_MASTER_API_PORT: master api port

KUBE_MASTER_API=${KUBE_MASTER_API:-http://localhost}
KUBE_MASTER_API_PORT=${KUBE_MASTER_API_PORT:-443}
KUBE_CONFIG=${KUBE_CONFIG:-~/.kube/config}

alias kubectl="kubectl --kubeconfig=${KUBE_CONFIG} --server=${KUBE_MASTER_API}:${KUBE_MASTER_API_PORT}"
alias cc="./cluster-capacity --kubeconfig ${KUBE_CONFIG}"
#### pre-tests checks

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

function printError {
  echo -e "${RED}$1${NC}"
}

function printSuccess {
  echo -e "${GREEN}$1${NC}"
}

echo "####PRE-TEST CHECKS"
# check the cluster is available
kubectl version
if [ "$?" -ne 0 ]; then
  printError "Unable to connect to kubernetes cluster"
  exit 1
fi

# check the cluster-capacity namespace exists
kubectl get ns cluster-capacity
if [ "$?" -ne 0 ]; then
  kubectl create -f examples/namespace.yml
  if [ "$?" -ne 0 ]; then
    printError "Unable to create cluster-capacity namespace"
    exit 1
  fi
fi

echo ""
echo ""

#### TESTS
echo "####RUNNING TESTS"
echo ""
echo "# Running simple estimation of examples/pod.yaml"
cc --podspec=examples/pod.yaml | tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "# Running simple estimation of examples/pod.yaml in verbose mode"
cc --podspec=examples/pod.yaml --verbose | tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "Decrease resource in the cluster by running new pods"
kubectl create -f examples/rc.yml
if [ "$?" -ne 0 ]; then
  printError "Unable to create additional resources"
  exit 1
fi

while [ $(kubectl get pods | grep nginx | grep "Running" | wc -l) -ne 3 ]; do
  echo "waiting for pods to come up"
  sleep 1s
done

echo ""
echo "# Running simple estimation of examples/pod.yaml in verbose mode with less resources"
cc --podspec=examples/pod.yaml --verbose | tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "Delete resource in the cluster by deleting rc"
kubectl delete -f examples/rc.yml

echo ""
echo ""
printSuccess "#### All tests passed"

exit 0
