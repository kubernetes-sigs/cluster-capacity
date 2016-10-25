package apiserver

import "sync"

//
type WatchChannelDistributor struct {
	inputChannel chan *Report
	outputChannels []chan *Report
	mux sync.Mutex
}

func NewWatchChannelDistributor(input chan *Report) *WatchChannelDistributor {
	return &WatchChannelDistributor{
		inputChannel: input,
		outputChannels: make([]chan *Report, 0),
	}
}

func (w *WatchChannelDistributor) Run() {
	for {
		select {
		case report := <-w.inputChannel:
			for i := 0; i < len(w.outputChannels); i++ {
				w.outputChannels[i] <- report
			}
		}
	}
}

func (w *WatchChannelDistributor) AddChannel(ch chan *Report) int {
	w.mux.Lock()
	defer w.mux.Unlock()
	pos := len(w.outputChannels)
	w.outputChannels = append(w.outputChannels, ch)
	return pos
}

func (w *WatchChannelDistributor) RemoveChannel(pos int) {
	w.mux.Lock()
	defer w.mux.Unlock()
	copy(w.outputChannels[pos:], w.outputChannels[pos+1:])
	w.outputChannels[len(w.outputChannels)-1] = nil
	w.outputChannels = w.outputChannels[:len(w.outputChannels)-1]
}