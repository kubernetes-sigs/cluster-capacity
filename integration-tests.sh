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

KUBE_CONFIG=${KUBE_CONFIG:-~/.kube/config}

KUBECTL="kubectl --kubeconfig=${KUBE_CONFIG}"
CC="./cluster-capacity --kubeconfig ${KUBE_CONFIG}"
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
$KUBECTL version
if [ "$?" -ne 0 ]; then
  printError "Unable to connect to kubernetes cluster"
  exit 1
fi

# check the cluster-capacity namespace exists
$KUBECTL get ns cluster-capacity
if [ "$?" -ne 0 ]; then
  $KUBECTL create -f examples/namespace.yml
  if [ "$?" -ne 0 ]; then
    printError "Unable to create cluster-capacity namespace"
    exit 1
  fi
fi

echo ""
echo ""

#### TESTS
echo ""
echo ""
echo ""
echo ""
echo ""
echo "####RUNNING TESTS"
echo ""
echo "# Running simple estimation of examples/pod.yaml"
$CC --podspec=examples/pod.yaml | tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "# Running simple estimation of examples/pod.yaml in verbose mode"
$CC --podspec=examples/pod.yaml --verbose | tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "# Decrease resource in the cluster by running new pods"
$KUBECTL create -f examples/rc.yml
if [ "$?" -ne 0 ]; then
  printError "Unable to create additional resources"
  exit 1
fi

while [ $($KUBECTL get pods | grep nginx | grep "Running" | wc -l) -ne 3 ]; do
  echo "waiting for pods to come up"
  sleep 1s
done

echo ""
echo "# Running simple estimation of examples/pod.yaml in verbose mode with less resources"
$CC --podspec=examples/pod.yaml --verbose | tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "# Delete resource in the cluster by deleting rc"
$KUBECTL delete -f examples/rc.yml


echo ""
echo ""
echo "# Running with ResourceQuota admission plugin"
echo "## Creating resource quota"
$KUBECTL create resourcequota pod-quota --namespace=cluster-capacity --hard=pods=1

$CC --podspec=examples/pod.yaml --resource-space-mode=ResourceSpacePartial --admission-control="ResourceQuota" | tee estimation.log
if [ -z "$(cat estimation.log | grep 'AdmissionControllerError')" ]; then
  printError "Missing Admission controller error"
  $KUBECTL delete resourcequota pod-quota --namespace=cluster-capacity
  exit 1
fi

echo "## Deleting resource quota"
$KUBECTL delete resourcequota pod-quota --namespace=cluster-capacity


echo ""
echo ""
echo "# Running with LimitRange admission plugin"
echo "## Creating limit range"
lrspec=$(mktemp)
cat << EOF > $lrspec
apiVersion: v1
kind: LimitRange
metadata:
  name: simple-limit
  namespace: cluster-capacity
spec:
  limits:
  - max:
      cpu: "1m"
    type: Pod
EOF
$KUBECTL create -f $lrspec
rm $lrspec

$CC --podspec=examples/pod.yaml --admission-control="LimitRanger" | tee estimation.log
if [ -z "$(cat estimation.log | grep 'AdmissionControllerError')" ]; then
  printError "Missing Admission controller error"
  $KUBECTL delete limitrange simple-limit --namespace=cluster-capacity
  exit 1
fi

echo "## Deleting limit range"
$KUBECTL delete limitrange simple-limit --namespace=cluster-capacity

echo ""
echo ""
echo "# Running with not supported admission plugin"
$CC --podspec=examples/pod.yaml --admission-control="foo" | tee estimation.log
if [ -z "$(cat estimation.log | grep 'not supported admission control plugin.')" ]; then
  printError "Should fail with unsupported admission plugin"
  exit 1
fi

printSuccess "#### All tests passed"

#### BOILERPLATE
echo ""
echo ""
echo ""
echo ""
echo ""
echo "####RUNNING BOILERPLATE"
./verify/verify-boilerplate.sh

