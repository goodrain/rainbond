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

package server

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/api/eventlog/conf"
	"github.com/goodrain/rainbond/api/eventlog/entry/grpc/pb"
	"github.com/goodrain/rainbond/api/eventlog/store"
	"io"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	// 批处理相关常量
	batchSize      = 100                    // 批处理大小
	batchTimeout   = 100 * time.Millisecond // 批处理超时
	maxMessageSize = 1024 * 1024            // 最大消息大小 1MB
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

	// 优化相关字段
	messagePool *LogMessagePool // 对象池
	batchBuffer *BatchBuffer    // 批处理缓冲区
	batchTicker *time.Ticker    // 批处理定时器
}

// NewServer server
func NewServer(conf conf.EventLogServerConf, log *logrus.Entry, storeManager store.Manager, listenErr chan error) *EventLogRPCServer {
	ctx, cancel := context.WithCancel(context.Background())
	server := &EventLogRPCServer{
		conf:         conf,
		log:          log,
		storemanager: storeManager,
		context:      ctx,
		cancel:       cancel,
		messageChan:  storeManager.ReceiveMessageChan(),
		listenErr:    listenErr,

		// 初始化优化组件
		messagePool: NewLogMessagePool(),
		batchBuffer: NewBatchBuffer(batchSize),
		batchTicker: time.NewTicker(batchTimeout),
	}

	// 启动批处理goroutine
	go server.processBatch()

	return server
}

// Start start grpc server
func (s *EventLogRPCServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.conf.BindIP, s.conf.BindPort))
	if err != nil {
		logrus.Errorf("failed to listen: %v", err)
		return err
	}
	s.lis = lis

	// 配置gRPC服务器选项
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
	}

	server := grpc.NewServer(opts...)
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
	if s.batchTicker != nil {
		s.batchTicker.Stop()
	}
	// 处理剩余的批处理消息
	s.flushBatch()
}

// Log impl EventLogServerServer - 优化版本
func (s *EventLogRPCServer) Log(stream pb.EventLog_LogServer) error {
	defer func() {
		if r := recover(); r != nil {
			s.log.Errorf("Log handler recovered from panic: %v", r)
		}
	}()

	for {
		select {
		case <-s.context.Done():
			if err := stream.SendAndClose(&pb.Reply{Status: "success", Message: "server closed"}); err != nil {
				return err
			}
			return nil
		default:
		}

		// 使用对象池获取LogMessage
		msg := s.messagePool.Get()

		// 接收消息并重用LogMessage对象
		if err := stream.RecvMsg(msg); err != nil {
			s.messagePool.Put(msg) // 归还对象到池
			if err == io.EOF {
				if err := stream.SendAndClose(&pb.Reply{Status: "success"}); err != nil {
					return err
				}
				return nil
			}
			s.log.Error("receive log error:", err.Error())
			return err
		}

		// 验证消息大小
		if len(msg.Log) > maxMessageSize {
			s.messagePool.Put(msg)
			s.log.Warnf("Message too large: %d bytes, dropping", len(msg.Log))
			continue
		}

		// 复制数据到新的字节切片（避免引用原始数据）
		logData := make([]byte, len(msg.Log))
		copy(logData, msg.Log)

		// 归还对象到池
		s.messagePool.Put(msg)

		// 尝试非阻塞发送到消息通道
		select {
		case s.messageChan <- logData:
			// 发送成功
		default:
			// 通道满了，记录警告但不阻塞
			s.log.Warn("Message channel is full, dropping message")
		}
	}
}

// processBatch 处理批量消息
func (s *EventLogRPCServer) processBatch() {
	defer func() {
		if r := recover(); r != nil {
			s.log.Errorf("Batch processor recovered from panic: %v", r)
		}
	}()

	for {
		select {
		case <-s.context.Done():
			s.flushBatch()
			return
		case <-s.batchTicker.C:
			s.flushBatch()
		}
	}
}

// flushBatch 刷新批处理缓冲区
func (s *EventLogRPCServer) flushBatch() {
	messages := s.batchBuffer.Flush()
	if len(messages) == 0 {
		return
	}

	// 批量处理消息
	s.log.Debugf("Processing batch of %d messages", len(messages))

	for _, msg := range messages {
		if msg != nil && len(msg.Log) > 0 {
			// 非阻塞发送
			select {
			case s.messageChan <- msg.Log:
			default:
				s.log.Warn("Message channel full during batch processing")
			}
			// 归还对象到池
			s.messagePool.Put(msg)
		}
	}
}
