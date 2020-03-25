// Copyright 2018 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// Package main contains the test driver for testing xDS manually.
package main

import (
	"context"
	cryptotls "crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	"github.com/envoyproxy/go-control-plane/pkg/test"

	"github.com/envoyproxy/go-control-plane/pkg/test/resource"
)

var (
	debug bool

	port         uint
	gatewayPort  uint
	upstreamPort uint
	basePort     uint
	alsPort      uint

	delay    time.Duration
	requests int
	updates  int

	mode          string
	clusters      int
	httpListeners int
	tcpListeners  int
	runtimes      int
	tls           bool

	nodeID string
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Use debug logging")
	flag.UintVar(&port, "port", 18000, "Management server port")
	flag.UintVar(&gatewayPort, "gateway", 18001, "Management server port for HTTP gateway")
	flag.UintVar(&upstreamPort, "upstream", 18080, "Upstream HTTP/1.1 port")
	flag.UintVar(&basePort, "base", 9000, "Listener port")
	flag.UintVar(&alsPort, "als", 18090, "Accesslog server port")
	flag.DurationVar(&delay, "delay", 500*time.Millisecond, "Interval between request batch retries")
	flag.IntVar(&requests, "r", 5, "Number of requests between snapshot updates")
	flag.IntVar(&updates, "u", 3, "Number of snapshot updates")
	flag.StringVar(&mode, "xds", resource.Ads, "Management server type (ads, xds, rest)")
	flag.IntVar(&clusters, "clusters", 4, "Number of clusters")
	flag.IntVar(&httpListeners, "http", 2, "Number of HTTP listeners (and RDS configs)")
	flag.IntVar(&tcpListeners, "tcp", 2, "Number of TCP pass-through listeners")
	flag.IntVar(&runtimes, "runtimes", 1, "Number of RTDS layers")
	flag.StringVar(&nodeID, "nodeID", "test-id", "Node ID")
	flag.BoolVar(&tls, "tls", false, "Enable TLS on all listeners and use SDS for secret delivery")
}

// main returns code 1 if any of the batches failed to pass all requests
func main() {
	flag.Parse()
	ctx := context.Background()

	// start upstream
	go test.RunHTTP(ctx, upstreamPort)

	// create a cache
	signal := make(chan struct{})
	cb := &callbacks{signal: signal}
	config := cache.NewSnapshotCache(mode == resource.Ads, cache.IDHash{}, logger{})
	srv := server.NewServer(context.Background(), config, cb)
	als := &test.AccessLogService{}

	// create a test snapshot
	snapshots := resource.TestSnapshot{
		Xds:              mode,
		UpstreamPort:     uint32(upstreamPort),
		BasePort:         uint32(basePort),
		NumClusters:      clusters,
		NumHTTPListeners: httpListeners,
		NumTCPListeners:  tcpListeners,
		TLS:              tls,
		NumRuntimes:      runtimes,
	}

	// start the xDS server
	go test.RunAccessLogServer(ctx, als, alsPort)
	go test.RunManagementServer(ctx, srv, port)
	go test.RunManagementGateway(ctx, srv, gatewayPort, logger{})

	log.Println("waiting for the first request...")
	select {
	case <-signal:
		break
	case <-time.After(1 * time.Minute):
		log.Println("timeout waiting for the first request")
		os.Exit(1)
	}
	log.Printf("initial snapshot %+v\n", snapshots)
	log.Printf("executing sequence updates=%d request=%d\n", updates, requests)

	for i := 0; i < updates; i++ {
		snapshots.Version = fmt.Sprintf("v%d", i)
		log.Printf("update snapshot %v\n", snapshots.Version)

		snapshot := snapshots.Generate()
		if err := snapshot.Consistent(); err != nil {
			log.Printf("snapshot inconsistency: %+v\n", snapshot)
		}

		err := config.SetSnapshot(nodeID, snapshot)
		if err != nil {
			log.Printf("snapshot error %q for %+v\n", err, snapshot)
			os.Exit(1)
		}

		// pass is true if all requests succeed at least once in a run
		pass := false
		for j := 0; j < requests; j++ {
			ok, failed := callEcho()
			if failed == 0 && !pass {
				pass = true
			}
			log.Printf("request batch %d, ok %v, failed %v, pass %v\n", j, ok, failed, pass)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		als.Dump(func(s string) {
			if debug {
				log.Println(s)
			}
		})
		cb.Report()

		if !pass {
			log.Printf("failed all requests in a run %d\n", i)
			os.Exit(1)
		}
	}

	log.Printf("Test for %s passed!\n", mode)
}

// callEcho calls upstream echo service on all listener ports and returns an error
// if any of the listeners returned an error.
func callEcho() (int, int) {
	total := httpListeners + tcpListeners
	ok, failed := 0, 0
	ch := make(chan error, total)

	// spawn requests
	for i := 0; i < total; i++ {
		go func(i int) {
			client := http.Client{
				Timeout: 100 * time.Millisecond,
				Transport: &http.Transport{
					TLSClientConfig: &cryptotls.Config{InsecureSkipVerify: true},
				},
			}
			proto := "http"
			if tls {
				proto = "https"
			}
			req, err := client.Get(fmt.Sprintf("%s://127.0.0.1:%d", proto, basePort+uint(i)))
			if err != nil {
				ch <- err
				return
			}
			defer req.Body.Close()
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				ch <- err
				return
			}
			if string(body) != test.Hello {
				ch <- fmt.Errorf("unexpected return %q", string(body))
				return
			}
			ch <- nil
		}(i)
	}

	for {
		out := <-ch
		if out == nil {
			ok++
		} else {
			failed++
		}
		if ok+failed == total {
			return ok, failed
		}
	}
}

type logger struct{}

func (logger logger) Debugf(format string, args ...interface{}) {
	if debug {
		log.Printf(format+"\n", args...)
	}
}

func (logger logger) Infof(format string, args ...interface{}) {
	if debug {
		log.Printf(format+"\n", args...)
	}
}

func (logger logger) Warnf(format string, args ...interface{}) {
	log.Printf(format+"\n", args...)
}

func (logger logger) Errorf(format string, args ...interface{}) {
	log.Printf(format+"\n", args...)
}

type callbacks struct {
	signal   chan struct{}
	fetches  int
	requests int
	mu       sync.Mutex
}

func (cb *callbacks) Report() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	log.Printf("server callbacks fetches=%d requests=%d\n", cb.fetches, cb.requests)
}
func (cb *callbacks) OnStreamOpen(_ context.Context, id int64, typ string) error {
	if debug {
		log.Printf("stream %d open for %s\n", id, typ)
	}
	return nil
}
func (cb *callbacks) OnStreamClosed(id int64) {
	if debug {
		log.Printf("stream %d closed\n", id)
	}
}
func (cb *callbacks) OnStreamRequest(int64, *v2.DiscoveryRequest) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.requests++
	if cb.signal != nil {
		close(cb.signal)
		cb.signal = nil
	}
	return nil
}
func (cb *callbacks) OnStreamResponse(int64, *v2.DiscoveryRequest, *v2.DiscoveryResponse) {}
func (cb *callbacks) OnFetchRequest(_ context.Context, req *v2.DiscoveryRequest) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.fetches++
	if cb.signal != nil {
		close(cb.signal)
		cb.signal = nil
	}
	return nil
}
func (cb *callbacks) OnFetchResponse(*v2.DiscoveryRequest, *v2.DiscoveryResponse) {}
