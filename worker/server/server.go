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
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/goodrain/rainbond/util"

	discover "github.com/goodrain/rainbond/discover.v2"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/server/pb"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

//RuntimeServer app runtime grpc server
type RuntimeServer struct {
	ctx       context.Context
	cancel    context.CancelFunc
	store     store.Storer
	conf      option.Config
	server    *grpc.Server
	hostIP    string
	keepalive *discover.KeepAlive
}

//CreaterRuntimeServer create a runtime grpc server
func CreaterRuntimeServer(conf option.Config, store store.Storer) *RuntimeServer {
	ctx, cancel := context.WithCancel(context.Background())
	rs := &RuntimeServer{
		conf:   conf,
		ctx:    ctx,
		cancel: cancel,
		server: grpc.NewServer(),
		hostIP: conf.HostIP,
		store:  store,
	}
	pb.RegisterAppRuntimeSyncServer(rs.server, rs)
	// Register reflection service on gRPC server.
	reflection.Register(rs.server)
	return rs
}

//Start start runtime server
func (r *RuntimeServer) Start(errchan chan error) {
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", r.conf.HostIP, r.conf.ServerPort))
		if err != nil {
			logrus.Errorf("failed to listen: %v", err)
			errchan <- err
		}
		if err := r.server.Serve(lis); err != nil {
			errchan <- err
		}
	}()
	if err := r.registServer(); err != nil {
		errchan <- err
	}
}

//GetAppStatus get app service status
func (r *RuntimeServer) GetAppStatus(ctx context.Context, re *pb.ServicesRequest) (*pb.StatusMessage, error) {
	status := r.store.GetAppServicesStatus(strings.Split(re.ServiceIds, ","))
	return &pb.StatusMessage{
		Status: status,
	}, nil
}

//GetAppDisk get app service volume disk size
func (r *RuntimeServer) GetAppDisk(ctx context.Context, re *pb.ServicesRequest) (*pb.DiskMessage, error) {
	return nil, nil
}

//GetAppPods get app pod list
func (r *RuntimeServer) GetAppPods(ctx context.Context, re *pb.ServiceRequest) (*pb.ServiceAppPodList, error) {
	var Pods []*pb.ServiceAppPod
	app := r.store.GetAppService(re.ServiceId)
	if app == nil {
		return &pb.ServiceAppPodList{
			Pods: Pods,
		}, nil
	}
	var deployType, deployID string
	if deployment := app.GetDeployment(); deployment != nil {
		deployType = "deployment"
		deployID = deployment.Name
	}
	if statefulset := app.GetStatefulSet(); statefulset != nil {
		deployType = "statefulset"
		deployID = statefulset.Name
	}
	pods := app.GetPods()
	for _, pod := range pods {
		var containers = make(map[string]*pb.Container, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			containers[container.Name] = &pb.Container{
				ContainerName: container.Name,
				MemoryLimit:   int32(container.Resources.Limits.Memory().Value()),
			}
		}
		Pods = append(Pods, &pb.ServiceAppPod{
			ServiceId:  app.ServiceID,
			DeployId:   deployID,
			DeployType: deployType,
			PodIp:      pod.Status.PodIP,
			PodName:    pod.Name,
			PodStatus:  string(pod.Status.Phase),
			Containers: containers,
		})
	}

	return &pb.ServiceAppPodList{
		Pods: Pods,
	}, nil
}

//GetDeployInfo get deploy info
func (r *RuntimeServer) GetDeployInfo(ctx context.Context, re *pb.ServiceRequest) (*pb.DeployInfo, error) {
	var deployinfo pb.DeployInfo
	appService := r.store.GetAppService(re.ServiceId)
	if appService != nil {
		deployinfo.Namespace = appService.TenantID
		if appService.GetStatefulSet() != nil {
			deployinfo.Statefuleset = appService.GetStatefulSet().Name
		}
		if appService.GetDeployment() != nil {
			deployinfo.Deployment = appService.GetDeployment().Name
		}
		if services := appService.GetServices(); services != nil {
			service := make(map[string]string, len(services))
			for _, s := range services {
				service[s.Name] = s.Name
			}
			deployinfo.Services = service
		}
		if secrets := appService.GetSecrets(); secrets != nil {
			secretsinfo := make(map[string]string, len(secrets))
			for _, s := range secrets {
				secretsinfo[s.Name] = s.Name
			}
			deployinfo.Secrets = secretsinfo
		}
		if ingresses := appService.GetIngress(); ingresses != nil {
			ingress := make(map[string]string, len(ingresses))
			for _, s := range ingresses {
				ingress[s.Name] = s.Name
			}
			deployinfo.Ingresses = ingress
		}
		if pods := appService.GetPods(); pods != nil {
			podNames := make(map[string]string, len(pods))
			for _, s := range pods {
				podNames[s.Name] = s.Name
			}
			deployinfo.Pods = podNames
		}
		if rss := appService.GetReplicaSets(); rss != nil {
			rsnames := make(map[string]string, len(rss))
			for _, s := range rss {
				rsnames[s.Name] = s.Name
			}
			deployinfo.Replicatset = rsnames
		}
		deployinfo.Status = appService.GetServiceStatus()
	}
	return &deployinfo, nil
}

//registServer
//regist sync server to etcd
func (r *RuntimeServer) registServer() error {
	if !r.store.Ready() {
		util.Exec(r.ctx, func() error {
			if r.store.Ready() {
				return fmt.Errorf("Ready")
			}
			logrus.Debugf("store module is not ready,runtime server is  waiting")
			return nil
		}, time.Second*3)
	}
	if r.keepalive == nil {
		keepalive, err := discover.CreateKeepAlive(r.conf.EtcdEndPoints, "app_sync_runtime_server", "", r.conf.HostIP, r.conf.ServerPort)
		if err != nil {
			return fmt.Errorf("create app sync server keepalive error,%s", err.Error())
		}
		r.keepalive = keepalive
	}
	return r.keepalive.Start()
}
