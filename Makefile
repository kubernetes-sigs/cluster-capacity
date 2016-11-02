build:
	go build -o cluster-capacity github.com/ingvagabund/cluster-capacity/cmd/cluster-capacity
run:
	@./cluster-capacity --kubeconfig ~/.kube/config --master http://localhost:8080 --podspec=pod.yaml --verbose
