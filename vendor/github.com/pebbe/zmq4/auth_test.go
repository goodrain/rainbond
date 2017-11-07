package zmq4_test

import (
	zmq "github.com/pebbe/zmq4"

	"testing"
)

func TestAuthCurvePublic(t *testing.T) {
	if _, minor, _ := zmq.Version(); minor < 2 {
		t.Skip("CurvePublic not available in ZeroMQ versions prior to 4.2.0")
	}
	expected := "Yne@$w-vo<fVvi]a<NY6T1ed:M$fCG*[IaLV{hID"
	public, err := zmq.AuthCurvePublic("D:)Q[IlAW!ahhC2ac:9*A}h:p?([4%wOTJ%JR%cs")
	if err != nil {
		t.Fatal(err)
	}
	if public != expected {
		t.Fatalf("Expected: %s, got: %s", expected, public)
	}
	public, err = zmq.AuthCurvePublic("blabla")
	if err == nil {
		t.Fatal("Error expected")
	}
}

func TestAuthStart(t *testing.T) {

	if _, minor, _ := zmq.Version(); minor >= 1 && !zmq.HasCurve() {
		t.Skip("Curve not available")
	}

	type Meta struct {
		key   string
		value string
		ok    bool
	}

	zmq.AuthSetVerbose(false)

	//  Start authentication engine
	err := zmq.AuthStart()
	if err != nil {
		t.Fatal("AuthStart:", err)
	}
	defer zmq.AuthStop()

	zmq.AuthSetMetadataHandler(
		func(version, request_id, domain, address, identity, mechanism string, credentials ...string) (metadata map[string]string) {
			return map[string]string{
				"Identity": identity,
				"User-Id":  "anonymous",
				"Hello":    "World!",
				"Foo":      "Bar",
			}
		})

	zmq.AuthAllow("domain1", "127.0.0.1")

	//  We need two certificates, one for the client and one for
	//  the server. The client must know the server's public key
	//  to make a CURVE connection.
	client_public, client_secret, err := zmq.NewCurveKeypair()
	if err != nil {
		t.Fatal("NewCurveKeypair:", err)
	}
	server_public, server_secret, err := zmq.NewCurveKeypair()
	if err != nil {
		t.Fatal("NewCurveKeypair:", err)
	}

	//  Tell authenticator to use this public client key
	zmq.AuthCurveAdd("domain1", client_public)

	//  Create and bind server socket
	server, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	defer func() {
		server.SetLinger(0)
		server.Close()
	}()
	server.SetIdentity("Server1")
	server.ServerAuthCurve("domain1", server_secret)
	err = server.Bind("tcp://*:9000")
	if err != nil {
		t.Fatal("server.Bind:", err)
	}

	//  Create and connect client socket
	client, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		t.Fatal("NewSocket:", err)
	}
	defer func() {
		client.SetLinger(0)
		client.Close()
	}()
	server.SetIdentity("Client1")
	client.ClientAuthCurve(server_public, client_public, client_secret)
	err = client.Connect("tcp://127.0.0.1:9000")
	if err != nil {
		t.Fatal("client.Connect:", err)
	}

	//  Send a message from client to server
	msg := []string{"Greetings", "Earthlings!"}
	_, err = client.SendMessage(msg[0], msg[1])
	if err != nil {
		t.Fatal("client.SendMessage:", err)
	}

	// Receive message and metadata on the server
	tests := []Meta{
		{"Identity", "Server1", true},
		{"User-Id", "anonymous", true},
		{"Socket-Type", "DEALER", true},
		{"Hello", "World!", true},
		{"Foo", "Bar", true},
		{"Fuz", "", false},
	}
	keys := make([]string, len(tests))
	for i, test := range tests {
		keys[i] = test.key
	}
	message, metadata, err := server.RecvMessageWithMetadata(0, keys...)
	if err != nil {
		t.Fatal("server.RecvMessageWithMetadata:", err)
	}
	if !arrayEqual(message, msg) {
		t.Errorf("Received message was %q, expected %q", message, msg)
	}
	if _, minor, _ := zmq.Version(); minor < 1 {
		t.Log("Metadata not avalable in ZeroMQ versions prior to 4.1.0")
	} else {
		for _, test := range tests {
			value, ok := metadata[test.key]
			if value != test.value || ok != test.ok {
				t.Errorf("Metadata %s, expected %q %v, got %q %v", test.key, test.value, test.ok, value, ok)
			}
		}
	}
}
