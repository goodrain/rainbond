// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/appruntimesync/cache"
	"github.com/goodrain/rainbond/appruntimesync/clean"
	"github.com/goodrain/rainbond/appruntimesync/pb"
	"github.com/goodrain/rainbond/appruntimesync/pod"
	"github.com/goodrain/rainbond/appruntimesync/source"
	"github.com/goodrain/rainbond/appruntimesync/status"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

//AppRuntimeSyncServer AppRuntimeSyncServer
type AppRuntimeSyncServer struct {
	StatusManager *status.Manager
	c             option.Config
	stopChan      chan struct{}
	Ctx           context.Context
	Cancel        context.CancelFunc
	ClientSet     *kubernetes.Clientset
	podCache      *pod.CacheManager
	clean         *clean.Manager
	cache         *cache.DiskCache
}

//NewAppRuntimeSyncServer create app runtime sync server
func NewAppRuntimeSyncServer(conf option.Config) *AppRuntimeSyncServer {
	ctx, cancel := context.WithCancel(context.Background())
	kubeconfig := conf.KubeConfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logrus.Error(err)
	}
	config.QPS = 50
	config.Burst = 100
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Info("Kube client api create success.")
	statusManager := status.NewManager(ctx, clientset)
	stopChan := make(chan struct{})
	podCache := pod.NewCacheManager(clientset)
	Clean, err := clean.NewManager(ctx, clientset)
	if err != nil {
		logrus.Error(err)
	}

	// disk used info collector
	c := cache.CreatDiskCache(ctx)

	arss := &AppRuntimeSyncServer{
		c:         conf,
		Ctx:       ctx,
		stopChan:  stopChan,
		Cancel:    cancel,
		ClientSet: clientset,
		podCache:  podCache,
		clean:     Clean,
		cache:     c,
	}
	arss.StatusManager = statusManager
	return arss
}

//GetAppStatus get specified app status
func (a *AppRuntimeSyncServer) GetAppStatus(ctx context.Context, sr *pb.StatusRequest) (*pb.StatusMessage, error) {
	var re pb.StatusMessage
	if sr.ServiceIds == "" {
		re.Status = a.StatusManager.GetAllStatus()
		return &re, nil
	}
	re.Status = make(map[string]string)
	if strings.Contains(sr.ServiceIds, ",") {
		ids := strings.Split(sr.ServiceIds, ",")
		for _, id := range ids {
			re.Status[id] = a.StatusManager.GetStatus(id)
		}
		return &re, nil
	}
	re.Status[sr.ServiceIds] = a.StatusManager.GetStatus(sr.ServiceIds)
	return &re, nil
}

//GetAppDisk get app disk information
func (a *AppRuntimeSyncServer) GetAppDisk(ctx context.Context, sr *pb.StatusRequest) (*pb.DiskMessage, error) {
	var re pb.DiskMessage
	if sr.ServiceIds == "" {
		re.Disks = a.cache.Get()
		return &re, nil
	}
	services := strings.Split(sr.ServiceIds, ",")
	var rev = make(map[string]float64)
	for _, s := range services {
		value := a.cache.GetServiceDisk(s)
		rev[s] = value
	}
	re.Disks = rev
	return &re, nil
}

//SetAppStatus set app status
func (a *AppRuntimeSyncServer) SetAppStatus(ctx context.Context, ps *pb.StatusMessage) (*pb.ErrorMessage, error) {
	if ps.Status != nil {
		for k, v := range ps.Status {
			a.StatusManager.SetStatus(k, v)
		}
	}
	return &pb.ErrorMessage{Message: "success"}, nil
}

//CheckAppStatus check app status
func (a *AppRuntimeSyncServer) CheckAppStatus(ctx context.Context, ps *pb.StatusRequest) (*pb.ErrorMessage, error) {
	ids := strings.Split(ps.ServiceIds, ",")
	for _, id := range ids {
		a.StatusManager.CheckStatus(id)
	}
	return &pb.ErrorMessage{Message: "success"}, nil
}

//IgnoreDeleteEvent ignore resource delete event
func (a *AppRuntimeSyncServer) IgnoreDeleteEvent(ctx context.Context, pi *pb.Ignore) (*pb.ErrorMessage, error) {
	a.StatusManager.IgnoreDelete(pi.Name)
	return &pb.ErrorMessage{Message: "success"}, nil
}

//RmIgnoreDeleteEvent rm ignore resource delete event
func (a *AppRuntimeSyncServer) RmIgnoreDeleteEvent(ctx context.Context, pi *pb.Ignore) (*pb.ErrorMessage, error) {
	a.StatusManager.RmIgnoreDelete(pi.Name)
	return &pb.ErrorMessage{Message: "success"}, nil
}

//Start start
func (a *AppRuntimeSyncServer) Start() error {
	if err := a.StatusManager.Start(); err != nil {
		return err
	}
	logrus.Info("k8s source watching started...")
	go source.NewSourceAPI(a.ClientSet.Core().RESTClient(),
		a.ClientSet.AppsV1beta1().RESTClient(),
		15*time.Minute,
		a.StatusManager.RCUpdateChan,
		a.StatusManager.DeploymentUpdateChan,
		a.StatusManager.StatefulSetUpdateChan,
		a.stopChan,
	)
	a.podCache.Start()
	a.clean.Start()
	go a.cache.Start()
	logrus.Info("app runtime sync server started...")
	return nil
}

//Stop stop
func (a *AppRuntimeSyncServer) Stop() {
	a.Cancel()
	close(a.stopChan)
	a.podCache.Stop()
	a.clean.Stop()
}
