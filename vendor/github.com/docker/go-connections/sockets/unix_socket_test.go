package sockets

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func runTest(t *testing.T, path string, l net.Listener, echoStr string) {
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte(echoStr))
			conn.Close()
		}
	}()

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 5)
	if _, err := conn.Read(buf); err != nil {
		t.Fatal(err)
	} else if string(buf) != echoStr {
		t.Fatal(fmt.Errorf("Msg may lost"))
	}
}

// TestNewUnixSocket run under root user.
func TestNewUnixSocket(t *testing.T) {
	gid := os.Getgid()
	path := "/tmp/test.sock"
	echoStr := "hello"
	l, err := NewUnixSocket(path, gid)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	runTest(t, path, l, echoStr)
}

func TestUnixSocketWithOpts(t *testing.T) {
	uid, gid := os.Getuid(), os.Getgid()
	path := "/tmp/test.sock"
	echoStr := "hello"
	l, err := NewUnixSocketWithOpts(path, WithChown(uid, gid), WithChmod(0660))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	runTest(t, path, l, echoStr)
}
