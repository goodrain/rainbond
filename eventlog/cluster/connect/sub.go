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

package connect

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/goodrain/rainbond/eventlog/db"
	"github.com/goodrain/rainbond/eventlog/store"
	"github.com/goodrain/rainbond/util"
	"github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type Instance struct {
	HostIP  string `json:"hostIP"`
	PubPort int    `json:"pubPort"`
	//Port that receives container logs.
	DockerLogPort    int       `json:"dockerLogPort"`
	WebsocketPort    int       `json:"websocketPort`
	Status           string    `json:"status"`
	StatusUpdate     time.Time `json:"statusUpdate"`
	IsLeader         bool      `json:"isLeader"`
	ComponentLogChan []string  `json:"componentLogChan"`
}

func (i *Instance) Key() string {
	return fmt.Sprintf("%s:%d", i.HostIP, i.PubPort)
}

type InstanceMessage struct {
	Instances    []*Instance `json:"instances"`
	LeaderHostIP string      `json:"leaderHostIP"`
}

// Sub -
type Sub struct {
	conf                conf.PubSubConf
	log                 *logrus.Entry
	cancel              func()
	context             context.Context
	storemanager        store.Manager
	subMessageChan      chan [][]byte
	listenErr           chan error
	instanceMap         map[string]*Instance
	instanceMessageChan chan *InstanceMessage
	subClient           map[string]*SubClient
	mapLock             sync.Mutex
	subLock             sync.Mutex
	current             *Instance
}

// SubClient -
type SubClient struct {
	context *zmq4.Context
	socket  *zmq4.Socket
	quit    chan interface{}
}

//NewSub 创建zmq sub客户端
func NewSub(conf conf.PubSubConf, log *logrus.Entry, storeManager store.Manager, current *Instance) *Sub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Sub{
		conf:                conf,
		log:                 log,
		cancel:              cancel,
		context:             ctx,
		storemanager:        storeManager,
		listenErr:           make(chan error),
		instanceMessageChan: make(chan *InstanceMessage, 8),
		instanceMap:         make(map[string]*Instance),
		subClient:           make(map[string]*SubClient),
		subMessageChan:      storeManager.SubMessageChan(),
		current:             current,
	}
}

//Run 执行
func (s *Sub) Run() error {
	go s.instanceListen()
	go s.checkHealth()
	s.log.Info("Message Sub manager start")
	return nil
}

//Stop 停止
func (s *Sub) Stop() {
	s.cancel()
	s.subLock.Lock()
	defer s.subLock.Unlock()
	for _, v := range s.subClient {
		close(v.quit)
	}
	s.log.Info("Message Sub manager stop")
}

func (s *Sub) GetIntance(key string) *Instance {
	return s.instanceMap[key]
}

func (s *Sub) Sub(instance *Instance) {
	s.subLock.Lock()
	defer s.subLock.Unlock()
	if instance.Key() == s.current.Key() {
		return
	}
	key := fmt.Sprintf("%s:%d", instance.HostIP, instance.PubPort)
	if _, ok := s.instanceMap[key]; !ok {
		go s.listen(instance)
		instance.Status = "unhealth"
		s.instanceMap[key] = instance
	}
}
func (s *Sub) UpdateIntances() {
	s.mapLock.Lock()
	defer s.mapLock.Unlock()
	for _, v := range s.instanceMap {
		if v.StatusUpdate.Add(time.Second * 10).Before(time.Now()) {
			s.log.Infof("instance %s maybe not health", v.HostIP)
			v.Status = "unhealth"
			v.StatusUpdate = time.Now()
		}
	}
}
func (s *Sub) GetIntances(health bool) (re []*Instance) {
	s.mapLock.Lock()
	defer s.mapLock.Unlock()
	for _, v := range s.instanceMap {
		if v.Status == "health" || !health {
			re = append(re, v)
		}
	}
	return
}
func (s *Sub) GetSubNumber() int {
	s.subLock.Lock()
	defer s.subLock.Unlock()
	return len(s.subClient)
}
func (s *Sub) checkHealth() {
	tike := time.NewTicker(time.Minute * 2)
	defer tike.Stop()
	for {
		select {
		case <-s.context.Done():
			return
		case <-tike.C:
		}
		var unHealth []string
		s.mapLock.Lock()
		s.subLock.Lock()
		for k, ins := range s.instanceMap {
			if _, ok := s.subClient[k]; !ok {
				go s.listen(ins)
				unHealth = append(unHealth, ins.HostIP)
			}
		}
		s.subLock.Unlock()
		s.mapLock.Unlock()
		if len(unHealth) == 0 {
			s.log.Debug("sub manager check listen client health: All health.")
		} else {
			s.log.Info("sub manager check listen client health: UnHealth instances:", unHealth)
		}

	}
}
func (s *Sub) instanceListen() {
	for {
		select {
		case instances := <-s.instanceMessageChan:
			if instances != nil {
				func() {
					s.mapLock.Lock()
					defer s.mapLock.Unlock()
					var newKeys []string
					for _, instance := range instances.Instances {
						if instance.Key() == s.current.Key() {
							continue
						}
						key := fmt.Sprintf("%s:%d", instance.HostIP, instance.PubPort)
						if _, ok := s.instanceMap[key]; !ok {
							go s.listen(instance)
						}
						s.instanceMap[key] = instance
						newKeys = append(newKeys, key)
					}
					for k, instance := range s.instanceMap {
						if !util.StringArrayContains(newKeys, k) {
							s.log.Infof("sub manager release the subscription to the %s node", instance.HostIP)
							delete(s.instanceMap, k)
							s.unlisten(instance)
						}
					}
				}()
			}
		case <-s.context.Done():
			s.mapLock.Lock()
			s.instanceMap = nil
			s.mapLock.Unlock()
			s.log.Debug("Instance listen manager stop.")
			return
		}
	}
}

func (s *Sub) listen(ins *Instance) {
	if ins.Key() == s.current.Key() {
		return
	}
	chQuit := make(chan interface{})
	chErr := make(chan error, 2)
	newInstanceListen := func(sock *zmq4.Socket, instance string) {
		socketHandler := func(state zmq4.State) error {
			msgs, err := sock.RecvMessageBytes(0)
			if err != nil {
				s.log.Error("sub client receive message error.", err.Error())
				return err
			}
			if len(msgs) == 2 {
				switch string(msgs[0]) {
				case string(db.EventMessage), string(db.ServiceMonitorMessage), string(db.ServiceNewMonitorMessage):
					s.subMessageChan <- msgs
				case string(db.ClusterMetaMessage):
					var instances InstanceMessage
					if err := json.Unmarshal(msgs[1], &instances); err != nil {
						s.log.Errorf("unmarshal cluster monitor message failure %s, message: %s", err.Error(), string(msgs[1]))
					} else {
						s.instanceMessageChan <- &instances
					}
				case string(db.HealthMessage):
					s.log.Debugf("receive health message from instance %s", ins.HostIP)
					ins.Status = "health"
					ins.StatusUpdate = time.Now()
				default:
					s.log.Error("cluster sub receive a message protocol error.")
				}
			} else {
				s.log.Error("cluster sub receive a message protocol error.")
			}
			return nil
		}
		quitHandler := func(interface{}) error {
			s.log.Infof("Sub instance %s quit.", instance)
			sock.Close()
			return errors.New("Quit")
		}
		reactor := zmq4.NewReactor()
		reactor.AddSocket(sock, zmq4.POLLIN, socketHandler)
		reactor.AddChannel(chQuit, 0, quitHandler)
		err := reactor.Run(s.conf.PollingTimeout)
		chErr <- err
	}
	for {
		context, err := zmq4.NewContext()
		if err != nil {
			s.log.Error("create sub context error", err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		subscriber, err := context.NewSocket(zmq4.SUB)
		if err != nil {
			s.log.Errorf("create sub client to instance %s error %s", ins.HostIP, err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		err = subscriber.Connect(fmt.Sprintf("tcp://%s:%d", ins.HostIP, ins.PubPort))
		if err != nil {
			s.log.Errorf("sub client connect to instance host %s port %d error", ins.HostIP, ins.PubPort)
			time.Sleep(time.Second * 5)
			continue
		}
		subscriber.SetSubscribe("")
		go newInstanceListen(subscriber, ins.HostIP)

		key := fmt.Sprintf("%s:%d", ins.HostIP, ins.PubPort)
		s.subLock.Lock()
		client := &SubClient{socket: subscriber, context: context, quit: chQuit}
		s.subClient[key] = client
		s.subLock.Unlock()
		s.log.Infof("message client sub instance %s ", ins.HostIP)
		break
	}
}

func (s *Sub) unlisten(ins *Instance) {
	s.subLock.Lock()
	defer s.subLock.Unlock()
	key := fmt.Sprintf("%s:%d", ins.HostIP, ins.PubPort)
	if client, ok := s.subClient[key]; ok {
		client.quit <- true
		close(client.quit)
		delete(s.subClient, key)
	}
}
