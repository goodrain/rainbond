package zmq4_test

import (
	zmq "github.com/pebbe/zmq4"

	"testing"
)

func TestRemoteEndpoint(t *testing.T) {

	if _, minor, _ := zmq.Version(); minor < 1 {
		t.Skip("RemoteEndpoint not avalable in ZeroMQ versions prior to 4.1.0")
	}

	addr := "tcp://127.0.0.1:9560"
	peer := "127.0.0.1"

	var rep, req *zmq.Socket
	defer func() {
		for _, s := range []*zmq.Socket{rep, req} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	rep, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	req, err = zmq.NewSocket(zmq.REQ)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	if err = rep.Bind(addr); err != nil {
		t.Fatal("rep.Bind:", err)
	}
	if err = req.Connect(addr); err != nil {
		t.Fatal("req.Connect:", err)
	}

	tmp := "test"
	if _, err = req.Send(tmp, 0); err != nil {
		t.Fatal("req.Send:", err)
	}

	// get message with peer address (remote endpoint)
	msg, props, err := rep.RecvWithMetadata(0, "Peer-Address")
	if err != nil {
		t.Fatal("rep.RecvWithMetadata:", err)
		return
	}
	if msg != tmp {
		t.Errorf("rep.RecvWithMetadata: expected %q, got %q", tmp, msg)
	}

	if p := props["Peer-Address"]; p != peer {
		t.Errorf("rep.RecvWithMetadata: expected Peer-Address == %q, got %q", peer, p)
	}

	err = rep.Close()
	rep = nil
	if err != nil {
		t.Fatal("rep.Close:", err)
	}

	err = req.Close()
	req = nil
	if err != nil {
		t.Fatal("req.Close:", err)
	}
}
