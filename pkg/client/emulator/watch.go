package emulator

import (
	"bytes"
	"io"
	"fmt"
	"time"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/watch"
)

// Every watcher expects infinite byte stream
// E.g. http.Body can provide continuous byte stream,
// each chunk sent asynchronously (invocation of Read is blocked until new chunk)

// Implementation of io.ReadCloser
type WatchBuffer struct {
	buf   *bytes.Buffer
        read  chan []byte
        write chan []byte
        retc  chan retc

	closed bool
}

type retc struct {
	n int
	e error
}

var _ io.ReadCloser = &WatchBuffer{}

// Read watch events as byte stream
func (w *WatchBuffer) Read(p []byte) (n int, err error) {
	w.read <- p
	ret := <-w.retc
	return ret.n, ret.e
}

// Close all channels
func (w *WatchBuffer) Close() error {
	if !w.closed {
		close(w.read)
		close(w.write)
		close(w.retc)
		w.closed = true
	}
	return nil
}

// Write
func (w *WatchBuffer) Write(data []byte) (nr int, err error)  {
	w.write <- data
	return len(data), nil
}

func (c *WatchBuffer) EmitWatchEvent(eType watch.EventType, object runtime.Object) {
	//event := watch.Event{
	//	Type: eType,
	//	Object: object,
	//}
	var buffer bytes.Buffer
	buffer.WriteString("{\"type\":\"")
	buffer.WriteString(string(eType))
	buffer.WriteString("\",\"object\":")

	payload := []byte(buffer.String())
	payload = append(payload, ([]byte)(runtime.EncodeOrDie(testapi.Default.Codec(), object))...)
	payload = append(payload, []byte("}")...)

	c.Write(payload)
}

func (w *WatchBuffer) loop() {
	defer w.Close()

	var dataIn, dataOut []byte

	for {
		select {
			case dataIn = <-w.write:
				_, err := w.buf.Write(dataIn)
				if err != nil {
					// TODO(jchaloup): add log message
					fmt.Println("Write error")
					break
				}
			case dataOut = <-w.read:
				if w.buf.Len() == 0 {
					dataIn = <-w.write
					_, err := w.buf.Write(dataIn)
					if err != nil {
						// TODO(jchaloup): add log message
						fmt.Println("Write error")
						break
					}
				}
				nr, err := w.buf.Read(dataOut)
				w.retc <-retc{nr,err}
		}
	}
}


func NewWatchBuffer() *WatchBuffer {
	wb := &WatchBuffer{
		buf: bytes.NewBuffer(nil),
		read: make(chan []byte),
		write: make(chan []byte),
		retc: make(chan retc),
		closed: false,
	}

	go wb.loop()
	return wb
}

func main() {
	buffer := NewWatchBuffer()

	go func() {
		buffer.Write([]byte("Ahoj"))
		time.Sleep(5*time.Second)
		buffer.Write([]byte(" Svete"))
		//time.Sleep(time.Second)
	}()

	data := []byte{0,0,0,0,0,0}
	buffer.Read(data)
	fmt.Printf("\tdata: %s\n", data)
	buffer.Read(data)
	fmt.Printf("\tdata: %s\n", data)

	time.Sleep(10*time.Second)

	buffer.Close()
	fmt.Println("Ahoj")
}
