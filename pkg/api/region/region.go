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

package region

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/bitly/go-simplejson"
	"github.com/goodrain/rainbond/pkg/api/model"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/api/util"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	utilhttp "github.com/goodrain/rainbond/pkg/util/http"
	"github.com/pquerna/ffjson/ffjson"
)

var regionAPI, token string
var region *Region

type Region struct {
	regionAPI string
	token     string
	authType  string
}

func (r *Region) Tenants() TenantInterface {
	return &tenant{prefix: "/tenants"}
}

type tenant struct {
	tenantID string
	prefix   string
}
type services struct {
	tenant *tenant
	prefix string
	model  model.ServiceStruct
}

//TenantInterface TenantInterface
type TenantInterface interface {
	Get(name string) *tenant
	Services() ServiceInterface
	DefineSources(ss *api_model.SourceSpec) DefineSourcesInterface
	DefineCloudAuth(gt *api_model.GetUserToken) DefineCloudAuthInterface
}

func (t *tenant) Get(name string) *tenant {
	t.tenantID = name
	return t
}
func (t *tenant) Delete(name string) error {
	return nil
}
func (t *tenant) Services() ServiceInterface {
	return &services{
		prefix: "services",
		tenant: t,
	}
}

//ServiceInterface ServiceInterface
type ServiceInterface interface {
	Get(name string) (map[string]string, *util.APIHandleError)
	Pods(serviceAlisa string) ([]*dbmodel.K8sPod, *util.APIHandleError)
	List() ([]*model.ServiceStruct, *util.APIHandleError)
	Stop(serviceAlisa, eventID string) *util.APIHandleError
	Start(serviceAlisa, eventID string) *util.APIHandleError
	EventLog(serviceAlisa, eventID, level string) ([]*model.MessageData, *util.APIHandleError)
}

func (s *services) Pods(serviceAlisa string) ([]*dbmodel.K8sPod, *util.APIHandleError) {
	body, code, err := request("/v2"+s.tenant.prefix+"/"+s.tenant.tenantID+"/"+s.prefix+"/"+serviceAlisa+"/pods", "GET", nil)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc []*dbmodel.K8sPod
	res.List = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.List.(*[]*dbmodel.K8sPod); ok {
		return *gc, nil
	}
	return nil, nil
}
func (s *services) Get(name string) (map[string]string, *util.APIHandleError) {
	body, code, err := request("/v2"+s.tenant.prefix+"/"+s.tenant.tenantID+"/"+s.prefix+"/"+name, "GET", nil)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get err with code %d", code))
	}
	j, err := simplejson.NewJson(body)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	m := make(map[string]string)
	bean := j.Get("bean")
	sa, err := bean.Get("serviceAlias").String()
	si, err := bean.Get("serviceId").String()
	ti, err := bean.Get("tenantId").String()
	tn, err := bean.Get("tenantName").String()
	m["serviceAlias"] = sa
	m["serviceId"] = si
	m["tenantId"] = ti
	m["tenantName"] = tn
	return m, nil
}
func (s *services) EventLog(serviceAlisa, eventID, level string) ([]*model.MessageData, *util.APIHandleError) {
	data := []byte(`{"event_id":"` + eventID + `","level":"` + level + `"}`)
	body, code, err := request("/v2"+s.tenant.prefix+"/"+s.tenant.tenantID+"/"+s.prefix+"/event-log", "POST", data)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc []*model.MessageData
	res.List = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.List.(*[]*model.MessageData); ok {
		return *gc, nil
	}
	return nil, nil
}

func (s *services) List() ([]*model.ServiceStruct, *util.APIHandleError) {
	body, code, err := request("/v2"+s.tenant.prefix+"/"+s.tenant.tenantID+"/"+s.prefix, "GET", nil)

	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get with code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc []*model.ServiceStruct
	res.List = &gc

	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.List.(*[]*model.ServiceStruct); ok {
		return *gc, nil
	}
	return nil, nil
}
func (s *services) Stop(name, eventID string) *util.APIHandleError {
	data := []byte(`{"event_id":"` + eventID + `"}`)
	_, code, err := request("/v2"+s.tenant.prefix+"/"+s.tenant.tenantID+"/"+s.prefix+"/"+name+"/stop", "POST", data)
	return handleErrAndCode(err, code)
}
func (s *services) Start(name, eventID string) *util.APIHandleError {
	data := []byte(`{"event_id":"` + eventID + `"}`)
	_, code, err := request("/v2"+s.tenant.prefix+"/"+s.tenant.tenantID+"/"+s.prefix+"/"+name+"/start", "POST", data)
	return handleErrAndCode(err, code)
}

func request(url, method string, body []byte) ([]byte, int, error) {
	logrus.Infof("req url is %s", region.regionAPI+url)
	request, err := http.NewRequest(method, region.regionAPI+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, 500, err
	}
	request.Header.Set("Content-Type", "application/json")
	if region.token != "" {
		request.Header.Set("Authorization", "Token "+region.token)
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, 500, err
	}

	data, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	return data, res.StatusCode, err
}
func NewRegion(regionAPI, token, authType string) *Region {
	if region == nil {
		region = &Region{
			regionAPI: regionAPI,
			token:     token,
			authType:  authType,
		}
	}
	return region
}
func GetRegion() *Region {
	return region
}

func LoadConfig(regionAPI, token string) (map[string]map[string]interface{}, error) {
	if regionAPI != "" {
		//return nil, errors.New("region api url can not be empty")
		//return nil, errors.New("region api url can not be empty")
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
	return nil, errors.New("wrong region api ")

}

//SetInfo 设置
func SetInfo(region, t string) {
	regionAPI = region
	token = t
}
func handleErrAndCode(err error, code int) *util.APIHandleError {
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return util.CreateAPIHandleError(code, fmt.Errorf("error with code %d", code))
	}
	return nil
}
