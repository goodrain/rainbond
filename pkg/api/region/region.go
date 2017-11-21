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

package region

import (
	"net/http"
	"errors"
	"io/ioutil"
	"encoding/json"
	"github.com/goodrain/rainbond/pkg/api/model"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	"bytes"
	"github.com/Sirupsen/logrus"
	//api "github.com/goodrain/rainbond/pkg/util/http"
	//"github.com/bitly/go-simplejson"
	"github.com/bitly/go-simplejson"
)

var regionAPI, token string
var region *Region


type Region struct {
	regionAPI string
	token string
	authType string
}
func (r *Region)Tenants() TenantInterface {
	return &Tenant{}
}

type Tenant struct {
	tenantID string
}
type Services struct {
	tenant *Tenant
	model model.ServiceStruct
}
type TenantInterface interface {
	Get(name string) *Tenant
	Services() ServiceInterface
}
func (t *Tenant)Get(name string) *Tenant {
	return &Tenant{
		tenantID:name,
	}
}
func (t *Tenant)Delete(name string) error {
	return nil
}
func (t *Tenant)Services() ServiceInterface {
	return &Services{
		tenant:t,
	}
}

type ServiceInterface interface {
	Get(name string) map[string]string
	Pods(serviceAlisa string) ([]*dbmodel.K8sPod,error)
	List() []model.ServiceStruct
	Stop(serviceAlisa ,eventID string) error
	Start(serviceAlisa ,eventID string) error
	EventLog(serviceAlisa,eventID,level string) ([]model.MessageData,error)
}

func (s *Services)Pods(serviceAlisa string) ([]*dbmodel.K8sPod,error) {
	resp,_,err:=DoRequest("/v2/tenants/"+s.tenant.tenantID+"/services/"+serviceAlisa+"/pods","GET",nil)
	if err != nil {
		logrus.Errorf("获取pods失败，details %s",err.Error())
		return nil,err
	}

	j,_:=simplejson.NewJson(resp)
	arr,err:=j.Get("list").Array()

	jsonA,_:=json.Marshal(arr)
	pods:=[]*dbmodel.K8sPod{}
	json.Unmarshal(jsonA,&pods)

	return pods,err
}
func (s *Services)Get(name string) map[string]string {
	resp,status,err:=DoRequest("/v2/tenants/"+s.tenant.tenantID+"/services/"+name,"GET",nil)
	if err != nil {
		logrus.Errorf("获取服务失败，details %s",err.Error())
		return nil
	}
	if status==404 {
		return nil
	}
	j,_:=simplejson.NewJson(resp)
	m:=make(map[string]string)
	bean:=j.Get("bean")
	sa,err:=bean.Get("serviceAlias").String()
	si,err:=bean.Get("serviceId").String()
	ti,err:=bean.Get("tenantId").String()
	tn,err:=bean.Get("tenantName").String()
	m["serviceAlias"]=sa
	m["serviceId"]=si
	m["tenantId"]=ti
	m["tenantName"]=tn

	return m
}
func (s *Services)EventLog(serviceAlisa,eventID,level string) ([]model.MessageData,error) {
	//{
	//	"event_id": "string",
	//	"level": "string"
	//}
	data := []byte(`{"event_id":"` + eventID + `","level":"`+level+`"}`)
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/event-log v2 logByAction
	resp,_,err:=DoRequest("/v2/tenants/"+s.tenant.tenantID+"/services/"+serviceAlisa+"/event-log","POST",data)
	//logrus.Infof("event log url is %s","/v2/tenants/"+s.tenant.tenantID+"/services/"+serviceAlisa+"/event-log")
	if err!=nil {
		return nil,err
	}


	dlj,err:=simplejson.NewJson(resp)
	arr,_:=dlj.Get("list").Array()

	a,_:=json.Marshal(arr)
	ss:=[]model.MessageData{}
	json.Unmarshal(a,&ss)

	return ss,nil
}

type listServices struct {
	list []model.ServiceStruct `json:"list"`
}
type beanDataLog struct {
	bean model.DataLog `json:"bean"`
}
type beanServiceStruct struct {
	bean model.ServiceStruct `json:"bean"`
}
func (s *Services)List() []model.ServiceStruct {

	request, err := http.NewRequest("GET", region.regionAPI+"/v2/tenants/"+s.tenant.tenantID+"/services", nil)
	if err != nil {
		//return err
		logrus.Errorf("error create request for region server,details %s",err.Error())
		return nil
	}
	request.Header.Set("Content-Type", "application/json")
	if region.token != "" {
		request.Header.Set("Authorization", "Token "+region.token)
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		//return nil, err
		logrus.Errorf("error connect to region server,details %s",err.Error())
		return nil
	}
	//de := json.NewDecoder(res.Body)

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		//return nil, err
		logrus.Errorf("error read response region server,details %s",err.Error())
		return nil
	}
	j,_:=simplejson.NewJson(data)
	arr,_:=j.Get("list").Array()
	b2,_:=json.Marshal(arr)

	ss:=[]model.ServiceStruct{}

	json.Unmarshal(b2,&ss)

	defer res.Body.Close()
	return ss

}
func (s *Services)Stop(name ,eventID string) error {

	data := []byte(`{"event_id":"` + eventID + `"}`)
	_,_,err:=DoRequest("/v2/tenants/"+s.tenant.tenantID+"/services/"+name+"/stop","POST",data)
	//request, err := http.NewRequest("POST", regionAPI+"/v2/tenants/"+s.tenant.tenantID+"/services"+s.model.ServiceAlias+"/stop", bytes.NewBuffer(data))
	if err!=nil {
		return err
	}

	return nil
}
func (s *Services)Start(name ,eventID string) error {

	data := []byte(`{"event_id":"` + eventID + `"}`)
	_,_,err:=DoRequest("/v2/tenants/"+s.tenant.tenantID+"/services/"+name+"/start","POST",data)
	//request, err := http.NewRequest("POST", regionAPI+"/v2/tenants/"+s.tenant.tenantID+"/services"+s.model.ServiceAlias+"/stop", bytes.NewBuffer(data))
	if err!=nil {
		return err
	}

	return nil
}

func DoRequest(url ,method string, body []byte) ([]byte,int,error) {
	request, err := http.NewRequest(method, region.regionAPI+url, bytes.NewBuffer(body))
	if err != nil {
		return nil,500,err
	}
	request.Header.Set("Content-Type", "application/json")
	if region.token != "" {
		request.Header.Set("Authorization", "Token "+region.token)
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, 500,err
	}

	data, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	return data,res.StatusCode,err
}
func NewRegion(regionAPI,token,authType string) *Region {
	if region==nil {
		region=&Region{
			regionAPI:regionAPI,
			token:token,
			authType:authType,
		}
	}
	return region
}
func GetRegion() *Region {
	return region
}




func LoadConfig(regionAPI, token string) (map[string]map[string]interface{}, error) {
	if regionAPI == "" {
		return nil, errors.New("region api url can not be empty")
	}
	//todo
	request, err := http.NewRequest("GET", regionAPI+"/v1/config", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Token "+token)
	}
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	config := make(map[string]map[string]interface{})
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, err
	}
	//{"k8s":{"url":"http://10.0.55.72:8181/api/v1","apitype":"kubernetes api"},
	//  "db":{"ENGINE":"django.db.backends.mysql",
	//               "AUTOCOMMIT":true,"ATOMIC_REQUESTS":false,"NAME":"region","CONN_MAX_AGE":0,
	//"TIME_ZONE":"Asia/Shanghai","OPTIONS":{},
	// "HOST":"10.0.55.72","USER":"writer1",
	// "TEST":{"COLLATION":null,"CHARSET":null,"NAME":null,"MIRROR":null},
	// "PASSWORD":"CeRYK8UzWD","PORT":"3306"}}
	return config, nil
}

//SetInfo 设置
func SetInfo(region, t string) {
	regionAPI = region
	token = t
}

