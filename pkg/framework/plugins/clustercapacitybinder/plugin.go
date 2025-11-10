package clustercapacitybinder

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	fwk "k8s.io/kube-scheduler/framework"
	framework "k8s.io/kubernetes/pkg/scheduler/framework"
)

const Name = "ClusterCapacityBinder"

type ClusterCapacityBinder struct {
	client       kubernetes.Interface
	postBindHook func(*v1.Pod) error
}

func New(client kubernetes.Interface, _ runtime.Object, _ framework.Handle, postBindHook func(*v1.Pod) error) (framework.Plugin, error) {
	return &ClusterCapacityBinder{
		client:       client,
		postBindHook: postBindHook,
	}, nil
}

func (b *ClusterCapacityBinder) Name() string {
	return Name
}

// TODO(jchaloup): Needs to be locked since the scheduler runs the binding phase in a go routine
func (b *ClusterCapacityBinder) Bind(ctx context.Context, state fwk.CycleState, p *v1.Pod, nodeName string) *fwk.Status {

	pod, err := b.client.CoreV1().Pods(p.Namespace).Get(context.TODO(), p.Name, metav1.GetOptions{})
	if err != nil {
		return fwk.NewStatus(fwk.Error, fmt.Sprintf("Unable to bind: %v", err))
	}
	updatedPod := pod.DeepCopy()
	updatedPod.Spec.NodeName = nodeName
	updatedPod.Status.Phase = v1.PodRunning

	if _, err = b.client.CoreV1().Pods(pod.Namespace).Update(ctx, updatedPod, metav1.UpdateOptions{}); err != nil {
		return fwk.NewStatus(fwk.Error, fmt.Sprintf("Unable to update binded pod: %v", err))
	}

	if err := b.postBindHook(updatedPod); err != nil {
		return fwk.NewStatus(fwk.Error, fmt.Sprintf("Invoking postBindHook gives an error: %v", err))
	}

	return nil
}
