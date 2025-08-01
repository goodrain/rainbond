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
	"github.com/goodrain/rainbond/api/eventlog/store"
	"net"
	"time"

	"golang.org/x/net/context"

	"sync"

	"github.com/sirupsen/logrus"
)

// BufferPool UDP缓冲区池
type BufferPool struct {
	pool sync.Pool
}

func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

func (p *BufferPool) Put(buf []byte) {
	p.pool.Put(buf)
}

// NMonitorMessageServer 新监控消息服务
type NMonitorMessageServer struct {
	conf         conf.NewMonitorMessageServerConf
	log          *logrus.Entry
	cancel       func()
	context      context.Context
	storemanager store.Manager
	messageChan  chan []byte
	listenErr    chan error
	listener     *net.UDPConn

	// 优化字段
	bufferPool *BufferPool
}

// NewNMonitorMessageServer 创建UDP服务端
func NewNMonitorMessageServer(conf conf.NewMonitorMessageServerConf, log *logrus.Entry, storeManager store.Manager) (*NMonitorMessageServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &NMonitorMessageServer{
		conf:         conf,
		log:          log,
		cancel:       cancel,
		context:      ctx,
		storemanager: storeManager,
		listenErr:    make(chan error),
		bufferPool:   NewBufferPool(65535), // 使用缓冲区池
	}

	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(conf.ListenerHost), Port: conf.ListenerPort})
	if err != nil {
		return nil, err
	}
	log.Infof("UDP Server Listener: %s", listener.LocalAddr().String())
	s.listener = listener
	s.messageChan = s.storemanager.NewMonitorMessageChan()
	if s.messageChan == nil {
		return nil, errors.New("receive monitor message server can not get store message chan")
	}
	return s, nil
}

// Serve 执行
func (s *NMonitorMessageServer) Serve() {
	s.handleMessage()
}

// Stop 停止
func (s *NMonitorMessageServer) Stop() {
	s.cancel()
	s.log.Info("receive new monitor message server stop")
}

func (s *NMonitorMessageServer) handleMessage() {
	defer s.listener.Close()
	s.log.Infoln("start receive monitor message by udp")

	for {
		select {
		case <-s.context.Done():
			return
		default:
		}

		// 从缓冲区池获取buffer
		buf := s.bufferPool.Get()

		// 设置读取超时
		s.listener.SetReadDeadline(time.Now().Add(time.Second * 5))

		n, _, err := s.listener.ReadFromUDP(buf)
		if err != nil {
			s.bufferPool.Put(buf) // 归还buffer
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // 超时继续
			}
			logrus.Errorf("read new monitor message from udp error,%s", err.Error())
			time.Sleep(time.Second * 2)
			continue
		}

		if n <= 0 {
			s.bufferPool.Put(buf)
			continue
		}

		// 创建精确大小的消息副本
		message := make([]byte, n)
		copy(message, buf[:n])

		// 归还buffer到池
		s.bufferPool.Put(buf)

		// 非阻塞发送消息
		select {
		case s.messageChan <- message:
		default:
			s.log.Warn("Monitor message channel is full, dropping message")
		}
	}
}

// ListenError listen error chan
func (s *NMonitorMessageServer) ListenError() chan error {
	return s.listenErr
}
