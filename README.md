# Cluster capacity analysis framework

Implementation of [cluster capacity analysis](https://github.com/ingvagabund/kubernetes/blob/a6cf56c2482627b0adebaffe1953c69ea4b4e4db/docs/proposals/cluster-capacity.md).

## Intro

As new pods get scheduled on nodes in a cluster, more resources get consumed.
Monitoring available resources in the cluster is very important
as operators can increase the current resources in time before all of them get exhausted.
Or, carry different steps that lead to increase of available resources.

Cluster capacity consists of capacities of individual cluster nodes.
Capacity covers CPU, memory, disk space and other resources.

Overall remaining allocatable capacity is a rough estimation since it does not assume all resources being distributed among nodes.
Goal is to analyze remaining allocatable resources and estimate available capacity that is still consumable
in terms of a number of instances of a pod with given requirements that can be scheduled in a cluster.

## Build and Run

Build the framework:

```sh
$ make build
```

and run the analysis:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=examples/pod.yaml
```

For more information about available options run:
```
$ ./cluster-capacity --help
```

## Demonstration

Assuming a cluster is running with 4 nodes and 1 master with each node with 2 CPUs and 4GB of memory.
With pod resource requirements to be `150m` of CPU and ``100Mi`` of Memory.

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=pod.yaml --verbose
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

To decrease available resources in the cluster you can use provided RC (`examples/rc.yml`):

```sh
$ kubectl create -f examples/rc.yml
```

E.g. to change a number of replicas to `6`, you can run:

```sh
$ kubectl patch -f examples/rc.yml -p '{"spec":{"replicas":6}}'
```

Once the number of running pods in the cluster grows and the analysis is run again,
the number of schedulable pods decreases as well:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=pod.yaml --verbose
Pod requirements:
	- cpu: 150m
	- memory: 100Mi

The cluster can schedule 46 instance(s) of the pod.
Termination reason: FailedScheduling: pod (small-pod-46) failed to fit in any node
fit failure on node (kube-node-1): Insufficient cpu
fit failure on node (kube-node-4): Insufficient cpu
fit failure on node (kube-node-2): Insufficient cpu
fit failure on node (kube-node-3): Insufficient cpu


Pod distribution among nodes:
	- kube-node-1: 11 instance(s)
	- kube-node-4: 12 instance(s)
	- kube-node-2: 11 instance(s)
	- kube-node-3: 12 instance(s)
```

## Pod generator

As pods are part of a namespace with resource limits and additional constraints (e.g. node selector forced by namespace annotation),
it is natural to analyse how many instances of a pod with maximal resource requirements can be scheduled.
In order to generate the pod, you can run:

```sh
$ genpod --kubeconfig <path to kubeconfig>  --namespace <namespace>
```

Assuming at least one resource limits object is available with at least one maximum resource type per pod.
If multiple resource limits objects per namespace are available, minimum of all maximum resources per type is taken.
If a namespace is annotated with `openshift.io/node-selector`, the selector is set as pod's node selector.

**Example**:

Assuming `cluster-capacity` namespace with `openshift.io/node-selector: "region=hpc,load=high"` annotation
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
$ genpod --kubeconfig <path to kubeconfig>  --namespace cluster-capacity
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

## Admissions

The analysis can be run with admissions enabled as well:

```sh
./cluster-capacity --kubeconfig <path to kubeconfig>  --podspec=examples/pod.yaml --admission-control LimitRanger,ResourceQuota --resource-space-mode ResourceSpacePartial
```

The admission controllers are configured the same way as in the Apiserver by specifying --admission-control flag.
Currently only supported admission plugins are ``LimitRanger``, and ``ResourceQuota``. Each time a pod is scheduled, it
is run through the admission controller first. ``LimitRanger`` can alter pod's specification, and ``ResourceQuota`` can
forbid the pod from being scheduled.

As the ``ResourceQuota`` limits pods in a namespace, it is not enabled in the
analysis by default. In order to enable the plugin as well, ``--resource-space-mode``
needs to be set to ``ResourceSpacePartial``.

**Example**:

Assuming the following namespace, resource quota and limit range objects are available in the cluster:

```sh
$ cat examples/namespace.yml 
apiVersion: v1
kind: Namespace
metadata:
  name: cluster-capacity
  annotations:
    openshift.io/node-selector: "region=hpc,load=high"
```

```sh
$ cat examples/limits.yml 
apiVersion: v1
kind: LimitRange
metadata:
  name: hpclimits
  namespace: cluster-capacity
spec:
  limits:
  - max:
      cpu: "200m"
      memory: 200Mi
    min:
      cpu: 10m
      memory: 6Mi
    type: Pod
  - default:
      cpu: 10m
      memory: 6Mi
    defaultRequest:
      cpu: 10m
      memory: 6Mi
    max:
      cpu: "250m"
      memory: 200Mi
    min:
      cpu: 10m
      memory: 6Mi
    type: Container
```

```sh
$ cat examples/rq.yml 
apiVersion: v1
kind: ResourceQuota
metadata:
  name: compute-resources
  namespace: cluster-capacity
spec:
  hard:
    pods: "4"
    requests.cpu: "1"
    requests.memory: 1Gi
    limits.cpu: "2"
    limits.memory: 2Gi
```

When running the analysis the number of instances of a pod is limited by the resource quota:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=examples/pod.yaml --admission-control LimitRanger,ResourceQuota  --verbose --resource-space-mode ResourceSpacePartial
Pod requirements:
	- cpu: 150m
	- memory: 100Mi

The cluster can schedule 4 instance(s) of the pod.
Termination reason: AdmissionControllerError: pods "small-pod-4" is forbidden: exceeded quota: compute-resources, requested: pods=1, used: pods=4, limited: pods=4

Pod distribution among nodes:
	- 127.0.0.1: 4 instance(s)
```

## Continuous cluster capacity analysis

The provided analysis can be run in loop to provide continuous stream of actual cluster capacities.


To start the continuous analysis providing the capacity each second you can run:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=examples/pod.yaml --period 1
```

With the ``period`` set to non-zero value, the ``cluster-capacity`` binary publishes the current capacity
at ``http://localhost:8081/capacity/status?watch=true`` address.
You can use ``curl`` to access the data:

```sh
$ curl http://localhost:8081/capacity/status?watch=true
[
  {
   "Timestamp": "2016-11-16T08:30:33.079973497Z",
   "PodRequirements": {
    "Cpu": "200m",
    "Memory": "100Mi"
   },
   "TotalInstances": 5,
   "NodesNumInstances": {
    "kube-node-2": 5
   },
   "FailReasons": {
    "FailType": "FailedScheduling",
    "FailMessage": "pod (cluster-capacity-stub-container-5) failed to fit in any node",
    "NodeFailures": {
     "kube-node-1": "MatchNodeSelector",
     "kube-node-2": "Insufficient cpu"
    }
   }
  },
  {
   "Timestamp": "2016-11-16T08:30:43.277040728Z",
   "PodRequirements": {
    "Cpu": "200m",
    "Memory": "100Mi"
   },
   "TotalInstances": 5,
   "NodesNumInstances": {
    "kube-node-2": 5
   },
   "FailReasons": {
    "FailType": "FailedScheduling",
    "FailMessage": "pod (cluster-capacity-stub-container-5) failed to fit in any node",
    "NodeFailures": {
     "kube-node-1": "MatchNodeSelector",
     "kube-node-2": "Insufficient cpu"
    }
   }
  }
 ]
...
```

## Roadmap

Underway:

* analysis covering scheduler and admission controller
* generic framework for any scheduler created by the default scheduler factory
* continuous stream of estimations

Would like to get soon:

* include multiple schedulers
* accept a list (sequence) of pods
* extend analysis with volume handling
* define common interface each scheduler need to implement if embedded in the framework

Other possibilities:

* incorporate re-scheduler
* incorporate preemptive scheduling
* include more of Kubelet's behaviour (e.g. recognize memory pressure, secrets/configmap existence test)
