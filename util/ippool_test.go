package util

import (
	"net"
	"testing"
)

type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }

// capability_id: rainbond.util.network.interface-address-filter
func TestCheckIPAddress(t *testing.T) {
	loopback := &net.IPNet{IP: net.ParseIP("127.0.0.1")}
	if got := checkIPAddress(loopback); got != nil {
		t.Fatalf("expected loopback to be filtered, got %+v", got)
	}

	normal := &net.IPNet{IP: net.ParseIP("192.168.10.20")}
	got := checkIPAddress(normal)
	if got == nil || !got.IP.Equal(normal.IP) {
		t.Fatalf("expected normal ip to pass through, got %+v", got)
	}

	if got := checkIPAddress(fakeAddr{}); got != nil {
		t.Fatalf("expected non-IP addr to be filtered, got %+v", got)
	}
}
