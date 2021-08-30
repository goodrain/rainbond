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

package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/eventlog/cluster/connect"
	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/goodrain/rainbond/eventlog/db"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/leader"
	"k8s.io/client-go/kubernetes"

	"golang.org/x/net/context"

	"github.com/goodrain/rainbond/eventlog/store"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

//Cluster 集群模块对外服务
type Cluster interface {
	//Select an instance that processes component logs
	GetSuitableInstance(serviceID string) *connect.Instance
	Start() error
	Stop()
	GetCurrentInstance() *connect.Instance
	GetLeaderInstance() *connect.Instance
	Scrape(ch chan<- prometheus.Metric, namespace, exporter string) error
}

//ClusterManager 控制器
type ClusterManager struct {
	currentInstance, leaderInstance *connect.Instance
	zmqPub                          *connect.Pub
	zmqSub                          *connect.Sub
	Conf                            conf.ClusterConf
	log                             *logrus.Entry
	storeManager                    store.Manager
	cancel                          func()
	context                         context.Context
	kubeClient                      *kubernetes.Clientset
	leaderlock                      sync.Mutex
}

//NewCluster 创建集群控制器
func NewCluster(ctx context.Context, conf conf.ClusterConf, kubeClient *kubernetes.Clientset, log *logrus.Entry, storeManager store.Manager) Cluster {
	ctx, cancel := context.WithCancel(ctx)
	current := &connect.Instance{
		HostIP:           conf.HostIP,
		PubPort:          conf.PubSub.PubBindPort,
		WebsocketPort:    conf.WebsocketPort,
		DockerLogPort:    conf.DockerLogPort,
		Status:           "health",
		IsLeader:         false,
		ComponentLogChan: []string{},
	}
	sub := connect.NewSub(conf.PubSub, log.WithField("module", "MessageSubManager"), storeManager, current)
	pub := connect.NewPub(conf.PubSub, log.WithField("module", "MessagePubServer"), storeManager)
	return &ClusterManager{
		kubeClient:      kubeClient,
		currentInstance: current,
		zmqSub:          sub,
		zmqPub:          pub,
		Conf:            conf,
		log:             log,
		storeManager:    storeManager,
		cancel:          cancel,
		context:         ctx,
	}
}

//Start 启动
func (s *ClusterManager) Start() error {
	if err := s.zmqPub.Run(); err != nil {
		return err
	}
	if err := s.zmqSub.Run(); err != nil {
		return err
	}
	go s.runServer()
	return s.selectLeader()
}

//Stop 停止
func (s *ClusterManager) Stop() {
	s.cancel()
	s.zmqPub.Stop()
	s.zmqSub.Stop()
}

//GetSuitableInstance The selection of log nodes is handled by the leader node.
func (s *ClusterManager) GetSuitableInstance(serviceID string) *connect.Instance {
	if s.currentInstance.IsLeader {
		ins := s.getSuitableInstance(serviceID)
		logrus.Infof("select instance %s for component %s", ins.HostIP, serviceID)
		return ins
	}
	if s.leaderInstance != nil {
		reqURL := fmt.Sprintf("http://%s:%d/get_component_instance?component_id=%s", s.leaderInstance.HostIP, s.Conf.ClusterPort, serviceID)
		res, err := http.Get(reqURL)
		if err != nil {
			logrus.Errorf("get component instance from leader failure %s", err.Error())
		}
		if res != nil && res.StatusCode == 200 {
			defer res.Body.Close()
			var in connect.Instance
			if err := json.NewDecoder(res.Body).Decode(&in); err == nil {
				logrus.Infof("select instance %s from leader for component %s", in.HostIP, serviceID)
				return &in
			} else {
				logrus.Errorf("select instance from leader for component failure %s", serviceID, err.Error())
			}
		} else if res != nil {
			logrus.Errorf("select instance from leader for component failure, status code is %d", res.StatusCode)
		}
	}
	logrus.Warnf("not select instance for component %s", serviceID)
	return nil
}

func (s *ClusterManager) getSuitableInstance(componentID string) *connect.Instance {
	s.leaderlock.Lock()
	defer s.leaderlock.Unlock()
	var selectInstance *connect.Instance
	var minInstance *connect.Instance
	for _, instance := range s.zmqSub.GetIntances(true) {
		if util.StringArrayContains(instance.ComponentLogChan, componentID) {
			selectInstance = instance
		}
		if minInstance == nil || len(instance.ComponentLogChan) < len(minInstance.ComponentLogChan) {
			minInstance = instance
		}
	}
	if selectInstance == nil && minInstance != nil {
		selectInstance = minInstance
	}
	if selectInstance != nil {
		selectInstance.ComponentLogChan = append(selectInstance.ComponentLogChan, componentID)
		return selectInstance
	}
	s.currentInstance.ComponentLogChan = append(s.currentInstance.ComponentLogChan, componentID)
	return s.currentInstance
}

func (s *ClusterManager) GetCurrentInstance() *connect.Instance {
	return s.currentInstance
}

func (s *ClusterManager) GetLeaderInstance() *connect.Instance {
	return s.leaderInstance
}
func (s *ClusterManager) runServer() {
	s.log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", s.Conf.ClusterPort), s.ServeHTTP()))
}
func (s *ClusterManager) ServeHTTP() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/regist", func(res http.ResponseWriter, req *http.Request) {
		// only leader handle server request
		if s.currentInstance.IsLeader {
			if req.Body != nil {
				defer req.Body.Close()
				var in connect.Instance
				if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
					s.log.Errorf("decode regist node message failure %s", err.Error())
				} else {
					// keepalive ComponentLogChan
					if ins := s.zmqSub.GetIntance(in.Key()); ins != nil {
						in.ComponentLogChan = ins.ComponentLogChan
					}
					s.zmqSub.Sub(&in)
					s.log.Infof("receive node  %s regist message", in.HostIP)
					s.pubNodeMeta()
				}
			}
			res.WriteHeader(200)
		} else {
			res.WriteHeader(411)
		}
	})
	mux.HandleFunc("/get_component_instance", func(res http.ResponseWriter, req *http.Request) {
		// only leader handle server request
		if s.currentInstance.IsLeader {
			componentID := req.FormValue("component_id")
			if componentID == "" {
				res.WriteHeader(412)
				res.Write([]byte(`{"message":"component id can not be empty.","status":"failure"}`))
				return
			}
			instance := s.getSuitableInstance(componentID)
			if instance != nil {
				res.WriteHeader(200)
				json.NewEncoder(res).Encode(instance)
			} else {
				res.WriteHeader(412)
			}
		} else {
			res.WriteHeader(411)
		}
	})
	return mux
}

func (s *ClusterManager) startAsLeader(ctx context.Context) {
	//pub all instance meta info to all instance
	defer func() {
		s.leaderInstance = nil
		s.currentInstance.IsLeader = false
		s.log.Info("lose leader")
	}()
	s.currentInstance.IsLeader = true
	s.leaderInstance = s.currentInstance
	s.log.Info("running as leader")
	timer := time.NewTimer(time.Second * 10)
	defer timer.Stop()
	for {
		s.pubNodeMeta()
		timer.Reset(time.Second * 10)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func (s *ClusterManager) selectLeader() error {
	// Leader election was requested.
	if s.Conf.LeaderElectionNamespace == "" {
		return fmt.Errorf("-leader-election-namespace must not be empty")
	}
	leaderElectionIdentity := fmt.Sprintf("%s:%d", s.Conf.HostIP, s.Conf.PubSub.PubBindPort)

	// Name of config map with leader election lock
	lockName := "rainbond-eventlog-leader"

	// Become leader again on stop leading.
	leaderCh := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-s.context.Done():
				return
			case <-leaderCh:
				func() {
					logrus.Info("try run as leader")
					ctx, cancel := context.WithCancel(s.context)
					defer cancel()
					leader.RunAsLeader(ctx, s.kubeClient, s.Conf.LeaderElectionNamespace, leaderElectionIdentity, lockName, s.startAsLeader, func() {
						leaderCh <- struct{}{}
						s.currentInstance.IsLeader = false
						logrus.Info("restart leader")
					}, func(leaderKey string) {
						connectInfo := strings.Split(leaderKey, ":")
						if len(connectInfo) == 2 {
							port, _ := strconv.Atoi(connectInfo[1])
							leader := &connect.Instance{HostIP: connectInfo[0], PubPort: port, IsLeader: true}
							s.leaderInstance = leader
							if connectInfo[0] != s.currentInstance.HostIP {
								logrus.Infof("node %s discover leader instance %s, will sub it", s.currentInstance.HostIP, leaderKey)
								s.zmqSub.Sub(leader)
								s.registNode(ctx, leader)
							}
						}
					})
				}()
			}
		}
	}()
	leaderCh <- struct{}{}
	return nil
}

func (s *ClusterManager) pubNodeMeta() {
	s.zmqSub.UpdateIntances()
	intences := s.zmqSub.GetIntances(true)
	intences = append(intences, s.currentInstance)
	message := connect.InstanceMessage{
		Instances:    intences,
		LeaderHostIP: s.currentInstance.HostIP,
	}
	data, _ := json.Marshal(message)
	s.zmqPub.RadioChan <- db.ClusterMessage{Mode: db.ClusterMetaMessage, Data: data}
}

func (s *ClusterManager) registNode(ctx context.Context, leader *connect.Instance) {
	ticker := time.NewTimer(time.Second * 5)
	defer ticker.Stop()
	for {
		// leader maybe changed.
		if leader.HostIP != s.leaderInstance.HostIP {
			return
		}
		reqURL := fmt.Sprintf("http://%s:%d/regist", s.leaderInstance.HostIP, s.Conf.ClusterPort)
		data, _ := json.Marshal(s.currentInstance)
		res, err := http.Post(reqURL, "application/json", bytes.NewBuffer(data))
		if err != nil {
			s.log.Errorf("regist node to leader %s failure %v", s.leaderInstance.HostIP, err)
		} else if res.StatusCode != 200 {
			s.log.Errorf("regist node to leader %s failure: response code is %d", s.leaderInstance.HostIP, res.StatusCode)
		} else {
			s.log.Infof("regist node %s to leader %s success", s.currentInstance.HostIP, s.leaderInstance.HostIP)
			return
		}
		ticker.Reset(time.Second * 5)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

//Scrape prometheus monitor metrics
func (s *ClusterManager) Scrape(ch chan<- prometheus.Metric, namespace, exporter string) error {
	subDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "sub_instance_count"),
		"Number of subscribed nodes",
		[]string{"instance_ip"}, nil,
	)
	insDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "instance_count"),
		"Number of nodes",
		[]string{"instance_ip"}, nil,
	)
	inshealthDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "instance_health_count"),
		"Number of health nodes",
		[]string{"instance_ip"}, nil,
	)
	leaderInstance := prometheus.NewDesc(prometheus.BuildFQName(namespace, exporter, "leader"),
		"leader instance",
		[]string{"leader_ip"}, nil)

	ch <- prometheus.MustNewConstMetric(subDesc, prometheus.GaugeValue, float64(s.zmqSub.GetSubNumber()), s.currentInstance.HostIP)
	if s.leaderInstance != nil {
		ch <- prometheus.MustNewConstMetric(leaderInstance, prometheus.GaugeValue, float64(1), s.leaderInstance.HostIP)
	}
	if s.currentInstance.IsLeader {
		ch <- prometheus.MustNewConstMetric(insDesc, prometheus.GaugeValue, float64(len(s.zmqSub.GetIntances(false))+1), s.currentInstance.HostIP)
		ch <- prometheus.MustNewConstMetric(inshealthDesc, prometheus.GaugeValue, float64(len(s.zmqSub.GetIntances(true))+1), s.leaderInstance.HostIP)
	}
	//TODO: Improve more monitoring data.
	return nil
}
