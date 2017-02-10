build:
	go build -o hypercc github.com/kubernetes-incubator/cluster-capacity/cmd/hypercc
	ln -sf hypercc cluster-capacity
	ln -sf hypercc genpod
run:
	@./cluster-capacity --kubeconfig ~/.kube/config --master http://localhost:8080 --podspec=pod.yaml --verbose

test:
	./test.sh

integration-tests:
	./integration-tests.sh

image:
	docker build -t docker.io/gofed/cluster-capacity .
