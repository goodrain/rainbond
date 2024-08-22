// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

// 文件: event_log.proto
// 说明: 该文件定义了事件日志服务的 Protocol Buffers (protobuf) 接口和消息格式。文件中包含
// 了服务端方法和消息体结构，用于在系统中处理事件日志的序列化和传输。通过这些定义，Rainbond
// 平台能够确保事件日志数据在不同服务之间的高效传递和处理。

package server

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/goodrain/rainbond/eventlog/store"

	"github.com/goodrain/rainbond/eventlog/entry/grpc/pb"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type EventLogRPCServer struct {
	conf         conf.EventLogServerConf
	log          *logrus.Entry
	cancel       func()
	context      context.Context
	storemanager store.Manager
	messageChan  chan []byte
	listenErr    chan error
	lis          net.Listener
}

// NewServer server
func NewServer(conf conf.EventLogServerConf, log *logrus.Entry, storeManager store.Manager, listenErr chan error) *EventLogRPCServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventLogRPCServer{
		conf:         conf,
		log:          log,
		storemanager: storeManager,
		context:      ctx,
		cancel:       cancel,
		messageChan:  storeManager.ReceiveMessageChan(),
		listenErr:    listenErr,
	}
}

// Start start grpc server
func (s *EventLogRPCServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.conf.BindIP, s.conf.BindPort))
	if err != nil {
		logrus.Errorf("failed to listen: %v", err)
		return err
	}
	s.lis = lis
	server := grpc.NewServer()
	pb.RegisterEventLogServer(server, s)
	// Register reflection service on gRPC server.
	reflection.Register(server)
	s.log.Infof("event message grpc server listen %s:%d", s.conf.BindIP, s.conf.BindPort)
	if err := server.Serve(lis); err != nil {
		s.log.Error("event log api grpc listen error.", err.Error())
		s.listenErr <- err
	}
	return nil
}

// Stop stop
func (s *EventLogRPCServer) Stop() {
	s.cancel()
	// if s.lis != nil {
	// 	s.lis.Close()
	// }
}

// Log impl EventLogServerServer
func (s *EventLogRPCServer) Log(stream pb.EventLog_LogServer) error {
	for {
		select {
		case <-s.context.Done():
			if err := stream.SendAndClose(&pb.Reply{Status: "success", Message: "server closed"}); err != nil {
				return err
			}
			return nil
		default:
		}
		log, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				s.log.Error("receive log error:", err.Error())
				if err := stream.SendAndClose(&pb.Reply{Status: "success"}); err != nil {
					return err
				}
				return nil
			}
			return err
		}
		select {
		case s.messageChan <- log.Log:
		default:
		}
	}
}
