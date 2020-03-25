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

// Package test contains test utilities
package test

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	"google.golang.org/grpc"

	v2grpc "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	accessloggrpc "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v2"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	logger "github.com/envoyproxy/go-control-plane/pkg/log"
	xds "github.com/envoyproxy/go-control-plane/pkg/server"
)

const (
	// Hello is the echo message
	Hello = "Hi, there!\n"
)

type echo struct{}

func (h echo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/text")
	if _, err := w.Write([]byte(Hello)); err != nil {
		log.Println(err)
	}
}

// RunHTTP opens a simple listener on the port.
func RunHTTP(ctx context.Context, upstreamPort uint) {
	log.Printf("upstream listening HTTP/1.1 on %d\n", upstreamPort)
	server := &http.Server{Addr: fmt.Sprintf(":%d", upstreamPort), Handler: echo{}}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()
}

// RunAccessLogServer starts an accessloggrpc service.
func RunAccessLogServer(ctx context.Context, als *AccessLogService, port uint) {
	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	accessloggrpc.RegisterAccessLogServiceServer(grpcServer, als)
	log.Printf("access log server listening on %d\n", port)

	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Println(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
}

const grpcMaxConcurrentStreams = 1000000

// RunManagementServer starts an xDS server at the given port.
func RunManagementServer(ctx context.Context, server xds.Server, port uint) {
	// gRPC golang library sets a very small upper bound for the number gRPC/h2
	// streams over a single TCP connection. If a proxy multiplexes requests over
	// a single connection to the management server, then it might lead to
	// availability problems.
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	// register services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	v2grpc.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	v2grpc.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	v2grpc.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	v2grpc.RegisterListenerDiscoveryServiceServer(grpcServer, server)
	discoverygrpc.RegisterSecretDiscoveryServiceServer(grpcServer, server)
	discoverygrpc.RegisterRuntimeDiscoveryServiceServer(grpcServer, server)

	log.Printf("management server listening on %d\n", port)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Println(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
}

// RunManagementGateway starts an HTTP gateway to an xDS server.
func RunManagementGateway(ctx context.Context, srv xds.Server, port uint, lg logger.Logger) {
	log.Printf("gateway listening HTTP/1.1 on %d\n", port)
	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: &xds.HTTPGateway{Server: srv, Log: lg}}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()
}
