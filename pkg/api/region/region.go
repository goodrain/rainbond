package region

import (
	"net/http"
	"errors"
	"io/ioutil"
	"encoding/json"
	"rainbond/cmd/grctl/option"
	"acp_api/pkg/model"

	"bytes"
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
	Get(name string) *model.ServiceStruct
	List() []model.ServiceStruct
	Stop(serviceAlisa ,eventID string) error
	Start(serviceAlisa ,eventID string) error
}


func (s *Services)Get(name string) *model.ServiceStruct {
	return nil
}

type listServices struct {
	list []model.ServiceStruct `json:"list"`
}
func (s *Services)List() []model.ServiceStruct {

	request, err := http.NewRequest("GET", regionAPI+"/v2/tenants/"+s.tenant.tenantID+"/services", nil)
	if err != nil {
		//return err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Token "+token)
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		//return nil, err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		//return nil, err
	}
	list:=listServices{}
	if err := json.Unmarshal(data, &list); err != nil {
		//return nil, err
	}

	return list.list
}
func (s *Services)Stop(name ,eventID string) error {

	data := []byte(`{"event_id":"` + eventID + `"}`)
	_,err:=DoRequest("/v2/tenants/"+s.tenant.tenantID+"/services"+name+"/stop","POST",data)
	//request, err := http.NewRequest("POST", regionAPI+"/v2/tenants/"+s.tenant.tenantID+"/services"+s.model.ServiceAlias+"/stop", bytes.NewBuffer(data))
	if err!=nil {
		return err
	}

	return nil
}
func (s *Services)Start(name ,eventID string) error {

	data := []byte(`{"event_id":"` + eventID + `"}`)
	_,err:=DoRequest("/v2/tenants/"+s.tenant.tenantID+"/services"+name+"/start","POST",data)
	//request, err := http.NewRequest("POST", regionAPI+"/v2/tenants/"+s.tenant.tenantID+"/services"+s.model.ServiceAlias+"/stop", bytes.NewBuffer(data))
	if err!=nil {
		return err
	}

	return nil
}

func DoRequest(url ,method string, body []byte) ([]byte,error) {
	request, err := http.NewRequest(method, regionAPI+url, bytes.NewBuffer(body))
	if err != nil {
		return nil,err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Token "+token)
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	return data,err
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
func SetRegionInfo(config *option.RegionAPI) {
	regionAPI = config.URL
	token = config.Token
}
