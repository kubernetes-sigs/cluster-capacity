package apiserver

import (
	"reflect"
	"testing"
	"time"

	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework"
)

func exampleReport() *framework.Report {
	revolution, err := time.Parse(TIMELAYOUT, "2000-01-01T00:00:00+00:00")
	if err != nil {
		panic(err)
	}
	return &framework.Report{
		Timestamp:      revolution,
		TotalInstances: 0,
	}
}

func TestWatchChannelDistributor_AddChannel(t *testing.T) {
	outputChannels := make([]*WatchChannel, 0)
	wcd := NewWatchChannelDistributor()

	go wcd.Run()

	for i := 0; i < 3; i++ {
		out, err := wcd.NewChannel()
		outputChannels = append(outputChannels, out)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	wcd.Broadcast(exampleReport())

	for i := 3; i < MAXWATCHERS; i++ {
		out, err := wcd.NewChannel()
		outputChannels = append(outputChannels, out)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	_, err := wcd.NewChannel()
	if err == nil {
		t.Errorf("Expected error not found: number of channels shouldn't exceed MAXREPORTS value")
	}

}

func TestWatchChannelDistributor_RemoveChannel(t *testing.T) {
	outputChannels := make([]*WatchChannel, 0)
	wcd := NewWatchChannelDistributor()

	go wcd.Run()

	for i := 0; i < 3; i++ {
		out, err := wcd.NewChannel()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		outputChannels = append(outputChannels, out)
	}

	eReport := exampleReport()

	wcd.Broadcast(eReport)

	for i := 0; i < 2; i++ {
		// read from the channel and close it
		<-outputChannels[i].Chan()
		outputChannels[i].Close()
	}

	result := <-outputChannels[2].Chan()
	if !reflect.DeepEqual(result, eReport) {
		t.Fatalf("Output not correct: Expected: %v, Actual: %v", eReport, result)
	}
}
