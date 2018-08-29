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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/cmd"
	dbmodel "github.com/goodrain/rainbond/db/model"
	coreutil "github.com/goodrain/rainbond/util"
	utilhttp "github.com/goodrain/rainbond/util/http"
)

var regionAPI, token string

var region Region

//AllTenant AllTenant
var AllTenant string

//Region region api
type Region interface {
	Tenants(name string) TenantInterface
	Resources() ResourcesInterface
	Tasks() TaskInterface
	Nodes() NodeInterface
	Cluster() ClusterInterface
	Configs() ConfigsInterface
	Version() string
	Monitor() MonitorInterface
	Notification() NotificationInterface
	DoRequest(path, method string, body io.Reader, decode *utilhttp.ResponseBody) (int, error)
}

//APIConf region api config
type APIConf struct {
	Endpoints []string `yaml:"endpoints"`
	Token     string   `yaml:"token"`
	AuthType  string   `yaml:"auth_type"`
	Cacert    string   `yaml:"client-ca-file"`
	Cert      string   `yaml:"tls-cert-file"`
	CertKey   string   `yaml:"tls-private-key-file"`
}

type serviceInfo struct {
	ServicesAlias string `json:"serviceAlias"`
	TenantName    string `json:"tenantName"`
	ServiceId     string `json:"serviceId"`
	TenantId      string `json:"tenantId"`
}

type podInfo struct {
	ServiceID string `json:"service_id"`
	//部署资源的ID ,例如rc ,deploment, statefulset
	ReplicationID   string                       `json:"rc_id"`
	ReplicationType string                       `json:"rc_type"`
	PodName         string                       `json:"pod_name"`
	PodIP           string                       `json:"pod_ip"`
	Container       map[string]map[string]string `json:"container"`
}

//NewRegion NewRegion
func NewRegion(c APIConf) (Region, error) {
	if region == nil {
		re := &regionImpl{
			APIConf: c,
		}
		if c.Cacert != "" && c.Cert != "" && c.CertKey != "" {
			pool := x509.NewCertPool()
			caCrt, err := ioutil.ReadFile(c.Cacert)
			if err != nil {
				logrus.Errorf("read ca file err: %s", err)
				return nil, err
			}
			pool.AppendCertsFromPEM(caCrt)

			cliCrt, err := tls.LoadX509KeyPair(c.Cert, c.CertKey)
			if err != nil {
				logrus.Errorf("Loadx509keypair err: %s", err)
				return nil, err
			}
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:      pool,
					Certificates: []tls.Certificate{cliCrt},
				},
			}
			re.Client = &http.Client{
				Transport: tr,
			}
		} else {
			re.Client = http.DefaultClient
		}
		region = re
	}
	return region, nil
}

//GetRegion GetRegion
func GetRegion() Region {
	return region
}

type regionImpl struct {
	APIConf
	Client *http.Client
}

//Tenants Tenants
func (r *regionImpl) Tenants(tenantName string) TenantInterface {
	return &tenant{prefix: path.Join("/v2/tenants", tenantName), tenantName: tenantName, regionImpl: *r}
}

//Version Version
func (r *regionImpl) Version() string {
	return cmd.GetVersion()
}

//Resources about resources
func (r *regionImpl) Resources() ResourcesInterface {
	return &resources{prefix: "/v2/resources", regionImpl: *r}
}

type tenant struct {
	regionImpl
	tenantName string
	prefix     string
}
type services struct {
	tenant
	prefix string
	model  model.ServiceStruct
}

//TenantInterface TenantInterface
type TenantInterface interface {
	Get() (*dbmodel.Tenants, *util.APIHandleError)
	List() ([]*dbmodel.Tenants, *util.APIHandleError)
	Delete() *util.APIHandleError
	Services(serviceAlias string) ServiceInterface
	// DefineSources(ss *api_model.SourceSpec) DefineSourcesInterface
	// DefineCloudAuth(gt *api_model.GetUserToken) DefineCloudAuthInterface
}

func (t *tenant) Get() (*dbmodel.Tenants, *util.APIHandleError) {
	var decode utilhttp.ResponseBody
	var tenant dbmodel.Tenants
	decode.Bean = &tenant
	code, err := t.DoRequest(t.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	return &tenant, nil
}
func (t *tenant) List() ([]*dbmodel.Tenants, *util.APIHandleError) {
	if t.tenantName != "" {
		return nil, util.CreateAPIHandleErrorf(400, "tenant name must be empty in this api")
	}
	var decode utilhttp.ResponseBody
	var tenants []*dbmodel.Tenants
	decode.List = &tenants
	code, err := t.DoRequest(t.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	return tenants, nil
}
func (t *tenant) Delete() *util.APIHandleError {
	return nil
}
func (t *tenant) Services(serviceAlias string) ServiceInterface {
	return &services{
		prefix: path.Join(t.prefix, "services", serviceAlias),
		tenant: *t,
	}
}

//ServiceInterface ServiceInterface
type ServiceInterface interface {
	Get() (*serviceInfo, *util.APIHandleError)
	Pods() ([]*podInfo, *util.APIHandleError)
	List() ([]*dbmodel.TenantServices, *util.APIHandleError)
	Stop(eventID string) (string, *util.APIHandleError)
	Start(eventID string) (string, *util.APIHandleError)
	EventLog(eventID, level string) ([]*model.MessageData, *util.APIHandleError)
}

func (s *services) Pods() ([]*podInfo, *util.APIHandleError) {
	var gc []*podInfo
	var decode utilhttp.ResponseBody
	decode.List = &gc
	code, err := s.DoRequest(s.prefix+"/pods", "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	return gc, nil
}
func (s *services) Get() (*serviceInfo, *util.APIHandleError) {
	var service serviceInfo
	var decode utilhttp.ResponseBody
	decode.Bean = &service
	code, err := s.DoRequest(s.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get err with code %d", code))
	}
	return &service, nil
}
func (s *services) EventLog(eventID, level string) ([]*model.MessageData, *util.APIHandleError) {
	data := []byte(`{"event_id":"` + eventID + `","level":"` + level + `"}`)
	var message []*model.MessageData
	var decode utilhttp.ResponseBody
	decode.List = &message
	code, err := s.DoRequest(s.prefix+"/event-log", "POST", bytes.NewBuffer(data), &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	return message, nil
}

func (s *services) List() ([]*dbmodel.TenantServices, *util.APIHandleError) {
	var gc []*dbmodel.TenantServices
	var decode utilhttp.ResponseBody
	decode.List = &gc
	code, err := s.DoRequest(s.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get with code %d", code))
	}
	return gc, nil
}
func (s *services) Stop(eventID string) (string, *util.APIHandleError) {
	if eventID == "" {
		eventID = coreutil.NewUUID()
	}
	data := []byte(`{"event_id":"` + eventID + `"}`)
	code, err := s.DoRequest(s.prefix+"/stop", "POST", bytes.NewBuffer(data), nil)
	return eventID, handleErrAndCode(err, code)
}
func (s *services) Start(eventID string) (string, *util.APIHandleError) {
	if eventID == "" {
		eventID = coreutil.NewUUID()
	}
	data := []byte(`{"event_id":"` + eventID + `"}`)
	code, err := s.DoRequest(s.prefix+"/start", "POST", bytes.NewBuffer(data), nil)
	return eventID, handleErrAndCode(err, code)
}
func (r *regionImpl) GetEndpoint() string {
	return r.Endpoints[0]
}

//DoRequest do request
func (r *regionImpl) DoRequest(path, method string, body io.Reader, decode *utilhttp.ResponseBody) (int, error) {
	request, err := http.NewRequest(method, r.GetEndpoint()+path, body)
	if err != nil {
		return 500, err
	}
	request.Header.Set("Content-Type", "application/json")
	if r.Token != "" {
		request.Header.Set("Authorization", "Token "+r.Token)
	}
	res, err := r.Client.Do(request)
	if err != nil {
		return 500, err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	if decode != nil {
		if err := json.NewDecoder(res.Body).Decode(decode); err != nil {
			return res.StatusCode, err
		}
	}
	return res.StatusCode, err
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

//ResourcesInterface ResourcesInterface
type ResourcesInterface interface {
	Tenants(tenantName string) ResourcesTenantInterface
}

type resources struct {
	regionImpl
	prefix string
}

func (r *resources) Tenants(tenantName string) ResourcesTenantInterface {
	return &resourcesTenant{prefix: path.Join(r.prefix, "tenants", tenantName), resources: *r}
}

//ResourcesTenantInterface ResourcesTenantInterface
type ResourcesTenantInterface interface {
	Get() (*model.TenantResource, *util.APIHandleError)
}
type resourcesTenant struct {
	resources
	prefix string
}

func (r *resourcesTenant) Get() (*model.TenantResource, *util.APIHandleError) {
	var rt model.TenantResource
	var decode utilhttp.ResponseBody
	decode.Bean = &rt
	code, err := r.DoRequest(r.prefix+"/res", "GET", nil, &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	return &rt, nil
}
