package apiserver

import (
	"fmt"
	"sync"

	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework"
)

var MAXWATCHERS = 10

const channel_cap = 1

type WatchChannelDistributor struct {
	inputChannel   chan *framework.Report
	outputChannels []chan *framework.Report
	mux            sync.Mutex
	remMux         sync.Mutex
}

func NewWatchChannelDistributor() *WatchChannelDistributor {
	return &WatchChannelDistributor{
		inputChannel:   make(chan *framework.Report),
		outputChannels: make([]chan *framework.Report, 0),
	}
}

type WatchChannel struct {
	w   *WatchChannelDistributor
	c   chan *framework.Report
	pos int
}

func (wc *WatchChannel) Chan() chan *framework.Report {
	return wc.c
}

func (wc *WatchChannel) Close() {
	wc.w.RemoveChannel(wc.pos)
}

// Each channel needs its own worker
func (w *WatchChannelDistributor) NewChannel() (*WatchChannel, error) {
	w.mux.Lock()
	defer w.mux.Unlock()

	for i := 0; i < len(w.outputChannels); i++ {
		if w.outputChannels[i] == nil {
			// set the channel capacity to 1
			// if the channel is full, do not send anything to it
			w.outputChannels[i] = make(chan *framework.Report, channel_cap)
			return &WatchChannel{
				w:   w,
				c:   w.outputChannels[i],
				pos: i,
			}, nil
		}
	}

	if len(w.outputChannels) >= MAXWATCHERS {
		return nil, fmt.Errorf("Maximum number of watches exceeded\n")
	}

	ch := make(chan *framework.Report, channel_cap)
	w.outputChannels = append(w.outputChannels, ch)
	return &WatchChannel{
		w:   w,
		c:   ch,
		pos: len(w.outputChannels) - 1,
	}, nil
}

func (w *WatchChannelDistributor) Run() {
	for {
		select {
		case report := <-w.inputChannel:

			func() {
				w.remMux.Lock()
				defer w.remMux.Unlock()
				for i := 0; i < len(w.outputChannels); i++ {
					if w.outputChannels[i] != nil && channel_cap > len(w.outputChannels[i]) {
						w.outputChannels[i] <- report
					}
				}
			}()

		}
	}
}

func (w *WatchChannelDistributor) Broadcast(r *framework.Report) {
	w.inputChannel <- r
}

func (w *WatchChannelDistributor) RemoveChannel(pos int) {
	w.mux.Lock()
	defer w.mux.Unlock()
	w.remMux.Lock()
	defer w.remMux.Unlock()
	close(w.outputChannels[pos])
	w.outputChannels[pos] = nil
}
