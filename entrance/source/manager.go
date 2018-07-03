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

package source

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/goodrain/rainbond/cmd/entrance/option"
	"github.com/goodrain/rainbond/entrance/core"
	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/source/config"

	kubeerrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Manager struct {
	RegionAPI         string
	LBAPIPort         string
	Token             string
	Ctx               context.Context
	Cancel            context.CancelFunc
	CoreManager       core.Manager
	ClientSet         *kubernetes.Clientset
	ErrChan           chan error
	ServiceUpdateChan chan config.ServiceUpdate
	PodUpdateChan     chan config.PodUpdate
	stopChan          chan struct{}
}

//NewSourceManager new
func NewSourceManager(c option.Config, coreManager core.Manager, errChan chan error) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	kubeconfig := c.K8SConfPath
	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logrus.Error(err)
	}
	conf.QPS = 50
	conf.Burst = 100
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Info("Kube client api create success.")
	ports := strings.Split(c.APIAddr, ":")
	if len(ports) < 2 {
		logrus.Errorf("Does your API port listen correctly?")
	}
	return &Manager{
		RegionAPI:         c.RegionAPIAddr,
		LBAPIPort:         ports[1],
		Token:             c.Token,
		Cancel:            cancel,
		Ctx:               ctx,
		CoreManager:       coreManager,
		ClientSet:         clientset,
		ErrChan:           errChan,
		ServiceUpdateChan: make(chan config.ServiceUpdate, 10),
		PodUpdateChan:     make(chan config.PodUpdate, 10),
		stopChan:          make(chan struct{}),
	}
}

//Start 启动
func (m *Manager) Start() error {
	logrus.Info("source manager starting...")
	go m.PodsLW()
	go m.ServicesLW()
	config.NewSourceAPI(m.ClientSet.Core().RESTClient(),
		15*time.Minute,
		m.ServiceUpdateChan,
		m.PodUpdateChan,
		m.stopChan,
	)
	logrus.Info("source manager started")
	return nil
}

//Stop 停止
func (m *Manager) Stop() error {
	logrus.Info("Source manager is stoping.")
	close(m.stopChan)
	m.Cancel()
	return nil
}

//NodeIsReady check node is ready
//TODO:
func (m *Manager) NodeIsReady(n *object.NodeObject) bool {
	var rc bool
	podName, err := m.readPodName(n.NodeName)
	if err != nil {
		logrus.Warn(err)
		return true
	}
	pods, err := m.ClientSet.CoreV1().Pods(n.Namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		if cerr, ok := err.(*kubeerrors.StatusError); ok {
			if cerr.Status().Code == 404 {
				return false
			}
			logrus.Errorf("List pod source error. Code is %d", cerr.Status().Code)
			return true
		}
		logrus.Errorf("List pod source error. %s", err.Error())
		return true
	}
	if pods.ResourceVersion != fmt.Sprintf("%d", n.Index) {
		logrus.Warnf("old version %s ,new version %d", pods.ResourceVersion, n.Index)
		return false
	}
	for _, statusInfo := range pods.Status.Conditions {
		if statusInfo.Type == "Ready" && statusInfo.Status == "True" {
			rc = true
		}
	}
	return rc
}

func (m *Manager) readPodName(strName string) (string, error) {
	var s string
	exp := regexp.MustCompile(config.NodeExpString)
	lexp := exp.FindStringSubmatch(strName)
	if len(lexp) != 3 {
		return s, errors.New("Name is illegal.Node name is " + strName)
	}
	return lexp[1], nil
}

func (m *Manager) readServiceName(strName string) (*config.ServiceInfo, error) {
	var s config.ServiceInfo
	if bytes.HasSuffix([]byte(strName), []byte(".Pool")) || bytes.HasSuffix([]byte(strName), []byte(".VS")) {
		exp := regexp.MustCompile(config.VSPoolExpString)
		lexp := exp.FindStringSubmatch(strName)
		if len(lexp) != 4 {
			return &s, errors.New("Name is illegal.Strname is " + strName)
		}
		s.SerName = lexp[2] + "ServiceOUT"
		s.Port = lexp[3]
	} else {
		return &s, errors.New("Name is illegal.Strname is " + strName)
	}
	return &s, nil
}

func (m *Manager) hasServiceSource(s *config.ServiceInfo) bool {
	logrus.Debug("checkout service name %s", s.SerName)
	lservices, err := m.ClientSet.CoreV1().Services(s.Namespace).List(metav1.ListOptions{
		LabelSelector: "name=" + s.SerName,
	})
	if err != nil {
		if cerr, ok := err.(*kubeerrors.StatusError); ok {
			if cerr.Status().Code == 404 {
				logrus.Debug("False info: services 404 false")
				return false
			}
		}
		logrus.Error("List pod source error.", err.Error())
		return true
	}
	if len(lservices.Items) == 0 {
		logrus.Debug("False info: no services sources false.")
		return false
	}
	//不检查service的版本，由于缓存资源的版本可能是pod的版本
	// for _, servicesInfo := range lservices.Items {
	// 	if servicesInfo.ResourceVersion != fmt.Sprintf("%d", s.Index) {
	// 		logrus.Debugf("False info: services resourceVersion false. old, %d; new, %s",
	// 			s.Index,
	// 			servicesInfo.ResourceVersion)
	// 		return false
	// 	}
	// }
	return true
}

//PoolIsReady check pool whether ready
// if pool is exist ,it can be ready
//TODO:
func (m *Manager) PoolIsReady(p *object.PoolObject) bool {
	s, err := m.readServiceName(p.Name)
	if err != nil {
		logrus.Warn(err)
		return true
	}
	s.Index = p.Index
	s.Namespace = p.Namespace
	return m.hasServiceSource(s)
}

//VSIsReady check vs is ready
//TODO:
func (m *Manager) VSIsReady(v *object.VirtualServiceObject) bool {
	s, err := m.readServiceName(v.Name)
	if err != nil {
		logrus.Warn(err)
		return true
	}
	s.Index = v.Index
	s.Namespace = v.Namespace
	return m.hasServiceSource(s)
}
