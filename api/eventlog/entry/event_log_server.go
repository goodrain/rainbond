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

package entry

import (
	"errors"
	"github.com/goodrain/rainbond/api/eventlog/conf"
	grpcserver "github.com/goodrain/rainbond/api/eventlog/entry/grpc/server"
	"github.com/goodrain/rainbond/api/eventlog/store"

	"golang.org/x/net/context"

	"sync"

	"github.com/sirupsen/logrus"
)

// EventLogServer 日志接受服务
type EventLogServer struct {
	conf               conf.EventLogServerConf
	log                *logrus.Entry
	cancel             func()
	context            context.Context
	storemanager       store.Manager
	messageChan        chan []byte
	listenErr          chan error
	serverLock         sync.Mutex
	stopReceiveMessage bool
	eventRPCServer     *grpcserver.EventLogRPCServer
}

// NewEventLogServer 创建zmq server服务端
func NewEventLogServer(conf conf.EventLogServerConf, log *logrus.Entry, storeManager store.Manager) (*EventLogServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &EventLogServer{
		conf:         conf,
		log:          log,
		cancel:       cancel,
		context:      ctx,
		storemanager: storeManager,
		listenErr:    make(chan error),
	}

	//grpc服务
	eventRPCServer := grpcserver.NewServer(conf, log, storeManager, s.listenErr)
	s.messageChan = s.storemanager.ReceiveMessageChan()
	if s.messageChan == nil {
		return nil, errors.New("receive log message server can not get store message chan ")
	}
	s.eventRPCServer = eventRPCServer
	return s, nil
}

// Serve 执行
func (s *EventLogServer) Serve() {
	s.eventRPCServer.Start()
}

// Stop 停止
func (s *EventLogServer) Stop() {
	s.cancel()
	s.eventRPCServer.Stop()
	s.log.Info("receive event message server stop")
}

// ListenError listen error chan
func (s *EventLogServer) ListenError() chan error {
	return s.listenErr
}
