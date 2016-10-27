package record

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/runtime"
)

type Event struct {
	Eventtype string
	Reason    string
	Message   string
}

type Recorder struct {
	Events chan Event
}

func (e *Recorder) Event(object runtime.Object, eventtype, reason, message string) {
	if e.Events != nil {
		e.Events <- Event{eventtype, reason, message}
	}
}

func (e *Recorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	if e.Events != nil {
		e.Events <- Event{eventtype, reason, fmt.Sprintf(messageFmt, args...)}
	}
}

func (e *Recorder) PastEventf(object runtime.Object, timestamp unversioned.Time, eventtype, reason, messageFmt string, args ...interface{}) {

}

// NewFakeRecorder creates new fake event recorder with event channel with
// buffer of given size.
func NewRecorder(bufferSize int) *Recorder {
	return &Recorder{
		Events: make(chan Event, bufferSize),
	}
}

var _ record.EventRecorder = &Recorder{}
