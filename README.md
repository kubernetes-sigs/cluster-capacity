# cluster-capacity

This is a proof of concept implementation of [cluster capacity analysis](https://github.com/ingvagabund/kubernetes/blob/a6cf56c2482627b0adebaffe1953c69ea4b4e4db/docs/proposals/cluster-capacity.md).

## Intro

As new pods get scheduled on nodes in a cluster, more resources get consumed.
Monitoring available resources in the cluster is very important
as operators can increase the current resources in time before all of them get exhausted.
Or, carry different steps that lead to increase of available resources.

Cluster capacity consists of capacities of individual cluster nodes.
Capacity covers CPU, memory, disk space and other resources.

Goal is to analyze remaining allocatable resources and estimate available capacity that is still consumable
in terms of how many instances of a pod with given requirements can be scheduled in a cluster.

## Example use:

```
go build -o cluster-capacity github.com/ingvagabund/cluster-capacity/cmd/cluster-capacity
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --master <API server address> --podspec=examples/pod.yaml
```

For more information run:
```
$ ./cluster-capacity --help
```

## Example output

**System parameters**:

* cluster with 4 nodes and 1 master with
* each node with 2 CPUs and 4GB of memory

**Pod requirements**:

* 150m
* 100Mi

**Output**:

```sh
Pod requirements:
	- cpu: 150m
	- memory: 100Mi

The cluster can schedule 52 instance(s) of the pod.
Termination reason: FailedScheduling: pod (small-pod-52) failed to fit in any node
fit failure on node (kube-node-1): Insufficient cpu
fit failure on node (kube-node-4): Insufficient cpu
fit failure on node (kube-node-2): Insufficient cpu
fit failure on node (kube-node-3): Insufficient cpu


Pod distribution among nodes:
	- kube-node-1: 13 instance(s)
	- kube-node-4: 13 instance(s)
	- kube-node-2: 13 instance(s)
	- kube-node-3: 13 instance(s)
```

To decrease available resources you can use provided RC (`examples/rc.yml`):

```sh
$ kubectl create -f examples/rc.yml
```

E.g. to change a number of replicas to `6`, you can run:

```sh
$ kubectl patch -f examples/rc.yml -p '{"spec":{"replicas":6}}
```

## Pod generator

As pods are part of a namespace with resource limits and additional constraints (e.g. node selector forced by namespace annotation),
it is natural to analyse how many instances of a pod with maximal resource requirements can be scheduled.
In order to generate the pod, you can run:

```sh
$ genpod --kubeconfig <path to kubeconfig> --master <API server address> --namespace <namespace>
```

Assuming at least one resource limits object is available with at least one maximum resource type per pod.
If multiple resource limits objects per namespace are available, minimum of all maximum resources per type is taken.
If a namespace is annotated with `openshift.io/node-selector`, the selector is set as pod's node selector.

**Example**:

Assuming `cluster-capacity` namespace is created with `openshift.io/node-selector: "region=hpc,load=high"` annotation
and resource limits are created (see `examples/namespace.yml` and `examples/limits.yml`)

```sh
$ kubectl describe limits hpclimits --namespace cluster-capacity
Name:           hpclimits
Namespace:      cluster-capacity
Type            Resource        Min     Max     Default Request Default Limit   Max Limit/Request Ratio
----            --------        ---     ---     --------------- -------------   -----------------------
Pod             cpu             10m     200m    -               -               -
Pod             memory          6Mi     100Mi   -               -               -
Container       memory          6Mi     20Mi    6Mi             6Mi             -
Container       cpu             10m     50m     10m             10m             -

```

```sh
$ genpod --kubeconfig <path to kubeconfig> --master <API server address> --namespace cluster-capacity
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  name: cluster-capacity-stub-container
  namespace: cluster-capacity
spec:
  containers:
  - image: gcr.io/google_containers/pause:2.0
    imagePullPolicy: Always
    name: cluster-capacity-stub-container
    resources:
      limits:
        cpu: 200m
        memory: 100Mi
      requests:
        cpu: 200m
        memory: 100Mi
  dnsPolicy: Default
  nodeSelector:
    load: high
    region: hpc
  restartPolicy: OnFailure
status: {}
```

## Continuous cluster capacity analysis

The provided analysis can be run in loop to provide continuous stream of actual cluster capacities.


To start the continuous analysis providing the capacity each second you can run:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --master <API server address > --podspec=examples/pod.yaml --period 1
```

With the ``period`` set to non-zero value, the ``cluster-capacity`` binary publishes the current capacity
at ``http://localhost:8081/capacity/status/watch`` address.
You can use ``curl`` to access the data:

```sh
$ curl http://localhost:8081/capacity/status/watch
{
  "Timestamp": "2016-10-24T22:27:52.67211714+02:00",
  "PodRequirements": {
    "Cpu": "150m",
    "Memory": "100Mi"
  },
  "Total": {
   "Instances": 23,
   "Reason": "FailedScheduling: pod (small-pod-23) failed to fit in any node\nfit failure on node (127.0.0.1): Insufficient cpu\n"
  },
  "Nodes": [
   {
    "NodeName": "127.0.0.1",
    "Instances": 23,
    "Reason": ""
   }
  ]
 }{
  "Timestamp": "2016-10-24T22:27:53.872736917+02:00",
  "PodRequirements": {
    "Cpu": "150m",
    "Memory": "100Mi"
  },
  "Total": {
   "Instances": 23,
   "Reason": "FailedScheduling: pod (small-pod-23) failed to fit in any node\nfit failure on node (127.0.0.1): Insufficient cpu\n"
  },
  "Nodes": [
   {
    "NodeName": "127.0.0.1",
    "Instances": 23,
    "Reason": ""
   }
  ]
...
```
