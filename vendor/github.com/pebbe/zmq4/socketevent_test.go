package zmq4_test

import (
	zmq "github.com/pebbe/zmq4"

	"fmt"
	"testing"
	"time"
)

func rep_socket_monitor(addr string, chMsg chan<- string) {

	defer close(chMsg)

	s, err := zmq.NewSocket(zmq.PAIR)
	if err != nil {
		chMsg <- fmt.Sprint("NewSocket:", err)
		return
	}
	defer func() {
		s.SetLinger(0)
		s.Close()
	}()

	err = s.Connect(addr)
	if err != nil {
		chMsg <- fmt.Sprint("s.Connect:", err)
		return
	}

	for {
		a, b, _, err := s.RecvEvent(0)
		if err != nil {
			chMsg <- fmt.Sprint("s.RecvEvent:", err)
			return
		}
		chMsg <- fmt.Sprint(a, " ", b)
		if a == zmq.EVENT_CLOSED {
			break
		}
	}
	chMsg <- "Done"
}

func TestSocketEvent(t *testing.T) {

	var rep *zmq.Socket
	defer func() {
		if rep != nil {
			rep.SetLinger(0)
			rep.Close()
		}
	}()

	// REP socket
	rep, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	// REP socket monitor, all events
	err = rep.Monitor("inproc://monitor.rep", zmq.EVENT_ALL)
	if err != nil {
		t.Fatal("rep.Monitor:", err)
	}
	chMsg := make(chan string, 10)
	go rep_socket_monitor("inproc://monitor.rep", chMsg)
	time.Sleep(time.Second)

	// Generate an event
	err = rep.Bind("tcp://*:9689")
	if err != nil {
		t.Fatal("rep.Bind:", err)
	}

	rep.Close()
	rep = nil

	expect := []string{
		"EVENT_LISTENING tcp://0.0.0.0:9689",
		"EVENT_CLOSED tcp://0.0.0.0:9689",
		"Done",
	}
	i := 0
	for msg := range chMsg {
		if i < len(expect) {
			if msg != expect[i] {
				t.Errorf("Expected message %q, got %q", expect[i], msg)
			}
			i++
		} else {
			t.Error("Unexpected message: %q", msg)
		}
	}
	for ; i < len(expect); i++ {
		t.Errorf("Expected message %q, got nothing", expect[i])
	}
}
