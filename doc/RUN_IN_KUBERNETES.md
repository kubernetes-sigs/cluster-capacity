# Deploying cluster-capacity on kubernetes

 - Have a running kubernetes environment - You can use e.g $ hack/local-up-cluster.sh in cloned local [kubernetes repository](https://github.com/kubernetes/kubernetes)

 - If your kubernetes cluster doesn't have limit ranges and requests specified you can do it by specifying limitrange object. If you just want to try cluster-capacity, you can use 
 [Example limit range file](https://github.com/ingvagabund/cluster-capacity/blob/master/doc/example-limit-range.yaml)
 
 - Create pod object:
```sh
$ cat cluster-capacity-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: cluster-capacity
  labels:
    name: cluster-capacity
spec:
  containers:
  - name: cluster-capacity
    image: docker.io/dhodovsk/cluster-capacity:latest
    command:
    - "/bin/sh"
    - "-c"
    - |
      /bin/genpod --namespace=default >> /pod.yaml && /bin/cluster-capacity --period=10 --podspec=/pod.yaml
    ports:
    - containerPort: 8081
$ kubectl create -f cluster-capacity-pod.yaml
```

 - We need to create proxy, so user can access server running in a pod. That can be done using [kubectl expose](http://kubernetes.io/docs/user-guide/kubectl/kubectl_expose/)

```sh
$ kubectl expose pod cluster-capacity --port=8081
```

 - Get endpoint URL
 
```sh
$ kubectl get endpoints cluster-capacity
```

 - Now you should be able to see cluster status: 

```sh
curl http://<endpoint>/capacity/status
```

 - For more information of how to access acquired data any see [API operations](https://github.com/ingvagabund/cluster-capacity/blob/master/doc/api-operations.md)
 