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

package handler

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/goodrain/rainbond/cmd/api/option"
	api_db "github.com/goodrain/rainbond/pkg/api/db"
	"github.com/goodrain/rainbond/pkg/mq/api/grpc/pb"

	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

//TenantAction tenant act
type TenantAction struct {
	MQClient   pb.TaskQueueClient
	KubeClient *kubernetes.Clientset
	//OpentsdbClient tsdbClient.Client
	OpentsdbAPI string
}

//CreateTenManager create Manger
func CreateTenManager(conf option.Config) (*TenantAction, error) {
	mq := api_db.MQManager{
		Endpoint: conf.MQAPI,
	}
	mqClient, errMQ := mq.NewMQManager()
	if errMQ != nil {
		logrus.Errorf("new MQ manager failed, %v", errMQ)
		return nil, errMQ
	}
	logrus.Debugf("mqclient is %v", mqClient)
	k8s := api_db.K8SManager{
		K8SConfig: conf.KubeConfig,
	}
	kubeClient, errK := k8s.NewKubeConnection()
	if errK != nil {
		logrus.Errorf("create kubeclient failed, %v", errK)
		return nil, errK
	}
	/*
		opentsdbManager := api_db.OpentsdbManager{
			Endpoint: conf.Opentsdb,
		}
		opentsdb, errO := opentsdbManager.NewOpentsdbManager()
		if errO != nil {
			logrus.Errorf("create opentsdbclient failed, %v", errO)
			return nil, errK
		}
	*/
	opentsdbAPI := fmt.Sprintf("%s/api", conf.Opentsdb)
	return &TenantAction{
		MQClient:   mqClient,
		KubeClient: kubeClient,
		//OpentsdbClient: opentsdb,
		OpentsdbAPI: opentsdbAPI,
	}, nil
}

//GetTenants get tenants
func (t *TenantAction) GetTenants() ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetALLTenants()
	if err != nil {
		return nil, err
	}
	return tenants, err
}


//StatsMemCPU StatsMemCPU
func (t *TenantAction) TotalMemCPU(services []*dbmodel.TenantServices) (*api_model.StatsInfo, error){
	cpus := 0
	mem := 0
	for _, service := range services {

		logrus.Debugf("service is %s, cpus is %v, mem is %v", service.ID, service.ContainerCPU, service.ContainerMemory)
		cpus += service.ContainerCPU
		mem += service.ContainerMemory
	}
	si := &api_model.StatsInfo{
		CPU: cpus,
		MEM: mem,
	}
	return si, nil
}

//StatsMemCPU StatsMemCPU
func (t *TenantAction) StatsMemCPU(services []*dbmodel.TenantServices) (*api_model.StatsInfo, error) {
	cpus := 0
	mem := 0
	for _, service := range services {
		status := service.CurStatus
		label := CheckLabel(service.ServiceID)
		if label {
			servicesStatus, err := db.GetManager().TenantServiceStatusDao().GetTenantServiceStatus(service.ServiceID)
			if err != nil {
				continue
			}
			status = servicesStatus.Status
		}
		logrus.Debugf("status is %s, cpus is %v, mem is %v", status, service.ContainerCPU, service.ContainerMemory)
		if status == "undeploy" || status == "closed" {
			continue
		}
		cpus += service.ContainerCPU
		mem += service.ContainerMemory
	}
	si := &api_model.StatsInfo{
		CPU: cpus,
		MEM: mem,
	}
	return si, nil
}

//QueryTsdb QueryTsdb
/*
func (t *TenantAction) QueryTsdb(md *api_model.MontiorData) (*tsdbClient.QueryResponse, error) {
	st2 := time.Now().Unix()
	queryParam := tsdbClient.QueryParam{
		Start:   md.Body.Start,
		End:     st2,
		Queries: md.Body.Queries,
	}
	queryResp, err := t.OpentsdbClient.Query(queryParam)
	if err != nil {
		logrus.Errorf("query tsdb error %v", err)
		return nil, err
	}
	return queryResp, nil
}
*/

//HTTPTsdb HTTPTsdb
func (t *TenantAction) HTTPTsdb(md *api_model.MontiorData) ([]byte, error) {
	uri := fmt.Sprintf("/query?start=%s&m=%s", md.Body.Start, md.Body.Queries)
	logrus.Debugf(fmt.Sprintf("uri is %v", uri))
	url := fmt.Sprintf("http://%s%s", t.OpentsdbAPI, uri)
	logrus.Debugf(fmt.Sprintf("url is %v", url))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: time.Second * 3,
	}
	response, errR := client.Do(req)
	if errR != nil {
		return nil, errR
	}
	body, errB := ioutil.ReadAll(response.Body)
	if errB != nil {
		return nil, errB
	}
	return body, nil
}

//GetTenantsResources GetTenantsResources
func (t *TenantAction) GetTenantsResources(tr *api_model.TenantResources) ([]*map[string]interface{}, error) {
	//返回全部资源
	return db.GetManager().TenantServiceDao().GetCPUAndMEM(tr.Body.TenantName)
}

//TenantsSum TenantsSum
func (t *TenantAction) TenantsSum() (int, error) {
	s, err := db.GetManager().TenantDao().GetALLTenants()
	if err != nil {
		return 0, err
	}
	return len(s), nil
}
