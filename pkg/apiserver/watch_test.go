package apiserver

import (
	"testing"
	"github.com/ingvagabund/cluster-capacity/pkg/framework"
	"time"
)

func exampleReport() *framework.Report {
	revolution, err := time.Parse(TIMELAYOUT, "2000-01-01T00:00:00+00:00")
	if err != nil {
		panic(err)
	}
	return &framework.Report{
		Timestamp: revolution,
		TotalInstances: 0,
	}
}
func TestWatchChannelDistributor_AddChannel(t *testing.T) {
	input := make(chan *framework.Report)
	outputChannels := make([]chan *framework.Report, 0)
	wcd := NewWatchChannelDistributor(input)

	go wcd.Run()

	for i := 0; i < 3; i++ {
		out := make(chan *framework.Report)
		outputChannels = append(outputChannels, out)
		pos, err := wcd.AddChannel(out)
		if pos != i {
			t.Fatalf("Unexpected channel index: Actual: %v, Expected: %v", pos, i)
		}
		if err != nil{
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	input <- exampleReport()

	for i := 3; i < MAXWATCHERS; i++ {
		out := make(chan *framework.Report)
		outputChannels = append(outputChannels, out)
		pos, err := wcd.AddChannel(out)
		if pos != i {
			t.Fatalf("Unexpected channel index: Actual: %v, Expected: %v", pos, i)
		}
		if err != nil{
			t.Fatalf("Unexpected error: %v", err)
		}
	}
	out := make(chan *framework.Report)
	_, err := wcd.AddChannel(out)
	if err == nil {
		t.Errorf("Expected error not found: number of channels shouldn't exceed MAXREPORTS value")
	}

}

func TestWatchChannelDistributor_RemoveChannel(t *testing.T) {
	input := make(chan *framework.Report)
	outputChannels := make([]chan *framework.Report, 0)
	wcd := NewWatchChannelDistributor(input)

	go wcd.Run()

	for i := 0; i < 3; i++ {
		out := make(chan *framework.Report)
		outputChannels = append(outputChannels, out)
		pos, err := wcd.AddChannel(out)
		if pos != i {
			t.Fatalf("Unexpected channel index: Actual: %v, Expected: %v", pos, i)
		}
		if err != nil{
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	input <- exampleReport()

	for i := 0; i < 2; i++ {
		wcd.RemoveChannel(i)
	}
	result := <- outputChannels[2]
		if result != exampleReport() {
			t.Fatalf("Output not correct: Expected: %v, Actual: %v", exampleReport(), result)
		}
}