package clientv3

import (
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	minHealthRetryDuration = 3 * time.Second
	unknownService         = "unknown service grpc.health.v1.Health"
)

var ErrNoAddrAvilable = status.Error(codes.Unavailable, "there is no address available")

type healthCheckFunc func(ep string) (bool, error)

type notifyMsg int

const (
	notifyReset notifyMsg = iota
	notifyNext
)

// healthBalancer is a minimal compatibility shim for old etcd clientv3 code.
// We only need endpoint rotation and readiness signaling for Rainbond's usage.
type healthBalancer struct {
	eps   []string
	addrs []string

	mu      sync.RWMutex
	pinAddr string

	readyc chan struct{}
	upc    chan struct{}
	stopc  chan struct{}

	updateAddrsC chan notifyMsg

	closed bool
}

func newHealthBalancer(eps []string, _ time.Duration, _ healthCheckFunc) *healthBalancer {
	hb := &healthBalancer{
		eps:          append([]string(nil), eps...),
		addrs:        append([]string(nil), eps...),
		readyc:       make(chan struct{}),
		upc:          make(chan struct{}),
		stopc:        make(chan struct{}),
		updateAddrsC: make(chan notifyMsg, 1),
	}
	if len(eps) > 0 {
		hb.pinAddr = eps[0]
		close(hb.readyc)
		close(hb.upc)
	}
	return hb
}

func (b *healthBalancer) ready() <-chan struct{} { return b.readyc }

func (b *healthBalancer) ConnectNotify() <-chan struct{} {
	return b.upc
}

func (b *healthBalancer) endpoint(host string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if host == "" {
		return b.pinAddr
	}
	for _, ep := range b.eps {
		if ep == host {
			return ep
		}
	}
	return host
}

func (b *healthBalancer) pinned() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.pinAddr
}

func (b *healthBalancer) hostPortError(_ string, _ error) {}

func (b *healthBalancer) next() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.eps) == 0 {
		b.pinAddr = ""
		return
	}
	current := 0
	for i, ep := range b.eps {
		if ep == b.pinAddr {
			current = i
			break
		}
	}
	b.pinAddr = b.eps[(current+1)%len(b.eps)]
	select {
	case b.updateAddrsC <- notifyNext:
	default:
	}
}

func (b *healthBalancer) updateAddrs(eps ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.eps = append([]string(nil), eps...)
	b.addrs = append([]string(nil), eps...)
	if len(b.eps) == 0 {
		b.pinAddr = ""
		return
	}
	if b.pinAddr == "" {
		b.pinAddr = b.eps[0]
	}
	select {
	case b.updateAddrsC <- notifyReset:
	default:
	}
}

func (b *healthBalancer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	close(b.stopc)
	b.closed = true
	return nil
}

func hasAddr(addrs []string, target string) bool {
	for _, addr := range addrs {
		if addr == target {
			return true
		}
	}
	return false
}
