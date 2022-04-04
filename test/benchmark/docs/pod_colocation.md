# Pod colocation

Ability to colocate pods in the same topology domain (e.g. node, zone, rack).
Once a pod is scheduled into a topology domain,
every other pod with the same colocation configuration is scheduled into
the same domain as well.

## Scheduling framework capabilities

Any scheduling capability is expected to be achieved by combination of
scheduling plugins. For the mentioned capability the framework offers
[InterPodAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) plugin.

## InterPodAffinity properties

- pod affinity is namespace scoped
- pods getting colocated on the same node (or a set of nodes)

## Verified cases

- topology domain a node, all pods scheduled onto the same node (topology of size 1)
- topology domain a zone, all pods scheduled onto nodes from the same zone (topology of size greater than 1)

Tests available under `test/benchmark`.
