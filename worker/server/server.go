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
	discover "github.com/goodrain/rainbond/discover.v2"
	"github.com/goodrain/rainbond/util"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	wutil "github.com/goodrain/rainbond/worker/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/kubernetes"
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
	logrus.Infof("runtime server start success")
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
	runningApps := r.store.GetTenantRunningApp(re.TenantId)
	for _, app := range runningApps {
		if app.ServiceKind == model.ServiceKindThirdParty {
			tr.RunningAppThirdNum++
		} else if app.ServiceKind == model.ServiceKindInternal {
			tr.RunningAppInternalNum++
		}
	}
	tr.RunningAppNum = int64(len(runningApps))
	tr.CpuLimit = res.CPULimit
	tr.CpuRequest = res.CPURequest
	tr.MemoryLimit = res.MemoryLimit / 1024 / 1024
	tr.MemoryRequest = res.MemoryRequest / 1024 / 1024
	return &tr, nil
}

//GetTenantResources get tenant resources
func (r *RuntimeServer) GetTenantResources(context.Context, *pb.Empty) (*pb.TenantResourceList, error) {
	res := r.store.GetTenantResourceList()
	var trs = make(map[string]*pb.TenantResource)
	for _, re := range res {
		var tr pb.TenantResource
		runningApps := r.store.GetTenantRunningApp(re.Namespace)
		for _, app := range runningApps {
			if app.ServiceKind == model.ServiceKindThirdParty {
				tr.RunningAppThirdNum++
			} else if app.ServiceKind == model.ServiceKindInternal {
				tr.RunningAppInternalNum++
			}
		}
		tr.RunningAppNum = int64(len(runningApps))
		tr.CpuLimit = re.CPULimit
		tr.CpuRequest = re.CPURequest
		tr.MemoryLimit = re.MemoryLimit / 1024 / 1024
		tr.MemoryRequest = re.MemoryRequest / 1024 / 1024
		trs[re.Namespace] = &tr
	}
	return &pb.TenantResourceList{Resources: trs}, nil
}

//GetAppPods get app pod list
func (r *RuntimeServer) GetAppPods(ctx context.Context, re *pb.ServiceRequest) (*pb.ServiceAppPodList, error) {
	app := r.store.GetAppService(re.ServiceId)
	if app == nil {
		return nil, ErrAppServiceNotFound
	}

	pods := app.GetPods(false)
	var oldpods, newpods []*pb.ServiceAppPod
	for _, pod := range pods {
		if v1.IsPodTerminated(pod) {
			continue
		}
		// Exception pod information due to node loss is no longer displayed
		if v1.IsPodNodeLost(pod) {
			continue
		}
		var containers = make(map[string]*pb.Container, len(pod.Spec.Containers))
		volumes := make([]string, 0)
		for _, container := range pod.Spec.Containers {
			containers[container.Name] = &pb.Container{
				ContainerName: container.Name,
				MemoryLimit:   container.Resources.Limits.Memory().Value(),
			}
			for _, vm := range container.VolumeMounts {
				volumes = append(volumes, vm.Name)
			}
		}

		sapod := &pb.ServiceAppPod{
			PodIp:      pod.Status.PodIP,
			PodName:    pod.Name,
			Containers: containers,
			PodVolumes: volumes,
		}
		podStatus := &pb.PodStatus{}
		wutil.DescribePodStatus(r.clientset, pod, podStatus, k8s.DefListEventsByPod)
		sapod.PodStatus = podStatus.Type.String()
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

//GetMultiAppPods get multi app pods
func (r *RuntimeServer) GetMultiAppPods(ctx context.Context, re *pb.ServicesRequest) (*pb.MultiServiceAppPodList, error) {
	serviceIDs := strings.Split(re.ServiceIds, ",")
	var res pb.MultiServiceAppPodList
	res.ServicePods = make(map[string]*pb.ServiceAppPodList, len(serviceIDs))
	for _, id := range serviceIDs {
		if len(id) != 0 {
			list, err := r.GetAppPods(ctx, &pb.ServiceRequest{ServiceId: id})
			if err != nil && err != ErrAppServiceNotFound {
				logrus.Errorf("get app %s pod list failure %s", id, err.Error())
				continue
			}
			res.ServicePods[id] = list
		}
	}
	return &res, nil
}

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
			interval = fmt.Sprintf("%s ago (x%d over %s)", translateTimestampSince(e.LastTimestamp), e.Count, translateTimestampSince(e.FirstTimestamp))
		} else {
			interval = translateTimestampSince(e.FirstTimestamp) + " ago"
		}
		podEvent := &pb.PodEvent{
			Type:    e.Type,
			Reason:  e.Reason,
			Age:     interval,
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
			deployinfo.StartTime = appService.GetStatefulSet().ObjectMeta.CreationTimestamp.Format(time.RFC3339)
		}
		if appService.GetDeployment() != nil {
			deployinfo.Deployment = appService.GetDeployment().Name
			deployinfo.StartTime = appService.GetDeployment().ObjectMeta.CreationTimestamp.Format(time.RFC3339)
		}
		if services := appService.GetServices(false); services != nil {
			service := make(map[string]string, len(services))
			for _, s := range services {
				service[s.Name] = s.Name
			}
			deployinfo.Services = service
		}
		if endpoints := appService.GetEndpoints(false); endpoints != nil &&
			appService.AppServiceBase.ServiceKind == model.ServiceKindThirdParty {
			eps := make(map[string]string, len(endpoints))
			for _, s := range endpoints {
				eps[s.Name] = s.Name
			}
			deployinfo.Endpoints = eps
		}
		if secrets := appService.GetSecrets(false); secrets != nil {
			secretsinfo := make(map[string]string, len(secrets))
			for _, s := range secrets {
				secretsinfo[s.Name] = s.Name
			}
			deployinfo.Secrets = secretsinfo
		}
		if ingresses := appService.GetIngress(false); ingresses != nil {
			ingress := make(map[string]string, len(ingresses))
			for _, s := range ingresses {
				ingress[s.Name] = s.Name
			}
			deployinfo.Ingresses = ingress
		}
		if pods := appService.GetPods(false); pods != nil {
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
		etcdClientArgs := &etcdutil.ClientArgs{
			Endpoints: r.conf.EtcdEndPoints,
			CaFile:    r.conf.EtcdCaFile,
			CertFile:  r.conf.EtcdCertFile,
			KeyFile:   r.conf.EtcdKeyFile,
		}
		keepalive, err := discover.CreateKeepAlive(etcdClientArgs, "app_sync_runtime_server", "", r.conf.HostIP, r.conf.ServerPort)
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
	addEndpoint := func(tpe *pb.ThirdPartyEndpoint) {
		if !exists[fmt.Sprintf("%s:%d", tpe.Ip, tpe.Port)] {
			pbeps = append(pbeps, tpe)
			exists[fmt.Sprintf("%s:%d", tpe.Ip, tpe.Port)] = true
		}
	}
	for _, ep := range as.GetEndpoints(false) {
		if ep.Subsets == nil || len(ep.Subsets) == 0 {
			logrus.Debugf("Key: %s; empty subsets", fmt.Sprintf("%s/%s", ep.Namespace, ep.Name))
			continue
		}
		for _, subset := range ep.Subsets {
			for _, port := range subset.Ports {
				for _, address := range subset.Addresses {
					ip := address.IP
					if ip == "1.1.1.1" {
						if len(as.GetServices(false)) > 0 {
							ip = as.GetServices(false)[0].Annotations["domain"]
						}
					}
					addEndpoint(&pb.ThirdPartyEndpoint{
						Uuid: port.Name,
						Sid:  ep.GetLabels()["service_id"],
						Ip:   ip,
						Port: port.Port,
						Status: func() string {
							return "healthy"
						}(),
					})
				}
				for _, address := range subset.NotReadyAddresses {
					ip := address.IP
					if ip == "1.1.1.1" {
						if len(as.GetServices(false)) > 0 {
							ip = as.GetServices(false)[0].Annotations["domain"]
						}
					}
					addEndpoint(&pb.ThirdPartyEndpoint{
						Uuid: port.Name,
						Sid:  ep.GetLabels()["service_id"],
						Ip:   ip,
						Port: port.Port,
						Status: func() string {
							return "unhealthy"
						}(),
					})
				}
			}
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
			IP:   re.Ip,
			Port: int(re.Port),
		},
	}
	return new(pb.Empty), nil
}

// GetStorageClasses get storageclass list
func (r *RuntimeServer) GetStorageClasses(ctx context.Context, re *pb.Empty) (*pb.StorageClasses, error) {
	storageclasses := new(pb.StorageClasses)
	// stes := r.store.GetStorageClasses()

	// if stes != nil {
	// 	for _, st := range stes {
	// 		var allowTopologies []*pb.TopologySelectorTerm
	// 		for _, topologySelectorTerm := range st.AllowedTopologies {
	// 			var expressions []*pb.TopologySelectorLabelRequirement
	// 			for _, value := range topologySelectorTerm.MatchLabelExpressions {
	// 				expressions = append(expressions, &pb.TopologySelectorLabelRequirement{Key: value.Key, Values: value.Values})
	// 			}
	// 			allowTopologies = append(allowTopologies, &pb.TopologySelectorTerm{MatchLabelExpressions: expressions})
	// 		}

	// 		var allowVolumeExpansion bool
	// 		if st.AllowVolumeExpansion == nil {
	// 			allowVolumeExpansion = false
	// 		} else {
	// 			allowVolumeExpansion = *st.AllowVolumeExpansion
	// 		}
	// 		storageclasses.List = append(storageclasses.List, &pb.StorageClassDetail{
	// 			Name:                 st.Name,
	// 			Provisioner:          st.Provisioner,
	// 			Parameters:           st.Parameters,
	// 			ReclaimPolicy:        st.ReclaimPolicy,
	// 			AllowVolumeExpansion: allowVolumeExpansion,
	// 			VolumeBindingMode:    st.VolumeBindingMode,
	// 			AllowedTopologies:    allowTopologies,
	// 		})
	// 	}
	// }
	return storageclasses, nil
}

// GetAppVolumeStatus get app volume status
func (r *RuntimeServer) GetAppVolumeStatus(ctx context.Context, re *pb.ServiceRequest) (*pb.ServiceVolumeStatusMessage, error) {
	ret := new(pb.ServiceVolumeStatusMessage)
	ret.Status = make(map[string]pb.ServiceVolumeStatus)
	as := r.store.GetAppService(re.ServiceId)
	if as == nil {
		return ret, nil
	}

	// get component all pods
	pods := as.GetPods(false)
	for _, pod := range pods {
		// if pod is terminated, volume status of pod is NOT_READY
		if v1.IsPodTerminated(pod) {
			continue
		}
		// Exception pod information due to node loss is no longer displayed, so volume status is NOT_READY
		if v1.IsPodNodeLost(pod) {
			continue
		}

		podStatus := &pb.PodStatus{}
		wutil.DescribePodStatus(r.clientset, pod, podStatus, k8s.DefListEventsByPod)

		for _, volume := range pod.Spec.Volumes {
			volumeName := volume.Name
			prefix := "manual" // all volume name start with manual but config file, volume name style: fmt.Sprintf("manual%d", TenantServiceVolume.ID)
			if strings.HasPrefix(volumeName, prefix) {
				volumeName = strings.TrimPrefix(volumeName, prefix)
				switch podStatus.Type {
				case pb.PodStatus_SCHEDULING:
					// pod can't bind volume
					ret.Status[volumeName] = pb.ServiceVolumeStatus_NOT_READY
				case pb.PodStatus_UNKNOWN:
					// pod status is unknown
					ret.Status[volumeName] = pb.ServiceVolumeStatus_NOT_READY
				case pb.PodStatus_INITIATING:
					// pod status is unknown
					ret.Status[volumeName] = pb.ServiceVolumeStatus_READY
					if pod.Status.Phase == corev1.PodPending {
						ret.Status[volumeName] = pb.ServiceVolumeStatus_NOT_READY
					}
				case pb.PodStatus_RUNNING, pb.PodStatus_ABNORMAL, pb.PodStatus_NOTREADY, pb.PodStatus_UNHEALTHY:
					// pod is running
					ret.Status[volumeName] = pb.ServiceVolumeStatus_READY
				}
			}
		}
	}

	return ret, nil
}
