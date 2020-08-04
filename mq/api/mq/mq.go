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

package mq

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/cmd/mq/option"
	"github.com/goodrain/rainbond/mq/client"

	"golang.org/x/net/context"

	etcdutil "github.com/goodrain/rainbond/util/etcd"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
)

//ActionMQ 队列操作
type ActionMQ interface {
	Enqueue(context.Context, string, string) error
	Dequeue(context.Context, string) (string, error)
	TopicIsExist(string) bool
	GetAllTopics() []string
	Start() error
	Stop() error
	MessageQueueSize(topic string) int64
}

// EnqueueNumber enqueue number
var EnqueueNumber float64 = 0

// DequeueNumber dequeue number
var DequeueNumber float64 = 0

//NewActionMQ new etcd mq
func NewActionMQ(ctx context.Context, c option.Config) ActionMQ {
	etcdQueue := etcdQueue{
		config: c,
		ctx:    ctx,
		queues: make(map[string]string),
	}
	return &etcdQueue
}

type etcdQueue struct {
	config     option.Config
	ctx        context.Context
	queues     map[string]string
	queuesLock sync.Mutex
	client     *clientv3.Client
}

func (e *etcdQueue) Start() error {
	logrus.Debug("etcd message queue client starting")
	etcdClientArgs := &etcdutil.ClientArgs{
		Endpoints:   e.config.EtcdEndPoints,
		CaFile:      e.config.EtcdCaFile,
		CertFile:    e.config.EtcdCertFile,
		KeyFile:     e.config.EtcdKeyFile,
		DialTimeout: time.Duration(e.config.EtcdTimeout) * time.Second,
	}
	cli, err := etcdutil.NewClient(context.Background(), etcdClientArgs)
	if err != nil {
		etcdutil.HandleEtcdError(err)
		return err
	}
	e.client = cli
	topics := os.Getenv("topics")
	if topics != "" {
		ts := strings.Split(topics, ",")
		for _, t := range ts {
			e.registerTopic(t)
		}
	}
	e.registerTopic(client.BuilderTopic)
	e.registerTopic(client.WindowsBuilderTopic)
	e.registerTopic(client.WorkerTopic)
	logrus.Info("etcd message queue client started success")
	return nil
}

//registerTopic 注册消息队列主题
func (e *etcdQueue) registerTopic(topic string) {
	e.queuesLock.Lock()
	defer e.queuesLock.Unlock()
	e.queues[topic] = topic
}

func (e *etcdQueue) TopicIsExist(topic string) bool {
	e.queuesLock.Lock()
	defer e.queuesLock.Unlock()
	_, ok := e.queues[topic]
	return ok
}
func (e *etcdQueue) GetAllTopics() []string {
	var topics []string
	for k := range e.queues {
		topics = append(topics, k)
	}
	return topics
}

func (e *etcdQueue) Stop() error {
	if e.client != nil {
		e.client.Close()
	}
	return nil
}
func (e *etcdQueue) queueKey(topic string) string {
	return e.config.EtcdPrefix + "/" + topic
}
func (e *etcdQueue) Enqueue(ctx context.Context, topic, value string) error {
	EnqueueNumber++
	queue := etcdutil.NewQueue(ctx, e.client, e.queueKey(topic))
	return queue.Enqueue(value)
}

func (e *etcdQueue) Dequeue(ctx context.Context, topic string) (string, error) {
	DequeueNumber++
	queue := etcdutil.NewQueue(ctx, e.client, e.queueKey(topic))
	return queue.Dequeue()
}

func (e *etcdQueue) MessageQueueSize(topic string) int64 {
	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	res, err := e.client.Get(ctx, e.queueKey(topic), clientv3.WithPrefix())
	if err != nil {
		logrus.Errorf("get message queue size failure %s", err.Error())
	}
	if res != nil {
		return res.Count
	}
	return 0
}
