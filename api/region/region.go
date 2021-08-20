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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/cmd"
	utilhttp "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

var regionAPI, token string

var region Region

//AllTenant AllTenant
var AllTenant string

//Region region api
type Region interface {
	Tenants(name string) TenantInterface
	Resources() ResourcesInterface
	Cluster() ClusterInterface
	Version() string
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
	ServiceID     string `json:"serviceId"`
	TenantID      string `json:"tenantId"`
}

type podInfo struct {
	ServiceID       string                       `json:"service_id"`
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
				Timeout:   15 * time.Second,
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

//LoadConfig load config
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
	if code >= 300 {
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

func handleAPIResult(code int, res utilhttp.ResponseBody) *util.APIHandleError {
	if code >= 300 {
		if res.ValidationError != nil && len(res.ValidationError) > 0 {
			return util.CreateAPIHandleErrorf(code, "msg:%s \napi validation_error: %+v", res.Msg, res.ValidationError)
		}
		return util.CreateAPIHandleErrorf(code, "msg:%s", res.Msg)
	}
	return nil
}
