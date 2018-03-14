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

package version1

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/goodrain/rainbond/pkg/api/apiFunc"
	"github.com/goodrain/rainbond/pkg/api/controller"
	api_db "github.com/goodrain/rainbond/pkg/api/db"
	"github.com/goodrain/rainbond/pkg/api/handler"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
	"github.com/goodrain/rainbond/pkg/worker/discover/model"

	"github.com/jinzhu/gorm"

	"k8s.io/client-go/kubernetes"

	context "golang.org/x/net/context"

	"github.com/goodrain/rainbond/pkg/mq/api/grpc/pb"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/pquerna/ffjson/ffjson"
)

type key string

//V1Routes v1Routes
type V1Routes struct {
	ServiceStruct *ServiceStruct
	APIFuncV1     apiFunc.TenantInterfaceWithV1
}

//Show test show
func Show(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("v1 urls"))
}

//UnFoundRequest UnFoundRequest
func (v1 *V1Routes) UnFoundRequest(w http.ResponseWriter, r *http.Request) {
	apis := v1.ServiceStruct.V1API
	/*
		if err != nil {
			logrus.Errorf("get v1 api url error. %v", err)
			return
		}
	*/
	logrus.Debugf("url not Found then reword to v1, %s%s", apis[0], r.URL)
	controller.HTTPRequest(w, r, apis)
}

//ServiceStruct service struct
type ServiceStruct struct {
	V1API      string
	MQClient   pb.TaskQueueClient
	KubeClient *kubernetes.Clientset
}

//StartService start service
func (s *ServiceStruct) StartService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in start service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		w.Write([]byte("trans start service\n"))
		/*
			services, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
			if err != nil {
				logrus.Errorf("get service by id error, %v", err)
				return
			}
		*/
		startTask := model.StartTaskBody{
			TenantID:      "1b5678rtyuityuirtyu5674567856",
			ServiceID:     "asdkfakhdfopadpfapdf",
			DeployVersion: "201708231819",
			EventID:       "5678956",
		}
		smp, errS := controller.TransBody(r)
		if errS != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		logrus.Debugf("event_id is %v", smp["event_id"].(string))
		ts := &api_db.TaskStruct{
			TaskType: "start",
			TaskBody: startTask,
			User:     "testUser",
		}
		eq, errEq := api_db.BuildTask(ts)
		if errEq != nil {
			logrus.Errorf("build equeue request error, %v", errEq)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500, `"msg":"start falied"`))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, err := s.MQClient.Enqueue(ctx, eq)
		cancel()
		if err != nil {
			logrus.Errorf("equque mq error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200))
		logrus.Debugf("equeue mq start task success")

	} else {
		logrus.Debugf("no labels proxy to v1 start")
		logrus.Debugf("in start services api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

//StopService stop service
func (s *ServiceStruct) StopService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in stop service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("trans stop service")
		services, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			logrus.Errorf("get service by id error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		smp, errS := controller.TransBody(r)
		if errS != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		logrus.Debugf("event_id is %v", smp["event_id"].(string))
		stopTask := model.StopTaskBody{
			TenantID:      services.TenantID,
			ServiceID:     services.ServiceID,
			DeployVersion: services.DeployVersion,
			EventID:       smp["event_id"].(string),
		}
		ts := &api_db.TaskStruct{
			TaskType: "stop",
			TaskBody: stopTask,
			User:     "define",
		}
		eq, errEq := api_db.BuildTask(ts)
		if errEq != nil {
			logrus.Errorf("build equeue stop request error, %v", errEq)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, err = s.MQClient.Enqueue(ctx, eq)
		cancel()
		if err != nil {
			logrus.Errorf("equque mq error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200))
		logrus.Debugf("equeue mq stop task success")
	} else {
		logrus.Debugf("no labels proxy to v1 stop")
		logrus.Debugf("in stop services api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
		db.GetManager().TenantServiceLabelDao().AddModel(&dbmodel.TenantServiceLable{
			ServiceID:  serviceID,
			LabelKey:   "service_type",
			LabelValue: "unknow",
		})
	}
}

//RestartService restart service
func (s *ServiceStruct) RestartService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in restart service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("trans restart service")
		services, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			logrus.Errorf("get service by id error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		smp, errS := controller.TransBody(r)
		if errS != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		restartTask := model.RestartTaskBody{
			TenantID:      services.TenantID,
			ServiceID:     services.ServiceID,
			DeployVersion: services.DeployVersion,
			EventID:       smp["event_id"].(string),
		}
		ts := &api_db.TaskStruct{
			TaskType: "start",
			TaskBody: restartTask,
			User:     "define",
		}
		eq, errEq := api_db.BuildTask(ts)
		if errEq != nil {
			logrus.Errorf("build equeue restart request error, %v", errEq)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, err = s.MQClient.Enqueue(ctx, eq)
		cancel()
		if err != nil {
			logrus.Errorf("equque mq error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200))
		logrus.Debugf("equeue mq restart task success")
	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in restart services api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

//VerticalService vertical service
func (s *ServiceStruct) VerticalService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in vertical service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("trans vertical service")
		services, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			logrus.Errorf("get service by id error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		vmp, errV := controller.TransBody(r)
		if errV != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		cpu := int(vmp["container_cpu"].(float64))
		mem := int(vmp["container_memory"].(float64))
		verticalTask := model.VerticalScalingTaskBody{
			TenantID:        services.TenantID,
			ServiceID:       services.ServiceID,
			EventID:         vmp["event_id"].(string),
			ContainerCPU:    cpu,
			ContainerMemory: mem,
		}
		ts := &api_db.TaskStruct{
			TaskType: "vertical_scaling",
			TaskBody: verticalTask,
			User:     "define",
		}
		eq, errEq := api_db.BuildTask(ts)
		if errEq != nil {
			logrus.Errorf("build equeue vertical request error, %v", errEq)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, err = s.MQClient.Enqueue(ctx, eq)
		cancel()
		if err != nil {
			logrus.Errorf("equque mq error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		logrus.Debugf("equeue mq vertical task success")
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200))
	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in vertical services api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

//HorizontalService horizontal service
func (s *ServiceStruct) HorizontalService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in horizontal service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("trans horizontal service")
		services, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			logrus.Errorf("get service by id error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		hmp, errH := controller.TransBody(r)
		if errH != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		replicas := int32(hmp["node_num"].(float64))
		horizontalTask := model.HorizontalScalingTaskBody{
			TenantID:  services.TenantID,
			ServiceID: services.ServiceID,
			EventID:   hmp["event_id"].(string),
			Replicas:  int32(replicas),
		}
		ts := &api_db.TaskStruct{
			TaskType: "horizontal_scaling",
			TaskBody: horizontalTask,
			User:     "define",
		}
		eq, errEq := api_db.BuildTask(ts)
		if errEq != nil {
			logrus.Errorf("build equeue horizontal request error, %v", errEq)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, err = s.MQClient.Enqueue(ctx, eq)
		cancel()
		if err != nil {
			logrus.Errorf("equque mq error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200))
		logrus.Debugf("equeue mq horizontal task success")
	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in horizontal services api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

//BuildService build service
func (s *ServiceStruct) BuildService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in build service serviceID is %v", serviceID)

	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("from v1 trans to v2 build service")
		serviceInfo, err := s.getServiceInfo(serviceID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				w.WriteHeader(404)
				w.Write(controller.RestInfo(500, fmt.Sprintf("have no service resource, %v", err)))
				return
			}
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500, fmt.Sprintf("query service info error, %v", err)))
			return
		}
		tenantInfo, errT := s.getTenantInfo(serviceInfo.TenantID)
		if errT != nil {
			if err == gorm.ErrRecordNotFound {
				w.WriteHeader(404)
				w.Write(controller.RestInfo(500, fmt.Sprintf("have no tenant resource, %v", err)))
				return
			}
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500, fmt.Sprintf("query tenant info error, %v", err)))
			return
		}
		var v1Build api_model.V1BuildServiceStruct
		ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &v1Build.Body, nil)
		if !ok {
			return
		}
		envs := make(map[string]string)
		if err := ffjson.Unmarshal([]byte(v1Build.Body.ENVS), &envs); err != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500, fmt.Sprintf("unformat env json error, %v", err)))
			return
		}
		build := api_model.BuildServiceStruct{}
		build.Body.EventID = v1Build.Body.EventID
		build.Body.ENVS = envs
		build.Body.Kind = "source"
		build.Body.Action = v1Build.Body.Action
		build.Body.DeployVersion = v1Build.Body.DeployVersion
		build.Body.Operator = v1Build.Body.Operator
		build.Body.RepoURL = v1Build.Body.GitURL
		build.Body.TenantName = tenantInfo.Name
		build.Body.ServiceAlias = serviceInfo.ServiceAlias
		logrus.Debugf("build body is %v", build.Body)
		if err := handler.GetServiceManager().ServiceBuild(serviceInfo.TenantID, serviceID, &build); err != nil {
			logrus.Debugf("build service error")
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500, fmt.Sprintf("build service error, %v", err)))
			return
		}
		logrus.Debugf("equeue mq build task success")
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200, `"status": "success"`))
	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in build services api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
		db.GetManager().TenantServiceLabelDao().AddModel(&dbmodel.TenantServiceLable{
			ServiceID:  serviceID,
			LabelKey:   "service_type",
			LabelValue: "unknow",
		})
	}
}

//DeployService deploy service
func (s *ServiceStruct) DeployService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in deploy service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("trans deploy service")
		services, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			logrus.Errorf("get service by id error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		smp, errS := controller.TransBody(r)
		if errS != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		services.DeployVersion = smp["deploy_version"].(string)
		if err := db.GetManager().TenantServiceDao().UpdateModel(services); err != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		restartTask := model.RestartTaskBody{
			TenantID:      services.TenantID,
			ServiceID:     services.ServiceID,
			DeployVersion: services.DeployVersion,
			EventID:       smp["event_id"].(string),
		}
		ts := &api_db.TaskStruct{
			TaskType: "start",
			TaskBody: restartTask,
			User:     "define",
		}
		eq, errEq := api_db.BuildTask(ts)
		if errEq != nil {
			logrus.Errorf("build equeue restart request error, %v", errEq)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, err = s.MQClient.Enqueue(ctx, eq)
		cancel()
		if err != nil {
			logrus.Errorf("equque mq error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200))
		logrus.Debugf("equeue mq restart task success")
	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in deploy services api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

//UpgradeService upgrade service
func (s *ServiceStruct) UpgradeService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in upgrade service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("trans to v1 upgrade service")
		services, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			logrus.Errorf("get service by id error, %v, %v", services, err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		Ump, errU := controller.TransBody(r)
		if errU != nil {
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		//两��deploy version
		upgradeTask := model.RollingUpgradeTaskBody{
			TenantID:             services.TenantID,
			ServiceID:            services.ServiceID,
			CurrentDeployVersion: services.DeployVersion,
			NewDeployVersion:     Ump["deploy_version"].(string),
			EventID:              Ump["event_id"].(string),
		}
		ts := &api_db.TaskStruct{
			TaskType: "rolling_upgrade",
			TaskBody: upgradeTask,
			User:     "define",
		}
		eq, errEq := api_db.BuildTask(ts)
		if errEq != nil {
			logrus.Errorf("build equeue upgrade request error, %v", errEq)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, err = s.MQClient.Enqueue(ctx, eq)
		cancel()
		if err != nil {
			logrus.Errorf("equque mq error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200))
		logrus.Debugf("equeue mq upgrade task success")
	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in upgrade services api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

//StatusService status service
func (s *ServiceStruct) StatusService(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.getStatusService(w, r)
	case "POST":
		s.postStatusService(w, r)
	default:
		w.WriteHeader(415)
	}
}

type podsList struct {
	PodIP    string `json:"pod_ip"`
	Phase    string `json:"phase"`
	PodName  string `json:"pod_name"`
	NodeName string `json:"node_name"`
}

type statusList struct {
	TenantID      string     `json:"tenant_id"`
	ServiceID     string     `json:"service_id"`
	ServiceAlias  string     `json:"service_alias"`
	DeployVersion string     `json:"deploy_version"`
	Replicas      int        `json:"replicas"`
	ContainerMem  int        `json:"container_memory"`
	CurStatus     string     `json:"cur_status"`
	ContainerCPU  int        `json:"container_cpu"`
	StatusCN      string     `json:"status_cn"`
	PodList       []podsList `json:"pod_list"`
}

func (s *ServiceStruct) getStatusService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in status service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("trans status service")
		servicesStatus, err := db.GetManager().TenantServiceStatusDao().GetTenantServiceStatus(serviceID)
		if err != nil {
			logrus.Errorf("get service status by id error, %v, %v", serviceID, err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		services, errS := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		if errS != nil {
			logrus.Errorf("get service by id error, %v, %v", serviceID, err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		sl := &statusList{
			TenantID:      services.TenantID,
			ServiceID:     serviceID,
			ServiceAlias:  services.ServiceAlias,
			DeployVersion: services.DeployVersion,
			Replicas:      services.Replicas,
			ContainerMem:  services.ContainerMemory,
			ContainerCPU:  services.ContainerCPU,
			CurStatus:     servicesStatus.Status,
			StatusCN:      controller.TransStatus(servicesStatus.Status),
		}
		pods, errP := controller.GetPodList(services.TenantID, services.ServiceAlias, s.KubeClient)
		if errP != nil {
			logrus.Errorf("get pods list by id error, %v, %v", serviceID, err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		if len(pods.Items) > 0 {
			for _, pod := range pods.Items {
				pl := podsList{
					PodIP:    pod.Status.PodIP,
					Phase:    string(pod.Status.Phase),
					PodName:  pod.Name,
					NodeName: pod.Spec.NodeName,
				}
				sl.PodList = append(sl.PodList, pl)
			}
		}
		logrus.Debugf("podMap is %v", sl.PodList)
		jsonsl, errJ := ffjson.Marshal(sl)
		if errJ != nil {
			logrus.Errorf("trans status list to json error, %v", errJ)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		w.WriteHeader(200)
		w.Write(jsonsl)
	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in status api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

func (s *ServiceStruct) postStatusService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	logrus.Debugf("in status service serviceID is %v", serviceID)
	label := s.checkLabel(serviceID)
	if label {
		logrus.Debugf("trans to v1 status service")
		servicesStatus, err := db.GetManager().TenantServiceStatusDao().GetTenantServiceStatus(serviceID)
		if err != nil {
			logrus.Errorf("post service status by id error, %v, %v", serviceID, err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500))
			return
		}
		msgStatus := servicesStatus.Status
		logrus.Debugf("service status is %v", msgStatus)
		w.WriteHeader(200)
		w.Write(controller.RestInfo(200, fmt.Sprintf(`"status": "%v"`, msgStatus), fmt.Sprintf(`"status_cn":"%v"`, controller.TransStatus(msgStatus))))

	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in status api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

//StatusServiceList service list status
func (s *ServiceStruct) StatusServiceList(w http.ResponseWriter, r *http.Request) {
	Smp, errU := controller.TransBody(r)
	if errU != nil {
		w.WriteHeader(500)
		w.Write(controller.RestInfo(500))
		return
	}
	serviceIDs := Smp["service_ids"].(string)
	//logrus.Debugf("service ids is %v", serviceIDs)
	serviceList := strings.Split(serviceIDs, ",")
	if len(serviceList) == 0 {
		w.WriteHeader(500)
		w.Write(controller.RestInfo(500, `"msg":"service list is empty"`))
	}
	//TODO: 优化数据库查询
	mapStatus := make(map[string]interface{})
	var oldList string
	for _, serviceID := range serviceList {
		if s.checkLabel(serviceID) {
			servicesStatus, err := db.GetManager().TenantServiceStatusDao().GetTenantServiceStatus(serviceID)
			if err != nil {
				logrus.Errorf("get service status by id error, %v, %v", serviceID, err)
				mapStatus[serviceID] = map[string]string{"status": "failure", "status_cn": controller.TransStatus("failure")}
			}
			msgStatus := servicesStatus.Status
			mapStatus[serviceID] = map[string]string{"status": msgStatus, "status_cn": controller.TransStatus(msgStatus)}
			logrus.Debugf("service in v2 status is %v", msgStatus)
		} else {
			if oldList == "" {
				oldList = serviceID
			} else {
				oldList += "," + serviceID
			}
		}
	}
	//logrus.Debugf("to v1 service_ids is %v", oldList)
	oldList = fmt.Sprintf(`{"service_ids":"%s"}`, oldList)
	//r.Body = bytes.NewBuffer([]bytes(oldList))
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s%s", s.V1API, r.URL), bytes.NewBufferString(oldList))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token 5ca196801173be06c7e6ce41d5f7b3b8071e680a")
	client := &http.Client{
		Timeout: time.Second * 3,
	}
	response, errR := client.Do(req)

	//response, errR := s.httpRequest(w, r, fmt.Sprintf("http://%s%s", s.V1API, r.URL))
	if errR != nil {
		w.WriteHeader(500)
		w.Write(controller.RestInfo(500, `"msg":"get v1 servicelist failure"`))
		return
	}
	body, _ := ioutil.ReadAll(response.Body)
	//logrus.Debugf("get service status list through v1 api, body is %v", string(body))
	var mapV1 map[string]interface{}
	err = ffjson.Unmarshal(body, &mapV1)
	if err != nil {
		logrus.Errorf("json unmarshal error, %v", err)
		w.WriteHeader(500)
		w.Write(controller.RestInfo(500, `"msg":"get v1 servicelist failure"`))
		return
	}
	if len(mapV1) != 0 {
		//logrus.Debugf("get service status list map is %v", mapV1)
		for k, v := range mapV1 {
			//logrus.Debugf("get v1 services %v, %v", v.(string), controller.TransStatus(v.(string)))
			mapStatus[k] = map[string]string{"status": v.(string), "status_cn": controller.TransStatus(v.(string))}
		}
	}
	mjson, errM := ffjson.Marshal(mapStatus)
	if errM != nil {
		w.WriteHeader(500)
		w.Write(controller.RestInfo(500, `"msg":"get servicelist failure"`))
	}
	//logrus.Debugf("new mmp status is %v", mjson)
	w.WriteHeader(200)
	w.Write(mjson)
}

//StatusContainerID StatusContainerID
func (s *ServiceStruct) StatusContainerID(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	label := s.checkLabel(serviceID)
	if label {
		// services, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		// ipMap, errI := v2.ListPodIP(services.TenantID, services.ServiceAlias, s.KubeClient)
		// if errI != nil {
		// 	w.WriteHeader(500)
		// 	w.Write(v2.RestInfo(500))
		// 	return
		// }
		pods, err := db.GetManager().K8sPodDao().GetPodByService(serviceID)
		if err != nil {
			logrus.Errorf("find service pod info error, %v", err)
			w.WriteHeader(500)
			w.Write(controller.RestInfo(500, `"msg":"find service pod info error"`))
		}
		if pods == nil || len(pods) == 0 {
			w.WriteHeader(404)
			w.Write(controller.RestInfo(404, `"msg":"service pod info not found"`))
			return
		}
		jMap := `{`
		for _, pod := range pods {
			jMap += fmt.Sprintf(`"%s":"%s",`, pod.PodName, "manager")
		}
		jMap = jMap[0:len(jMap)-1] + "}"
		w.WriteHeader(200)
		w.Write([]byte(jMap))
	} else {
		logrus.Debugf("no labels proxy to v1 restart")
		logrus.Debugf("in status api to v1, %s%s", s.V1API, r.URL)
		controller.HTTPRequest(w, r, s.V1API)
	}
}

//ReMultiAPI return multi apis format []
func (s *ServiceStruct) ReMultiAPI(r *http.Request) (api []string, err error) {
	type key string
	//TODO:增加校验
	V1API := r.Context().Value(key("v1_api"))
	logrus.Debugf("v1_api is %s", V1API)
	apis := strings.Split(V1API.(string), ",")
	logrus.Debugf("apis %v", apis)
	return apis, nil
}

func (s *ServiceStruct) httpRequest(w http.ResponseWriter, r *http.Request, api string) (*http.Response, error) {
	client := &http.Client{}
	request, _ := http.NewRequest("POST", api, r.Body)
	logrus.Debugf("http %v header is %v", api, r.Header)
	for key, value := range r.Header {
		request.Header.Set(key, value[0])
	}
	resp, err := client.Do(request)
	if err != nil {
		logrus.Errorf("get status request error, %v", err)
		return resp, err
	}
	return resp, nil
}

func (s *ServiceStruct) checkLabel(serviceID string) bool {
	//true for v2, false for v1
	serviceLabel, err := db.GetManager().TenantServiceLabelDao().GetTenantServiceLabel(serviceID)
	if err != nil {
		return false
	}
	if serviceLabel != nil && len(serviceLabel) > 0 {
		return true
	}
	return false
}

func (s *ServiceStruct) getServiceInfo(serviceID string) (*dbmodel.TenantServices, error) {
	sInfo, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, err
	}
	return sInfo, nil
}

func (s *ServiceStruct) getTenantInfo(tenantID string) (*dbmodel.Tenants, error) {
	tInfo, err := db.GetManager().TenantDao().GetTenantByUUID(tenantID)
	if err != nil {
		return nil, err
	}
	return tInfo, nil
}
