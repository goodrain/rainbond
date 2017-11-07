package zmq4_test

import (
	zmq "github.com/pebbe/zmq4"

	"errors"
	"fmt"
	"runtime"
	"strconv"
	"testing"
	"time"
)

var (
	err32 = errors.New("rc != 32")
)

func TestVersion(t *testing.T) {
	major, _, _ := zmq.Version()
	if major != 4 {
		t.Errorf("Expected major version 4, got %d", major)
	}
}

func TestMultipleContexts(t *testing.T) {

	chQuit := make(chan interface{})
	chErr := make(chan error, 2)
	needQuit := false
	var sock1, sock2, serv1, serv2 *zmq.Socket
	var serv_ctx1, serv_ctx2, ctx1, ctx2 *zmq.Context
	var err error

	defer func() {
		if needQuit {
			chQuit <- true
			chQuit <- true
			<-chErr
			<-chErr
		}
		for _, s := range []*zmq.Socket{sock1, sock2, serv1, serv2} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
		for _, c := range []*zmq.Context{serv_ctx1, serv_ctx2, ctx1, ctx2} {
			if c != nil {
				c.Term()
			}
		}
	}()

	addr1 := "tcp://127.0.0.1:9997"
	addr2 := "tcp://127.0.0.1:9998"

	serv_ctx1, err = zmq.NewContext()
	if err != nil {
		t.Fatal("NewContext:", err)
	}
	serv1, err = serv_ctx1.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = serv1.Bind(addr1)
	if err != nil {
		t.Fatal("Bind:", err)
	}

	serv_ctx2, err = zmq.NewContext()
	if err != nil {
		t.Fatal("NewContext:", err)
	}
	serv2, err = serv_ctx2.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = serv2.Bind(addr2)
	if err != nil {
		t.Fatal("Bind:", err)
	}

	new_service := func(sock *zmq.Socket, addr string) {
		socket_handler := func(state zmq.State) error {
			msg, err := sock.RecvMessage(0)
			if err != nil {
				return err
			}
			_, err = sock.SendMessage(addr, msg)
			return err
		}
		quit_handler := func(interface{}) error {
			return errors.New("quit")
		}

		reactor := zmq.NewReactor()
		reactor.AddSocket(sock, zmq.POLLIN, socket_handler)
		reactor.AddChannel(chQuit, 1, quit_handler)
		err = reactor.Run(100 * time.Millisecond)
		chErr <- err
	}

	go new_service(serv1, addr1)
	go new_service(serv2, addr2)
	needQuit = true

	time.Sleep(time.Second)

	// default context

	sock1, err = zmq.NewSocket(zmq.REQ)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	sock2, err = zmq.NewSocket(zmq.REQ)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = sock1.Connect(addr1)
	if err != nil {
		t.Fatal("sock1.Connect:", err)
	}
	err = sock2.Connect(addr2)
	if err != nil {
		t.Fatal("sock2.Connect:", err)
	}
	_, err = sock1.SendMessage(addr1)
	if err != nil {
		t.Fatal("sock1.SendMessage:", err)
	}
	_, err = sock2.SendMessage(addr2)
	if err != nil {
		t.Fatal("sock2.SendMessage:", err)
	}
	msg, err := sock1.RecvMessage(0)
	expected := []string{addr1, addr1}
	if err != nil || !arrayEqual(msg, expected) {
		t.Errorf("sock1.RecvMessage: expected %v %v, got %v %v", nil, expected, err, msg)
	}
	msg, err = sock2.RecvMessage(0)
	expected = []string{addr2, addr2}
	if err != nil || !arrayEqual(msg, expected) {
		t.Errorf("sock2.RecvMessage: expected %v %v, got %v %v", nil, expected, err, msg)
	}
	err = sock1.Close()
	sock1 = nil
	if err != nil {
		t.Fatal("sock1.Close:", err)
	}
	err = sock2.Close()
	sock2 = nil
	if err != nil {
		t.Fatal("sock2.Close:", err)
	}

	// non-default contexts

	ctx1, err = zmq.NewContext()
	if err != nil {
		t.Fatal("NewContext:", err)
	}
	ctx2, err = zmq.NewContext()
	if err != nil {
		t.Fatal("NewContext:", err)
	}
	sock1, err = ctx1.NewSocket(zmq.REQ)
	if err != nil {
		t.Fatal("ctx1.NewSocket:", err)
	}
	sock2, err = ctx2.NewSocket(zmq.REQ)
	if err != nil {
		t.Fatal("ctx2.NewSocket:", err)
	}
	err = sock1.Connect(addr1)
	if err != nil {
		t.Fatal("sock1.Connect:", err)
	}
	err = sock2.Connect(addr2)
	if err != nil {
		t.Fatal("sock2.Connect:", err)
	}
	_, err = sock1.SendMessage(addr1)
	if err != nil {
		t.Fatal("sock1.SendMessage:", err)
	}
	_, err = sock2.SendMessage(addr2)
	if err != nil {
		t.Fatal("sock2.SendMessage:", err)
	}
	msg, err = sock1.RecvMessage(0)
	expected = []string{addr1, addr1}
	if err != nil || !arrayEqual(msg, expected) {
		t.Errorf("sock1.RecvMessage: expected %v %v, got %v %v", nil, expected, err, msg)
	}
	msg, err = sock2.RecvMessage(0)
	expected = []string{addr2, addr2}
	if err != nil || !arrayEqual(msg, expected) {
		t.Errorf("sock2.RecvMessage: expected %v %v, got %v %v", nil, expected, err, msg)
	}
	err = sock1.Close()
	sock1 = nil
	if err != nil {
		t.Fatal("sock1.Close:", err)
	}
	err = sock2.Close()
	sock2 = nil
	if err != nil {
		t.Fatal("sock2.Close:", err)
	}

	err = ctx1.Term()
	ctx1 = nil
	if err != nil {
		t.Fatal("ctx1.Term", nil)
	}
	err = ctx2.Term()
	ctx1 = nil
	if err != nil {
		t.Fatal("ctx2.Term", nil)
	}

	needQuit = false
	for i := 0; i < 2; i++ {
		// close(chQuit) doesn't work because the reactor removes closed channels, instead of acting on them
		chQuit <- true
		err = <-chErr
		if err.Error() != "quit" {
			t.Errorf("Expected error value quit, got %v", err)
		}
	}
}

func TestAbstractIpc(t *testing.T) {

	var sb, sc *zmq.Socket
	defer func() {
		for _, s := range []*zmq.Socket{sb, sc} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	addr := "ipc://@/tmp/tester"

	// This is Linux only
	if runtime.GOOS != "linux" {
		t.Skip("Only on Linux")
	}

	sb, err := zmq.NewSocket(zmq.PAIR)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	err = sb.Bind(addr)
	if err != nil {
		t.Fatal("sb.Bind:", err)
	}

	endpoint, err := sb.GetLastEndpoint()
	expected := "ipc://@/tmp/tester"
	if endpoint != expected || err != nil {
		t.Fatalf("sb.GetLastEndpoint: expected 'nil' %q, got '%v' %q", expected, err, endpoint)
		return
	}

	sc, err = zmq.NewSocket(zmq.PAIR)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = sc.Connect(addr)
	if err != nil {
		t.Fatal("sc.Bind:", err)
	}

	resp, err := bounce(sb, sc)
	if err != nil {
		t.Error(resp, err)
	}

	err = sc.Close()
	sc = nil
	if err != nil {
		t.Fatal("sc.Close:", err)
	}

	err = sb.Close()
	sb = nil
	if err != nil {
		t.Fatal("sb.Close:", err)
	}
}

func TestConflate(t *testing.T) {

	var s_in, s_out *zmq.Socket
	defer func() {
		for _, s := range []*zmq.Socket{s_in, s_out} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	bind_to := "tcp://127.0.0.1:5555"

	err := zmq.SetIoThreads(1)
	if err != nil {
		t.Fatal("SetIoThreads(1):", err)
	}

	s_in, err = zmq.NewSocket(zmq.PULL)
	if err != nil {
		t.Fatal("NewSocket 1:", err)
	}

	err = s_in.SetConflate(true)
	if err != nil {
		t.Fatal("SetConflate(true):", err)
	}

	err = s_in.Bind(bind_to)
	if err != nil {
		t.Fatal("s_in.Bind:", err)
	}

	s_out, err = zmq.NewSocket(zmq.PUSH)
	if err != nil {
		t.Fatal("NewSocket 2:", err)
	}

	err = s_out.Connect(bind_to)
	if err != nil {
		t.Fatal("s_out.Connect:", err)
	}

	message_count := 20

	for j := 0; j < message_count; j++ {
		_, err = s_out.Send(fmt.Sprint(j), 0)
		if err != nil {
			t.Fatalf("s_out.Send %d: %v", j, err)
		}
	}

	time.Sleep(time.Second)

	payload_recved, err := s_in.Recv(0)
	if err != nil {
		t.Error("s_in.Recv:", err)
	} else {
		i, err := strconv.Atoi(payload_recved)
		if err != nil {
			t.Error("strconv.Atoi:", err)
		}
		if i != message_count-1 {
			t.Error("payload_recved != message_count - 1")
		}
	}

	err = s_in.Close()
	s_in = nil
	if err != nil {
		t.Error("s_in.Close:", err)
	}

	err = s_out.Close()
	s_out = nil
	if err != nil {
		t.Error("s_out.Close:", err)
	}
}

func TestConnectResolve(t *testing.T) {

	sock, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	defer func() {
		if sock != nil {
			sock.SetLinger(0)
			sock.Close()
		}
	}()

	err = sock.Connect("tcp://localhost:1234")
	if err != nil {
		t.Error("sock.Connect:", err)
	}

	fails := []string{
		"tcp://localhost:invalid",
		"tcp://in val id:1234",
		"invalid://localhost:1234",
	}
	for _, fail := range fails {
		if err = sock.Connect(fail); err == nil {
			t.Errorf("Connect %s, expected fail, got success", fail)
		}
	}

	err = sock.Close()
	sock = nil
	if err != nil {
		t.Error("sock.Close:", err)
	}
}

func TestCtxOptions(t *testing.T) {

	type Result struct {
		value interface{}
		err   error
	}

	i, err := zmq.GetMaxSockets()
	if err != nil {
		t.Error("GetMaxSockets:", err)
	}
	if i != zmq.MaxSocketsDflt {
		t.Errorf("MaxSockets != MaxSocketsDflt: %d != %d", i, zmq.MaxSocketsDflt)
	}

	i, err = zmq.GetIoThreads()
	if err != nil {
		t.Error("GetIoThreads:", err)
	}
	if i != zmq.IoThreadsDflt {
		t.Errorf("IoThreads != IoThreadsDflt: %d != %d", i, zmq.IoThreadsDflt)
	}

	b, err := zmq.GetIpv6()
	if b != false || err != nil {
		t.Errorf("GetIpv6 1: expected false <nil>, got %v %v", b, err)
	}

	zmq.SetIpv6(true)
	defer zmq.SetIpv6(false)
	b, err = zmq.GetIpv6()
	if b != true || err != nil {
		t.Errorf("GetIpv6 2: expected true <nil>, got %v %v", b, err)
	}

	router, _ := zmq.NewSocket(zmq.ROUTER)
	b, err = router.GetIpv6()
	if b != true || err != nil {
		t.Errorf("GetIpv6 3: expected true <nil>, got %v %v", b, err)
	}
	router.Close()
}

func TestDisconnectInproc(t *testing.T) {

	var pubSocket, subSocket *zmq.Socket
	defer func() {
		for _, s := range []*zmq.Socket{pubSocket, subSocket} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	publicationsReceived := 0
	isSubscribed := false

	pubSocket, err := zmq.NewSocket(zmq.XPUB)
	if err != nil {
		t.Fatal("NewSocket XPUB:", err)
	}
	subSocket, err = zmq.NewSocket(zmq.SUB)
	if err != nil {
		t.Fatal("NewSocket SUB:", err)
	}
	err = subSocket.SetSubscribe("foo")
	if err != nil {
		t.Fatal("subSocket.SetSubscribe:", err)
	}

	err = pubSocket.Bind("inproc://someInProcDescriptor")
	if err != nil {
		t.Fatal("pubSocket.Bind:", err)
	}

	iteration := 0

	poller := zmq.NewPoller()
	poller.Add(subSocket, zmq.POLLIN) // read publications
	poller.Add(pubSocket, zmq.POLLIN) // read subscriptions
	for {
		sockets, err := poller.Poll(100 * time.Millisecond)
		if err != nil {
			t.Error("Poll:", err)
			break //  Interrupted
		}

		for _, socket := range sockets {
			if socket.Socket == pubSocket {
				for {
					buffer, err := pubSocket.Recv(0)
					if err != nil {
						t.Fatal("pubSocket.Recv", err)
					}
					exp := "\x01foo"
					if isSubscribed {
						exp = "\x00foo"
					}
					if buffer != exp {
						t.Errorf("pubSocket.Recv: expected %q, got %q", exp, buffer)
					}

					if buffer[0] == 0 {
						if isSubscribed != true {
							t.Errorf("Poller: expected subscribed")
						}
						isSubscribed = false
					} else {
						if isSubscribed != false {
							t.Errorf("Poller: expected not subscribed")
						}
						isSubscribed = true
					}

					more, err := pubSocket.GetRcvmore()
					if err != nil {
						t.Fatal("pubSocket.GetRcvmore:", err)
					}
					if !more {
						break //  Last message part
					}
				}
				break
			}
		}

		for _, socket := range sockets {
			if socket.Socket == subSocket {
				for _, exp := range []string{"foo", "this is foo!", "", ""} {
					msg, err := subSocket.Recv(0)
					if err != nil {
						t.Fatal("subSocket.Recv:", err)
					}
					if msg != exp {
						t.Errorf("subSocket.Recv: expected %q, got %q", exp, msg)
					}
					more, err := subSocket.GetRcvmore()
					if err != nil {
						t.Fatal("subSocket.GetRcvmore:", err)
					}
					if !more {
						publicationsReceived++
						break //  Last message part
					}

				}
				break
			}
		}

		if iteration == 1 {
			err := subSocket.Connect("inproc://someInProcDescriptor")
			if err != nil {
				t.Fatal("subSocket.Connect", err)
			}
		}
		if iteration == 4 {
			err := subSocket.Disconnect("inproc://someInProcDescriptor")
			if err != nil {
				t.Fatal("subSocket.Disconnect", err)
			}
		}
		if iteration > 4 && len(sockets) == 0 {
			break
		}

		_, err = pubSocket.Send("foo", zmq.SNDMORE)
		if err != nil {
			t.Fatal("pubSocket.Send 1", err)
		}
		_, err = pubSocket.Send("this is foo!", 0)
		if err != nil {
			t.Fatal("pubSocket.Send 2", err)
		}

		iteration++

	}

	if publicationsReceived != 3 {
		t.Error("publicationsReceived != 3 ")
	}
	if isSubscribed {
		t.Error("isSubscribed")
	}

	err = pubSocket.Close()
	pubSocket = nil
	if err != nil {
		t.Error("pubSocket.Close:", err)
	}
	err = subSocket.Close()
	subSocket = nil
	if err != nil {
		t.Error("subSocket.Close:", err)
	}
}

func TestFork(t *testing.T) {

	address := "tcp://127.0.0.1:6571"
	NUM_MESSAGES := 5

	//  Create and bind pull socket to receive messages
	pull, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	defer func() {
		if pull != nil {
			pull.SetLinger(0)
			pull.Close()
		}
	}()
	err = pull.Bind(address)
	if err != nil {
		t.Fatal("pull.Bind:", err)
	}

	ready := make(chan bool)

	go func() {
		defer func() {
			close(ready)
		}()

		//  Create new socket, connect and send some messages

		push, err := zmq.NewSocket(zmq.PUSH)
		//err = fmt.Errorf("DUMMY ERROR")
		if err != nil {
			t.Error("NewSocket:", err)
			return
		}
		defer func() {
			err := push.Close()
			if err != nil {
				t.Error("push.Close:", err)
			}
		}()

		err = push.Connect(address)
		if err != nil {
			t.Error("push.Connect:", err)
			return
		}

		for count := 0; count < NUM_MESSAGES; count++ {
			ready <- true
			_, err = push.Send("Hello", 0)
			if err != nil {
				t.Error("push.Send:", err)
				return
			}
		}

	}()

	for {
		if r := <-ready; !r {
			break
		}
		msg, err := pull.Recv(0)
		if err != nil {
			t.Error("pull.Recv:", err)
		}
		if msg != "Hello" {
			t.Errorf("pull.Recv: expected \"Hello\", got %q", msg)
		}
	}

	err = pull.Close()
	pull = nil
	if err != nil {
		t.Error("pull.Close", err)
	}

	<-ready // false
}

func TestHwm(t *testing.T) {

	MAX_SENDS := 10000
	BIND_FIRST := 1
	CONNECT_FIRST := 2

	test_defaults := func() (result int) {

		result = -1

		// Set up bind socket
		bind_socket, err := zmq.NewSocket(zmq.PULL)
		if err != nil {
			t.Error("NewSocket:", err)
			return
		}
		defer func() {
			err := bind_socket.Close()
			if err != nil {
				t.Error("bind_socket.Close:", err)
			}
		}()

		err = bind_socket.Bind("inproc://a")
		if err != nil {
			t.Error("bind_socket.Bind:", err)
			return
		}

		// Set up connect socket
		connect_socket, err := zmq.NewSocket(zmq.PUSH)
		if err != nil {
			t.Error("NewSocket:", err)
			return
		}
		defer func() {
			err := connect_socket.Close()
			if err != nil {
				t.Error("connect_socket.Close:", err)
			}
		}()

		err = connect_socket.Connect("inproc://a")
		if err != nil {
			t.Error("connect_socket.Connect:", err)
			return
		}

		// Send until we block
		send_count := 0
		for send_count < MAX_SENDS {
			_, err := connect_socket.Send("", zmq.DONTWAIT)
			if err != nil {
				break
			}
			send_count++
		}

		// Now receive all sent messages
		recv_count := 0
		for {
			_, err := bind_socket.Recv(zmq.DONTWAIT)
			if err != nil {
				break
			}
			recv_count++
		}
		if send_count != recv_count {
			t.Error("test_defaults: send_count == recv_count")
		}

		return send_count
	}

	count_msg := func(send_hwm, recv_hwm, testType int) (result int) {

		result = -1

		var bind_socket, connect_socket *zmq.Socket
		var err error

		if testType == BIND_FIRST {
			// Set up bind socket
			bind_socket, err = zmq.NewSocket(zmq.PULL)
			if err != nil {
				t.Error("NewSocket:", err)
				return
			}
			defer func() {
				err := bind_socket.Close()
				if err != nil {
					t.Error("bind_socket.Close:", err)
				}
			}()

			err = bind_socket.SetRcvhwm(recv_hwm)
			if err != nil {
				t.Error("bind_socket.SetRcvhwm:", err)
				return
			}

			err = bind_socket.Bind("inproc://a")
			if err != nil {
				t.Error("bind_socket.Bind:", err)
				return
			}

			// Set up connect socket
			connect_socket, err = zmq.NewSocket(zmq.PUSH)
			if err != nil {
				t.Error("NewSocket:", err)
				return
			}
			defer func() {
				err := connect_socket.Close()
				if err != nil {
					t.Error(err)
				}
			}()

			err = connect_socket.SetSndhwm(send_hwm)
			if err != nil {
				t.Error("connect_socket.SetSndhwm:", err)
				return
			}

			err = connect_socket.Connect("inproc://a")
			if err != nil {
				t.Error("connect_socket.Connect:", err)
				return
			}
		} else {
			// Set up connect socket
			connect_socket, err = zmq.NewSocket(zmq.PUSH)
			if err != nil {
				t.Error("NewSocket:", err)
				return
			}
			defer func() {
				err := connect_socket.Close()
				if err != nil {
					t.Error("connect_socket.Close:", err)
				}
			}()

			err = connect_socket.SetSndhwm(send_hwm)
			if err != nil {
				t.Error("connect_socket.SetSndhwm:", err)
				return
			}

			err = connect_socket.Connect("inproc://a")
			if err != nil {
				t.Error("connect_socket.Connect:", err)
				return
			}

			// Set up bind socket
			bind_socket, err = zmq.NewSocket(zmq.PULL)
			if err != nil {
				t.Error("NewSocket:", err)
				return
			}
			defer func() {
				err := bind_socket.Close()
				if err != nil {
					t.Error("bind_socket.Close:", err)
				}
			}()

			err = bind_socket.SetRcvhwm(recv_hwm)
			if err != nil {
				t.Error("bind_socket.SetRcvhwm:", err)
				return
			}

			err = bind_socket.Bind("inproc://a")
			if err != nil {
				t.Error("bind_socket.Bind:", err)
				return
			}
		}

		// Send until we block
		send_count := 0
		for send_count < MAX_SENDS {
			_, err := connect_socket.Send("", zmq.DONTWAIT)
			if err != nil {
				break
			}
			send_count++
		}

		// Now receive all sent messages
		recv_count := 0
		for {
			_, err := bind_socket.Recv(zmq.DONTWAIT)
			if err != nil {
				break
			}
			recv_count++
		}
		if send_count != recv_count {
			t.Error("count_msg: send_count != recv_count")
		}

		// Now it should be possible to send one more.
		_, err = connect_socket.Send("", 0)
		if err != nil {
			t.Error("connect_socket.Send:", err)
			return
		}

		//  Consume the remaining message.
		_, err = bind_socket.Recv(0)
		if err != nil {
			t.Error("bind_socket.Recv:", err)
		}

		return send_count
	}

	test_inproc_bind_first := func(send_hwm, recv_hwm int) int {
		return count_msg(send_hwm, recv_hwm, BIND_FIRST)
	}

	test_inproc_connect_first := func(send_hwm, recv_hwm int) int {
		return count_msg(send_hwm, recv_hwm, CONNECT_FIRST)
	}

	test_inproc_connect_and_close_first := func(send_hwm, recv_hwm int) (result int) {

		result = -1

		// Set up connect socket
		connect_socket, err := zmq.NewSocket(zmq.PUSH)
		if err != nil {
			t.Error("NewSocket:", err)
			return
		}
		defer func() {
			if connect_socket != nil {
				connect_socket.Close()
			}
		}()

		err = connect_socket.SetSndhwm(send_hwm)
		if err != nil {
			t.Error("connect_socket.SetSndhwm:", err)
			return
		}

		err = connect_socket.Connect("inproc://a")
		if err != nil {
			t.Error("connect_socket.Connect:", err)
			return
		}

		// Send until we block
		send_count := 0
		for send_count < MAX_SENDS {
			_, err := connect_socket.Send("", zmq.DONTWAIT)
			if err != nil {
				break
			}
			send_count++
		}

		// Close connect
		err = connect_socket.Close()
		connect_socket = nil
		if err != nil {
			t.Error("connect_socket.Close:", err)
			return
		}

		// Set up bind socket
		bind_socket, err := zmq.NewSocket(zmq.PULL)
		if err != nil {
			t.Error("NewSocket:", err)
			return
		}
		defer func() {
			err := bind_socket.Close()
			if err != nil {
				t.Error("bind_socket.Close:", err)
			}
		}()

		err = bind_socket.SetRcvhwm(recv_hwm)
		if err != nil {
			t.Error("bind_socket.SetRcvhwm:", err)
			return
		}

		err = bind_socket.Bind("inproc://a")
		if err != nil {
			t.Error("bind_socket.Bind:", err)
			return
		}

		// Now receive all sent messages
		recv_count := 0
		for {
			_, err := bind_socket.Recv(zmq.DONTWAIT)
			if err != nil {
				break
			}
			recv_count++
		}
		if send_count != recv_count {
			t.Error("test_inproc_connect_and_close_first: send_count != recv_count")
		}
		return send_count
	}

	// Default values are 1000 on send and 1000 one receive, so 2000 total
	if count := test_defaults(); count != 2000 {
		t.Errorf("test_defaults: expected 2000, got %d", count)
	}
	time.Sleep(100 * time.Millisecond)

	// Infinite send and receive buffer
	if count := test_inproc_bind_first(0, 0); count != MAX_SENDS {
		t.Errorf("test_inproc_bind_first(0, 0): expected %d, got %d", MAX_SENDS, count)
	}
	time.Sleep(100 * time.Millisecond)
	if count := test_inproc_connect_first(0, 0); count != MAX_SENDS {
		t.Errorf("test_inproc_connect_first(0, 0): expected %d, got %d", MAX_SENDS, count)
	}
	time.Sleep(100 * time.Millisecond)

	// Infinite send buffer
	if count := test_inproc_bind_first(1, 0); count != MAX_SENDS {
		t.Errorf("test_inproc_bind_first(1, 0): expected %d, got %d", MAX_SENDS, count)
	}
	time.Sleep(100 * time.Millisecond)
	if count := test_inproc_connect_first(1, 0); count != MAX_SENDS {
		t.Errorf("test_inproc_connect_first(1, 0): expected %d, got %d", MAX_SENDS, count)
	}
	time.Sleep(100 * time.Millisecond)

	// Infinite receive buffer
	if count := test_inproc_bind_first(0, 1); count != MAX_SENDS {
		t.Errorf("test_inproc_bind_first(0, 1): expected %d, got %d", MAX_SENDS, count)
	}
	time.Sleep(100 * time.Millisecond)
	if count := test_inproc_connect_first(0, 1); count != MAX_SENDS {
		t.Errorf("test_inproc_connect_first(0, 1): expected %d, got %d", MAX_SENDS, count)
	}
	time.Sleep(100 * time.Millisecond)

	// Send and recv buffers hwm 1, so total that can be queued is 2
	if count := test_inproc_bind_first(1, 1); count != 2 {
		t.Errorf("test_inproc_bind_first(1, 1): expected 2, got %d", count)
	}
	time.Sleep(100 * time.Millisecond)
	if count := test_inproc_connect_first(1, 1); count != 2 {
		t.Errorf("test_inproc_connect_first(1, 1): expected 2, got %d", count)
	}
	time.Sleep(100 * time.Millisecond)

	// Send hwm of 1, send before bind so total that can be queued is 1
	if count := test_inproc_connect_and_close_first(1, 0); count != 1 {
		t.Errorf("test_inproc_connect_and_close_first(1, 0): expected 1, got %d", count)
	}
	time.Sleep(100 * time.Millisecond)
}

func TestPairIpc(t *testing.T) {

	var sb, sc *zmq.Socket

	defer func() {
		for _, s := range []*zmq.Socket{sb, sc} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	sb, err := zmq.NewSocket(zmq.PAIR)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	err = sb.Bind("ipc:///tmp/tester")
	if err != nil {
		t.Fatal("sb.Bind:", err)
	}

	sc, err = zmq.NewSocket(zmq.PAIR)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	err = sc.Connect("ipc:///tmp/tester")
	if err != nil {
		t.Fatal("sc.Connect:", err)
	}

	msg, err := bounce(sb, sc)
	if err != nil {
		t.Error(msg, err)
	}

	err = sc.Close()
	sc = nil
	if err != nil {
		t.Error("sc.Close:", err)
	}

	err = sb.Close()
	sb = nil
	if err != nil {
		t.Error("sb.Close:", err)
	}
}

func TestPairTcp(t *testing.T) {

	var sb, sc *zmq.Socket

	defer func() {
		for _, s := range []*zmq.Socket{sb, sc} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	sb, err := zmq.NewSocket(zmq.PAIR)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	err = sb.Bind("tcp://127.0.0.1:9736")
	if err != nil {
		t.Fatal("sb.Bind:", err)
	}

	sc, err = zmq.NewSocket(zmq.PAIR)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	err = sc.Connect("tcp://127.0.0.1:9736")
	if err != nil {
		t.Fatal("sc.Connect:", err)
	}

	msg, err := bounce(sb, sc)

	if err != nil {
		t.Error(msg, err)
	}

	err = sc.Close()
	sc = nil
	if err != nil {
		t.Error("sc.Close:", err)
	}

	err = sb.Close()
	sb = nil
	if err != nil {
		t.Error("sb.Close:", err)
	}
}

func TestPoller(t *testing.T) {

	var sb, sc *zmq.Socket

	defer func() {
		for _, s := range []*zmq.Socket{sb, sc} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	sb, err := zmq.NewSocket(zmq.PAIR)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	err = sb.Bind("tcp://127.0.0.1:9737")
	if err != nil {
		t.Fatal("sb.Bind:", err)
	}

	sc, err = zmq.NewSocket(zmq.PAIR)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	err = sc.Connect("tcp://127.0.0.1:9737")
	if err != nil {
		t.Fatal("sc.Connect:", err)
	}

	poller := zmq.NewPoller()
	idxb := poller.Add(sb, 0)
	idxc := poller.Add(sc, 0)
	if idxb != 0 || idxc != 1 {
		t.Errorf("idxb=%d idxc=%d", idxb, idxc)
	}

	if pa, err := poller.PollAll(100 * time.Millisecond); err != nil {
		t.Error("PollAll 1:", err)
	} else if len(pa) != 2 {
		t.Errorf("PollAll 1 len = %d", len(pa))
	} else if pa[0].Events != 0 || pa[1].Events != 0 {
		t.Errorf("PollAll 1 events = %v, %v", pa[0], pa[1])
	}

	poller.Update(idxb, zmq.POLLOUT)
	poller.UpdateBySocket(sc, zmq.POLLIN)

	if pa, err := poller.PollAll(100 * time.Millisecond); err != nil {
		t.Error("PollAll 2:", err)
	} else if len(pa) != 2 {
		t.Errorf("PollAll 2 len = %d", len(pa))
	} else if pa[0].Events != zmq.POLLOUT || pa[1].Events != 0 {
		t.Errorf("PollAll 2 events = %v, %v", pa[0], pa[1])
	}

	poller.UpdateBySocket(sb, 0)

	content := "12345678ABCDEFGH12345678ABCDEFGH"

	//  Send message from client to server
	if rc, err := sb.Send(content, zmq.DONTWAIT); err != nil {
		t.Error("sb.Send DONTWAIT:", err)
	} else if rc != 32 {
		t.Error("sb.Send DONTWAIT:", err32)
	}

	if pa, err := poller.PollAll(100 * time.Millisecond); err != nil {
		t.Error("PollAll 3:", err)
	} else if len(pa) != 2 {
		t.Errorf("PollAll 3 len = %d", len(pa))
	} else if pa[0].Events != 0 || pa[1].Events != zmq.POLLIN {
		t.Errorf("PollAll 3 events = %v, %v", pa[0], pa[1])
	}

	//  Receive message
	if msg, err := sc.Recv(zmq.DONTWAIT); err != nil {
		t.Error("sb.Recv DONTWAIT:", err)
	} else if msg != content {
		t.Error("sb.Recv msg != content")
	}

	poller.UpdateBySocket(sb, zmq.POLLOUT)
	poller.Update(idxc, zmq.POLLIN)

	if pa, err := poller.PollAll(100 * time.Millisecond); err != nil {
		t.Error("PollAll 4:", err)
	} else if len(pa) != 2 {
		t.Errorf("PollAll 4 len = %d", len(pa))
	} else if pa[0].Events != zmq.POLLOUT || pa[1].Events != 0 {
		t.Errorf("PollAll 4 events = %v, %v", pa[0], pa[1])
	}

	err = sc.Close()
	sc = nil
	if err != nil {
		t.Error("sc.Close:", err)
	}

	err = sb.Close()
	sb = nil
	if err != nil {
		t.Error("sb.Close:", err)
	}
}

func TestSecurityCurve(t *testing.T) {

	time.Sleep(100 * time.Millisecond)

	var handler, server, client *zmq.Socket
	defer func() {
		for _, s := range []*zmq.Socket{handler} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	if _, minor, _ := zmq.Version(); minor >= 1 && !zmq.HasCurve() {
		t.Skip("Curve not available")
	}

	//  Generate new keypairs for this test
	client_public, client_secret, err := zmq.NewCurveKeypair()
	if err != nil {
		t.Fatal("NewCurveKeypair:", err)
	}
	server_public, server_secret, err := zmq.NewCurveKeypair()
	if err != nil {
		t.Fatal("NewCurveKeypair:", err)
	}

	handler, err = zmq.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = handler.Bind("inproc://zeromq.zap.01")
	if err != nil {
		t.Fatal("handler.Bind:", err)
	}

	doHandler := func(state zmq.State) error {
		msg, err := handler.RecvMessage(0)
		if err != nil {
			return err //  Terminating
		}
		version := msg[0]
		sequence := msg[1]
		// domain := msg[2]
		// address := msg[3]
		identity := msg[4]
		mechanism := msg[5]
		client_key := msg[6]
		client_key_text := zmq.Z85encode(client_key)

		if version != "1.0" {
			return errors.New("version != 1.0")
		}
		if mechanism != "CURVE" {
			return errors.New("mechanism != CURVE")
		}
		if identity != "IDENT" {
			return errors.New("identity != IDENT")
		}

		if client_key_text == client_public {
			handler.SendMessage(version, sequence, "200", "OK", "anonymous", "")
		} else {
			handler.SendMessage(version, sequence, "400", "Invalid client public key", "", "")
		}
		return nil
	}

	doQuit := func(i interface{}) error {
		err := handler.Close()
		handler = nil
		if err != nil {
			t.Error("handler.Close:", err)
		}
		return errors.New("Quit")
	}
	quit := make(chan interface{})

	reactor := zmq.NewReactor()
	reactor.AddSocket(handler, zmq.POLLIN, doHandler)
	reactor.AddChannel(quit, 0, doQuit)
	go func() {
		reactor.Run(100 * time.Millisecond)
		quit <- true
	}()
	defer func() {
		quit <- true
		<-quit
		close(quit)
	}()

	//  Server socket will accept connections
	server, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = server.SetCurveServer(1)
	if err != nil {
		t.Fatal("server.SetCurveServer(1):", err)
	}
	err = server.SetCurveSecretkey(server_secret)
	if err != nil {
		t.Fatal("server.SetCurveSecretkey:", err)
	}
	err = server.SetIdentity("IDENT")
	if err != nil {
		t.Fatal("server.SetIdentity:", err)
	}
	server.Bind("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("server.Bind:", err)
	}

	err = server.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("server.SetRcvtimeo:", err)
	}

	//  Check CURVE security with valid credentials
	client, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = client.SetCurveServerkey(server_public)
	if err != nil {
		t.Fatal("client.SetCurveServerkey:", err)
	}
	err = client.SetCurvePublickey(client_public)
	if err != nil {
		t.Fatal("client.SetCurvePublickey:", err)
	}
	err = client.SetCurveSecretkey(client_secret)
	if err != nil {
		t.Fatal("client.SetCurveSecretkey:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	msg, err := bounce(server, client)
	if err != nil {
		t.Error(msg, err)
	}
	err = client.Close()
	client = nil
	if err != nil {
		t.Fatal("client.Close:", err)
	}

	time.Sleep(100 * time.Millisecond)

	//  Check CURVE security with a garbage server key
	//  This will be caught by the curve_server class, not passed to ZAP
	garbage_key := "0000111122223333444455556666777788889999"
	client, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = client.SetCurveServerkey(garbage_key)
	if err != nil {
		t.Fatal("client.SetCurveServerkey:", err)
	}
	err = client.SetCurvePublickey(client_public)
	if err != nil {
		t.Fatal("client.SetCurvePublickey:", err)
	}
	err = client.SetCurveSecretkey(client_secret)
	if err != nil {
		t.Fatal("client.SetCurveSecretkey:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	err = client.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("client.SetRcvtimeo:", err)
	}
	_, err = bounce(server, client)
	if err == nil {
		t.Error("Expected failure, got success")
	}
	client.SetLinger(0)
	err = client.Close()
	client = nil
	if err != nil {
		t.Fatal("client.Close:", err)
	}

	time.Sleep(100 * time.Millisecond)

	//  Check CURVE security with a garbage client secret key
	//  This will be caught by the curve_server class, not passed to ZAP
	client, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = client.SetCurveServerkey(server_public)
	if err != nil {
		t.Fatal("client.SetCurveServerkey:", err)
	}
	err = client.SetCurvePublickey(garbage_key)
	if err != nil {
		t.Fatal("client.SetCurvePublickey:", err)
	}
	err = client.SetCurveSecretkey(client_secret)
	if err != nil {
		t.Fatal("client.SetCurveSecretkey:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	err = client.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("client.SetRcvtimeo:", err)
	}
	_, err = bounce(server, client)
	if err == nil {
		t.Error("Expected failure, got success")
	}
	client.SetLinger(0)
	err = client.Close()
	client = nil
	if err != nil {
		t.Fatal("client.Close:", err)
	}

	time.Sleep(100 * time.Millisecond)

	//  Check CURVE security with a garbage client secret key
	//  This will be caught by the curve_server class, not passed to ZAP
	client, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = client.SetCurveServerkey(server_public)
	if err != nil {
		t.Fatal("client.SetCurveServerkey:", err)
	}
	err = client.SetCurvePublickey(client_public)
	if err != nil {
		t.Fatal("client.SetCurvePublickey:", err)
	}
	err = client.SetCurveSecretkey(garbage_key)
	if err != nil {
		t.Fatal("client.SetCurveSecretkey:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	err = client.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("client.SetRcvtimeo:", err)
	}
	_, err = bounce(server, client)
	if err == nil {
		t.Error("Expected failure, got success")
	}
	client.SetLinger(0)
	err = client.Close()
	client = nil
	if err != nil {
		t.Fatal("client.Close:", err)
	}

	time.Sleep(100 * time.Millisecond)

	//  Check CURVE security with bogus client credentials
	//  This must be caught by the ZAP handler

	bogus_public, bogus_secret, _ := zmq.NewCurveKeypair()
	client, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = client.SetCurveServerkey(server_public)
	if err != nil {
		t.Fatal("client.SetCurveServerkey:", err)
	}
	err = client.SetCurvePublickey(bogus_public)
	if err != nil {
		t.Fatal("client.SetCurvePublickey:", err)
	}
	err = client.SetCurveSecretkey(bogus_secret)
	if err != nil {
		t.Fatal("client.SetCurveSecretkey:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	err = client.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("client.SetRcvtimeo:", err)
	}
	_, err = bounce(server, client)
	if err == nil {
		t.Error("Expected failure, got success")
	}
	client.SetLinger(0)
	err = client.Close()
	client = nil
	if err != nil {
		t.Fatal("client.Close:", err)
	}

	//  Shutdown
	err = server.Close()
	server = nil
	if err != nil {
		t.Error("server.Close:", err)
	}
}

func TestSecurityNull(t *testing.T) {

	time.Sleep(100 * time.Millisecond)

	var handler, server, client *zmq.Socket
	defer func() {
		for _, s := range []*zmq.Socket{handler} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	handler, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = handler.Bind("inproc://zeromq.zap.01")
	if err != nil {
		t.Fatal("handler.Bind:", err)
	}

	doHandler := func(state zmq.State) error {
		msg, err := handler.RecvMessage(0)
		if err != nil {
			return err //  Terminating
		}
		version := msg[0]
		sequence := msg[1]
		domain := msg[2]
		// address := msg[3]
		// identity := msg[4]
		mechanism := msg[5]

		if version != "1.0" {
			return errors.New("version != 1.0")
		}
		if mechanism != "NULL" {
			return errors.New("mechanism != NULL")
		}

		if domain == "TEST" {
			handler.SendMessage(version, sequence, "200", "OK", "anonymous", "")
		} else {
			handler.SendMessage(version, sequence, "400", "BAD DOMAIN", "", "")
		}
		return nil
	}

	doQuit := func(i interface{}) error {
		err := handler.Close()
		handler = nil
		if err != nil {
			t.Error("handler.Close:", err)
		}
		return errors.New("Quit")
	}
	quit := make(chan interface{})

	reactor := zmq.NewReactor()
	reactor.AddSocket(handler, zmq.POLLIN, doHandler)
	reactor.AddChannel(quit, 0, doQuit)
	go func() {
		reactor.Run(100 * time.Millisecond)
		quit <- true
	}()
	defer func() {
		quit <- true
		<-quit
		close(quit)
	}()

	//  We bounce between a binding server and a connecting client
	server, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	client, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}

	//  We first test client/server with no ZAP domain
	//  Libzmq does not call our ZAP handler, the connect must succeed
	err = server.Bind("tcp://127.0.0.1:9683")
	if err != nil {
		t.Fatal("server.Bind:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9683")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	msg, err := bounce(server, client)
	if err != nil {
		t.Error(msg, err)
	}
	server.Unbind("tcp://127.0.0.1:9683")
	client.Disconnect("tcp://127.0.0.1:9683")

	//  Now define a ZAP domain for the server; this enables
	//  authentication. We're using the wrong domain so this test
	//  must fail.
	err = server.SetZapDomain("WRONG")
	if err != nil {
		t.Fatal("server.SetZapDomain:", err)
	}
	err = server.Bind("tcp://127.0.0.1:9687")
	if err != nil {
		t.Fatal("server.Bind:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9687")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	err = client.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("client.SetRcvtimeo:", err)
	}
	err = server.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("server.SetRcvtimeo:", err)
	}
	_, err = bounce(server, client)
	if err == nil {
		t.Error("Expected failure, got success")
	}
	server.Unbind("tcp://127.0.0.1:9687")
	client.Disconnect("tcp://127.0.0.1:9687")

	//  Now use the right domain, the test must pass
	err = server.SetZapDomain("TEST")
	if err != nil {
		t.Fatal("server.SetZapDomain:", err)
	}
	err = server.Bind("tcp://127.0.0.1:9688")
	if err != nil {
		t.Fatal("server.Bind:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9688")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	msg, err = bounce(server, client)
	if err != nil {
		t.Error(msg, err)
	}
	server.Unbind("tcp://127.0.0.1:9688")
	client.Disconnect("tcp://127.0.0.1:9688")

	err = client.Close()
	client = nil
	if err != nil {
		t.Error("client.Close:", err)
	}
	err = server.Close()
	server = nil
	if err != nil {
		t.Error("server.Close:", err)
	}
}

func TestSecurityPlain(t *testing.T) {

	time.Sleep(100 * time.Millisecond)

	var handler, server, client *zmq.Socket
	defer func() {
		for _, s := range []*zmq.Socket{handler} {
			if s != nil {
				s.SetLinger(0)
				s.Close()
			}
		}
	}()

	handler, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = handler.Bind("inproc://zeromq.zap.01")
	if err != nil {
		t.Fatal("handler.Bind:", err)
	}

	doHandler := func(state zmq.State) error {
		msg, err := handler.RecvMessage(0)
		if err != nil {
			return err //  Terminating
		}
		version := msg[0]
		sequence := msg[1]
		// domain := msg[2]
		// address := msg[3]
		identity := msg[4]
		mechanism := msg[5]
		username := msg[6]
		password := msg[7]

		if version != "1.0" {
			return errors.New("version != 1.0")
		}
		if mechanism != "PLAIN" {
			return errors.New("mechanism != PLAIN")
		}
		if identity != "IDENT" {
			return errors.New("identity != IDENT")
		}

		if username == "admin" && password == "password" {
			handler.SendMessage(version, sequence, "200", "OK", "anonymous", "")
		} else {
			handler.SendMessage(version, sequence, "400", "Invalid username or password", "", "")
		}
		return nil
	}

	doQuit := func(i interface{}) error {
		err := handler.Close()
		if err != nil {
			t.Error("handler.Close:", err)
		}
		return errors.New("Quit")
	}
	quit := make(chan interface{})

	reactor := zmq.NewReactor()
	reactor.AddSocket(handler, zmq.POLLIN, doHandler)
	reactor.AddChannel(quit, 0, doQuit)
	go func() {
		reactor.Run(100 * time.Millisecond)
		quit <- true
	}()
	defer func() {
		quit <- true
		<-quit
		close(quit)
	}()

	//  Server socket will accept connections
	server, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket", err)
	}
	err = server.SetIdentity("IDENT")
	if err != nil {
		t.Fatal("server.SetIdentity:", err)
	}
	err = server.SetPlainServer(1)
	if err != nil {
		t.Fatal("server.SetPlainServer(1):", err)
	}
	err = server.Bind("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("server.Bind")
	}

	//  Check PLAIN security with correct username/password
	client, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	err = client.SetPlainUsername("admin")
	if err != nil {
		t.Fatal("client.SetPlainUsername:", err)
	}
	err = client.SetPlainPassword("password")
	if err != nil {
		t.Fatal("client.SetPlainPassword:", err)
	}
	err = client.Connect("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	msg, err := bounce(server, client)
	if err != nil {
		t.Error(msg, err)
	}
	err = client.Close()
	client = nil
	if err != nil {
		t.Fatal("client.Close:", err)
	}

	//  Check PLAIN security with badly configured client (as_server)
	//  This will be caught by the plain_server class, not passed to ZAP
	client, err = zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	client.SetPlainServer(1)
	if err != nil {
		t.Fatal("client.SetPlainServer(1):", err)
	}
	err = client.Connect("tcp://127.0.0.1:9998")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}
	err = client.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("client.SetRcvtimeo:", err)
	}
	err = server.SetRcvtimeo(time.Second)
	if err != nil {
		t.Fatal("server.SetRcvtimeo:", err)
	}
	_, err = bounce(server, client)
	if err == nil {
		t.Error("Expected failure, got success")
	}
	client.SetLinger(0)
	err = client.Close()
	client = nil
	if err != nil {
		t.Fatal("client.Close:", err)
	}

	err = server.Close()
	server = nil
	if err != nil {
		t.Fatal("server.Close:", err)
	}
}

func bounce(server, client *zmq.Socket) (msg string, err error) {

	content := "12345678ABCDEFGH12345678abcdefgh"

	//  Send message from client to server
	rc, err := client.Send(content, zmq.SNDMORE|zmq.DONTWAIT)
	if err != nil {
		return "client.Send SNDMORE|DONTWAIT:", err
	}
	if rc != 32 {
		return "client.Send SNDMORE|DONTWAIT:", err32
	}

	rc, err = client.Send(content, zmq.DONTWAIT)
	if err != nil {
		return "client.Send DONTWAIT:", err
	}
	if rc != 32 {
		return "client.Send DONTWAIT:", err32
	}

	//  Receive message at server side
	msg, err = server.Recv(0)
	if err != nil {
		return "server.Recv 1:", err
	}

	//  Check that message is still the same
	if msg != content {
		return "server.Recv 1:", errors.New(fmt.Sprintf("%q != %q", msg, content))
	}

	rcvmore, err := server.GetRcvmore()
	if err != nil {
		return "server.GetRcvmore 1:", err
	}
	if !rcvmore {
		return "server.GetRcvmore 1:", errors.New(fmt.Sprint("rcvmore ==", rcvmore))
	}

	//  Receive message at server side
	msg, err = server.Recv(0)
	if err != nil {
		return "server.Recv 2:", err
	}

	//  Check that message is still the same
	if msg != content {
		return "server.Recv 2:", errors.New(fmt.Sprintf("%q != %q", msg, content))
	}

	rcvmore, err = server.GetRcvmore()
	if err != nil {
		return "server.GetRcvmore 2:", err
	}
	if rcvmore {
		return "server.GetRcvmore 2:", errors.New(fmt.Sprint("rcvmore == ", rcvmore))
	}

	// The same, from server back to client

	//  Send message from server to client
	rc, err = server.Send(content, zmq.SNDMORE)
	if err != nil {
		return "server.Send SNDMORE:", err
	}
	if rc != 32 {
		return "server.Send SNDMORE:", err32
	}

	rc, err = server.Send(content, 0)
	if err != nil {
		return "server.Send 0:", err
	}
	if rc != 32 {
		return "server.Send 0:", err32
	}

	//  Receive message at client side
	msg, err = client.Recv(0)
	if err != nil {
		return "client.Recv 1:", err
	}

	//  Check that message is still the same
	if msg != content {
		return "client.Recv 1:", errors.New(fmt.Sprintf("%q != %q", msg, content))
	}

	rcvmore, err = client.GetRcvmore()
	if err != nil {
		return "client.GetRcvmore 1:", err
	}
	if !rcvmore {
		return "client.GetRcvmore 1:", errors.New(fmt.Sprint("rcvmore ==", rcvmore))
	}

	//  Receive message at client side
	msg, err = client.Recv(0)
	if err != nil {
		return "client.Recv 2:", err
	}

	//  Check that message is still the same
	if msg != content {
		return "client.Recv 2:", errors.New(fmt.Sprintf("%q != %q", msg, content))
	}

	rcvmore, err = client.GetRcvmore()
	if err != nil {
		return "client.GetRcvmore 2:", err
	}
	if rcvmore {
		return "client.GetRcvmore 2:", errors.New(fmt.Sprint("rcvmore == ", rcvmore))
	}
	return "OK", nil
}

func arrayEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
