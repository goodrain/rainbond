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

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/discover.v2"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
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
	clientset kubernetes.Interface
	updateCh  *channels.RingChannel
}

//CreaterRuntimeServer create a runtime grpc server
func CreaterRuntimeServer(conf option.Config,
	store store.Storer,
	clientset kubernetes.Interface,
	updateCh *channels.RingChannel) *RuntimeServer {
	ctx, cancel := context.WithCancel(context.Background())
	rs := &RuntimeServer{
		conf:      conf,
		ctx:       ctx,
		cancel:    cancel,
		server:    grpc.NewServer(),
		hostIP:    conf.HostIP,
		store:     store,
		clientset: clientset,
		updateCh:  updateCh,
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
	var servicdIDs []string
	if re.ServiceIds != "" {
		servicdIDs = strings.Split(re.ServiceIds, ",")
	}
	status := r.store.GetAppServicesStatus(servicdIDs)
	return &pb.StatusMessage{
		Status: status,
	}, nil
}

//GetTenantResource get tenant resource
//if TenantId is "" will return the sum of the all tenant
func (r *RuntimeServer) GetTenantResource(ctx context.Context, re *pb.TenantRequest) (*pb.TenantResource, error) {
	var tr pb.TenantResource
	res := r.store.GetTenantResource(re.TenantId)
	if res == nil {
		return &tr, nil
	}
	// tr.RunningAppNum = int64(len(r.store.GetTenantRunningApp(re.TenantId)))
	// tr.RunningAppNum = int64(len(r.store.GetTenantRunningApp(re.TenantId)))
	runningApps := r.store.GetTenantRunningApp(re.TenantId)
	for _, app := range runningApps {
		if app.ServiceKind == model.ServiceKindThirdParty {
			tr.RunningAppThirdNum += 1
		} else if app.ServiceKind == model.ServiceKindInternal {
			tr.RunningAppInternalNum += 1
		}
	}
	tr.RunningAppNum = int64(len(runningApps))
	tr.CpuLimit = res.CPULimit
	tr.CpuRequest = res.CPURequest
	tr.MemoryLimit = res.MemoryLimit / 1024 / 1024
	tr.MemoryRequest = res.MemoryRequest / 1024 / 1024
	tr.UnscdCpuLimit = res.UnscdCPULimit
	tr.UnscdCpuReq = res.UnscdCPUReq
	tr.UnscdMemoryLimit = res.UnscdMemoryLimit / 1024 / 1024
	tr.UnscdMemoryReq = res.UnscdMemoryReq / 1024 / 1024
	return &tr, nil
}

//GetAppPods get app pod list
func (r *RuntimeServer) GetAppPods(ctx context.Context, re *pb.ServiceRequest) (*pb.ServiceAppPodList, error) {
	app := r.store.GetAppService(re.ServiceId)
	if app == nil {
		// TODO: make sure that '*pb.ServiceAppPodList' can be nil.
		return nil, nil
	}

	pods := app.GetPods()
	var oldpods, newpods []*pb.ServiceAppPod
	for _, pod := range pods {
		var containers = make(map[string]*pb.Container, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			containers[container.Name] = &pb.Container{
				ContainerName: container.Name,
				MemoryLimit:   container.Resources.Limits.Memory().Value(),
			}
		}
		sapod := &pb.ServiceAppPod{
			PodIp:      pod.Status.PodIP,
			PodName:    pod.Name,
			PodStatus:  string(pod.Status.Phase),
			Containers: containers,
		}
		if app.DistinguishPod(pod) {
			newpods = append(newpods, sapod)
		} else {
			oldpods = append(oldpods, sapod)
		}
	}

	return &pb.ServiceAppPodList{
		OldPods: oldpods,
		NewPods: newpods,
	}, nil
}

// GetPodEvents -
func (r *RuntimeServer) GetPodEvents(ctx context.Context, req *pb.GetPodEventsReq) (*pb.GetPodEventsResp, error) {
	app := r.store.GetAppService(req.Sid)
	if app == nil {
		return nil, nil // TODO: grpc allow return nil?
	}
	name := req.PodName
	namespace := app.TenantID
	pod := app.GetPodsByName(name)
	if pod == nil {
		eventsInterface := r.clientset.CoreV1().Events(namespace)
		selector := eventsInterface.GetFieldSelector(&name, &namespace, nil, nil)
		options := metav1.ListOptions{FieldSelector: selector.String()}
		events, err := eventsInterface.List(options)
		if err == nil && len(events.Items) > 0 {
			podEvents := DescribeEvents(events)
			result := &pb.GetPodEventsResp{
				Evnets: podEvents,
			}
			return result, nil
		}
		return nil, err
	}

	var events *corev1.EventList
	if ref, err := reference.GetReference(scheme.Scheme, pod); err != nil {
		logrus.Errorf("Unable to construct reference to '%#v': %v", pod, err)
	} else {
		ref.Kind = ""
		if _, isMirrorPod := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
			ref.UID = types.UID(pod.Annotations[corev1.MirrorPodAnnotationKey])
		}
		events, _ = r.clientset.CoreV1().Events(namespace).Search(scheme.Scheme, ref)
	}
	podEvents := DescribeEvents(events)
	result := &pb.GetPodEventsResp{
		Evnets: podEvents,
	}
	return result, nil
}

func GetPodDetail()

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

// formatEventSource formats EventSource as a comma separated string excluding Host when empty
func formatEventSource(es corev1.EventSource) string {
	EventSourceString := []string{es.Component}
	if len(es.Host) > 0 {
		EventSourceString = append(EventSourceString, es.Host)
	}
	return strings.Join(EventSourceString, ", ")
}

// DescribeEvents -
func DescribeEvents(el *corev1.EventList) []*pb.PodEvent {
	if len(el.Items) == 0 {
		return nil
	}
	// sort.Sort(event.SortableEvents(el.Items)) TODO
	var podEvents []*pb.PodEvent
	for _, e := range el.Items {
		var interval string
		if e.Count > 1 {
			interval = fmt.Sprintf("%s (x%d over %s)", translateTimestampSince(e.LastTimestamp), e.Count, translateTimestampSince(e.FirstTimestamp))
		} else {
			interval = translateTimestampSince(e.FirstTimestamp)
		}
		podEvent := &pb.PodEvent{
			Type:    e.Type,
			Reason:  e.Reason,
			Age:     interval,
			From:    formatEventSource(e.Source),
			Message: strings.TrimSpace(e.Message),
		}
		podEvents = append(podEvents, podEvent)
	}
	return podEvents
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
		if endpoints := appService.GetEndpoints(); endpoints != nil &&
			appService.AppServiceBase.ServiceKind == model.ServiceKindThirdParty {
			eps := make(map[string]string, len(endpoints))
			for _, s := range endpoints {
				eps[s.Name] = s.Name
			}
			deployinfo.Endpoints = eps
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

// ListThirdPartyEndpoints returns a collection of third-part endpoints.
func (r *RuntimeServer) ListThirdPartyEndpoints(ctx context.Context, re *pb.ServiceRequest) (*pb.ThirdPartyEndpoints, error) {
	as := r.store.GetAppService(re.ServiceId)
	if as == nil {
		return new(pb.ThirdPartyEndpoints), nil
	}
	var pbeps []*pb.ThirdPartyEndpoint
	// The same IP may correspond to two endpoints, which are internal and external endpoints.
	// So it is need to filter the same IP.
	exists := make(map[string]bool)
	for _, ep := range as.GetEndpoints() {
		if ep.Subsets == nil || len(ep.Subsets) == 0 {
			logrus.Debugf("Key: %s; empty subsets", fmt.Sprintf("%s/%s", ep.Namespace, ep.Name))
			continue
		}
		for idx, subset := range ep.Subsets {
			if exists[subset.Ports[0].Name] {
				continue
			}
			ip := func(subset corev1.EndpointSubset) string {
				if subset.Addresses != nil && len(subset.Addresses) > 0 {
					return subset.Addresses[0].IP
				}
				if subset.NotReadyAddresses != nil && len(subset.NotReadyAddresses) > 0 {
					return subset.NotReadyAddresses[0].IP
				}
				return ""
			}(subset)
			if strings.TrimSpace(ip) == "" {
				logrus.Debugf("Key: %s; Index: %d; IP not found", fmt.Sprintf("%s/%s", ep.Namespace, ep.Name), idx)
				continue
			}
			exists[subset.Ports[0].Name] = true
			pbep := &pb.ThirdPartyEndpoint{
				Uuid: subset.Ports[0].Name,
				Sid:  ep.GetLabels()["service_id"],
				Ip:   ip,
				Port: subset.Ports[0].Port,
				Status: func(item *corev1.Endpoints) string {
					if subset.Addresses != nil && len(subset.Addresses) > 0 {
						return "healthy"
					}
					if subset.NotReadyAddresses != nil && len(subset.NotReadyAddresses) > 0 {
						return "unhealthy"
					}
					return "unknown"
				}(ep),
			}
			pbeps = append(pbeps, pbep)
		}
	}
	return &pb.ThirdPartyEndpoints{
		Obj: pbeps,
	}, nil
}

// AddThirdPartyEndpoint creates a create event.
func (r *RuntimeServer) AddThirdPartyEndpoint(ctx context.Context, re *pb.AddThirdPartyEndpointsReq) (*pb.Empty, error) {
	as := r.store.GetAppService(re.Sid)
	if as == nil {
		return new(pb.Empty), nil
	}
	rbdep := &v1.RbdEndpoint{
		UUID: re.Uuid,
		Sid:  re.Sid,
		IP:   re.Ip,
		Port: int(re.Port),
	}
	r.updateCh.In() <- discovery.Event{
		Type: discovery.CreateEvent,
		Obj:  rbdep,
	}
	return new(pb.Empty), nil
}

// UpdThirdPartyEndpoint creates a update event.
func (r *RuntimeServer) UpdThirdPartyEndpoint(ctx context.Context, re *pb.UpdThirdPartyEndpointsReq) (*pb.Empty, error) {
	as := r.store.GetAppService(re.Sid)
	if as == nil {
		return new(pb.Empty), nil
	}
	rbdep := &v1.RbdEndpoint{
		UUID:     re.Uuid,
		Sid:      re.Sid,
		IP:       re.Ip,
		Port:     int(re.Port),
		IsOnline: re.IsOnline,
	}
	if re.IsOnline == false {
		r.updateCh.In() <- discovery.Event{
			Type: discovery.DeleteEvent,
			Obj:  rbdep,
		}
	} else {
		r.updateCh.In() <- discovery.Event{
			Type: discovery.UpdateEvent,
			Obj:  rbdep,
		}
	}
	return new(pb.Empty), nil
}

// DelThirdPartyEndpoint creates a delete event.
func (r *RuntimeServer) DelThirdPartyEndpoint(ctx context.Context, re *pb.DelThirdPartyEndpointsReq) (*pb.Empty, error) {
	as := r.store.GetAppService(re.Sid)
	if as == nil {
		return new(pb.Empty), nil
	}
	r.updateCh.In() <- discovery.Event{
		Type: discovery.DeleteEvent,
		Obj: &v1.RbdEndpoint{
			UUID: re.Uuid,
			Sid:  re.Sid,
		},
	}
	return new(pb.Empty), nil
}

// ListPodsBySID lists information of pods based on the given service id.
func (r *RuntimeServer) ListPodsBySID(ctx context.Context, in *pb.ListPodsBySIDReq) (*pb.Empty, error) {
	return new(pb.Empty), nil
}
