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
go build .
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --master <API server address > --podspec=examples/pod.yaml
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

* 0.15 CPU
* 100Mi

**Output**:

```sh
Pod requirements:
	- cpu: 0.15
	- memory: 104857600

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
   "Cpu": 0.15,
   "Memory": 104857600
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
   "Cpu": 0.15,
   "Memory": 104857600
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


