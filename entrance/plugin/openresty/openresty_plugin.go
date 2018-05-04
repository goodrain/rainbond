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

package openresty

import(
	"net/http"
	"errors"

	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/plugin"
	"fmt"
	"encoding/json"
	"bytes"
	"io/ioutil"
	"crypto/tls"
	"strings"
	"time"
	"io"
	"net/url"
	"github.com/Sirupsen/logrus"
)

const (
	GET = "GET"
	POST = "POST"
	UPDATE = "UPDATE"
	DELETE = "DELETE"

	GET_OK = 200
	POST_OK = 201
	UPDATE_OK = 202
	DELETE_OK = 204
	HTTP_OK = 205
)


type openresty struct {
	APIVersion string
	ctx        plugin.Context
	client     *http.Client
	user string
	password string
	endpoints []NginxInstance
}

var defaultNodeList = []NginxNode{
	NginxNode{
		"Active",
		"127.0.0.1:404",
		1,
	}}

func init() {
	plugin.RegistPlugin("openresty", New)
	plugin.RegistPluginOptionCheck("openresty", Check)
}

func (this *openresty) urlPool(srcName string) string {
	return fmt.Sprintf("/%s/upstreams/%s", this.APIVersion, srcName)
}

func (this *openresty) urlServer(srcName string) string {
	return fmt.Sprintf("/api/%s/servers/%s", this.APIVersion, srcName)
}

// pool name => domain name
// vzrd9po6@grf40863_8000.Pool => 5000.grb5060d.vzrd9po6.tvga8.goodrain.org
func getUpstreamNameByPool(name string) (string, error) {

	// vzrd9po6@grf40863_8000.Pool => vzrd9po6_grf40863_8000_Pool
	str := strings.Replace(name, "@", "_", -1)
	str = strings.Replace(str, ".", "_", -1)

	// split by "_"
	words := strings.Split(str, "_")

	if len(words) < 3 {
		return "", errors.New("Failed to get the upstream name by pool: " + name)
	}

	domainName := fmt.Sprintf("%s.%s.%s", words[2], words[1], words[0])

	return domainName, nil
}

// vs name => domain name
// voa1i9kc_gr9e98de_8088.VS => 5000.grb5060d.vzrd9po6.tvga8.goodrain.org
func getUpstreamNameByVs(name string) (string, error) {

	// vzrd9po6@grf40863_8000.Pool => vzrd9po6_grf40863_8000_Pool
	str := strings.Replace(name, ".", "_", -1)

	// split by "_"
	words := strings.Split(str, "_")

	if len(words) < 3 {
		return "", errors.New("Failed to get the upstream name by vs: " + name)
	}

	domainName := fmt.Sprintf("%s.%s.%s", words[2], words[1], words[0])

	return domainName, nil
}

func reduceErr(errs []error) error {
	var msg string

	if errs == nil || len(errs) < 1 {
		return nil
	}

	for _, e := range errs {
		msg += e.Error() + "; "
	}

	return errors.New(msg)
}

func (this *openresty) getHealthEndpoints() []NginxInstance {
	arr := []NginxInstance{}
	for _, ins := range this.endpoints {
		if ins.State == "health" {
			arr = append(arr, ins)
		}
	}
	return arr
}

// 调用后台openresty实例的API，如果有多个openresty实例，则循环调用
func (this *openresty) doEach(method string, url string, body interface{}) (e error) {
	var errMsg string

	for _, endpoint := range this.getHealthEndpoints() {
		var bodyReader io.Reader

		if body != nil {
			jsonPool, err := json.Marshal(body)
			if err != nil {
				errMsg += err.Error() + "; "
				logrus.Error(method, " ", err)
				continue
			}

			bodyReader = bytes.NewReader(jsonPool)
		}

		request, err := http.NewRequest(method, endpoint.Addr + url, bodyReader)
		if err != nil {
			errMsg += err.Error() + "; "
			logrus.Error(method, " ", err)
			continue
		}

		response, err := this.client.Do(request)
		if err != nil {
			errMsg += err.Error() + "; "
			logrus.Error(method, " ", err)
			continue
		}

		if response.StatusCode != HTTP_OK  {
			b, err := ioutil.ReadAll(response.Body)
			if err != nil {
				errMsg += err.Error() + "; "
				logrus.Error(method, " ", err)
			} else {
				result := string(b)
				errMsg += result + "; "
				logrus.Error(method, " ", result)
			}
			continue
		}
	}

	if errMsg != "" {
		return errors.New(errMsg)
	}

	logrus.Debug(method, " ", url)
	return nil
}

func (this *openresty) AddPool(originalPools ...*object.PoolObject) error {
	return this.UpdatePool(originalPools...)
}

// 根据pool名字拼接出一个没有后缀的域名，该字名将作为nginx端的upstream名字，后缀部分由nginx端补齐
// nginx默认会试图将所有请求根据请求头中的host字段转发到名字与该host字段值相同的upstream
func (this *openresty) UpdatePool(originalPools ...*object.PoolObject) error {
	var errs []error

	for _, originalPool := range(originalPools) {
		upstreamName, err := getUpstreamNameByPool(originalPool.Name)
		if err != nil {
			logrus.Error(fmt.Sprintf("Failed to update pool %s: %s", originalPool.Name, err))
			continue
		}

		// get nodes from store, for example etcd.
		originalNodes, err := this.ctx.Store.GetNodeByPool(originalPool.Name)
		if err != nil {
			logrus.Error("Failed to GetNodeByPool: ", err.Error())
			errs = append(errs, err)
			continue
		}

		if len(originalNodes) < 1 {
			logrus.Info("Delete the pool, because no servers are inside the pool ", originalPool.Name)
			this.deleteUpstream(originalPool.Name)
			continue
		}

		protocol := "tcp"
		_, err = this.ctx.Store.GetVSByPoolName(originalPool.Name)
		if err != nil {
			logrus.Error("Failed get vs by pool name: ", err)
			protocol = "http"
			continue
		}

		// build pool for openresty by original nodes
		pool := NginxUpstream{upstreamName, []NginxNode{}, protocol}
		for _, originalNode := range(originalNodes){
			state := originalNode.State
			if state == "" {
				state = "Active"
			}

			addr := fmt.Sprintf("%s:%d", originalNode.Host, originalNode.Port)
			if len(originalNode.Host) < 7 || originalNode.Port < 1 {
				logrus.Info(fmt.Sprintf("Ignore node in pool %s, illegal address [%s]", pool.Name, addr))
				continue
			}

			weight := originalNode.Weight
			if weight < 1 {
				weight = 1
			}
			node := NginxNode{
				state,
				addr,
				weight,
			}
			pool.AddNode(node)
		}

		if len(pool.Servers) < 1 {
			logrus.Info("Ignore update the pool, because no servers are inside the pool ", pool.Name)
			continue
		}

		// push data to all openresty instance by rest api
		err = this.doEach(UPDATE, this.urlPool(pool.Name), pool)

		if err != nil {
			errs = append(errs, err)
			continue
		}

	}

	return reduceErr(errs)
}

func (this *openresty) DeletePool(pools ...*object.PoolObject) error {
	var errs []error

	for _, pool := range(pools) {
		upstreamName, err := getUpstreamNameByPool(pool.Name)
		if err != nil {
			logrus.Error(fmt.Sprintf("Failed to update vs %s: %s", pool.Name, err))
			continue
		}

		protocol := "tcp"
		_, err = this.ctx.Store.GetVSByPoolName(pool.Name)
		if err != nil {
			logrus.Error("Failed get vs by pool name: ", err)
			protocol = "http"
			continue
		}

		// request all openresty instance by rest api
		err = this.doEach(DELETE, this.urlPool(upstreamName), map[string]string{"protocol": protocol})

		if err != nil {
			errs = append(errs, err)
			logrus.Error(err)
			continue
		}

	}

	return reduceErr(errs)
}

func (this *openresty) GetPool(name string) *object.PoolObject {
	return nil
}

func (this *openresty) AddNode(nodes ...*object.NodeObject) error {
	return this.UpdateNode(nodes...)
}

// 将node根据所属pool分类，根据每个pool名字取出该pool下所有node，然后全量更新
func (this *openresty) UpdateNode(nodes ...*object.NodeObject) error {
	poolNames := make(map[string]string, 0)

	for _, node := range nodes {
		poolNames[node.PoolName] = node.NodeName
	}

	pools, err := this.ctx.Store.GetPools(poolNames)
	if err != nil {
		logrus.Error(err)
		return err
	}

	return this.UpdatePool(pools...)
}

func (this *openresty) DeleteNode(nodes ...*object.NodeObject) error {
	return this.UpdateNode(nodes...)
}

func (this *openresty) GetNode(name string) *object.NodeObject {
	return nil
}

func (this *openresty) deleteUpstream(poolName string) error {
	if err := this.doEach(DELETE, this.urlPool(poolName), nil); err != nil {
		return err
	}

	return nil
}

// 该函数据存在是为了方便其它函数执行创建upstream的操作
// 比如在nginx中创建server时该server对应的upstream必须存在，此时应该执行此函数
// 如果集群中不存在该upstream次源，则创建一个默认upstream
// poolName指该pool在entrance中的名字，poolAlias指nginx中upstream的名字，一般为一个无后缀域名
func (this *openresty) mustCreateUpstream(poolName string, poolAlias string) error {
	// get nodes from store, for example etcd.
	originalNodes, err := this.ctx.Store.GetNodeByPool(poolName)
	if err != nil {
		logrus.Error("Failed to GetNodeByPool: ", err.Error())
		return err
	}

	protocol := "tcp"
	_, err = this.ctx.Store.GetVSByPoolName(poolName)
	if err != nil {
		logrus.Error("Failed get vs by pool name: ", err)
		protocol = "http"
	}

	// build pool for openresty by original nodes
	pool := NginxUpstream{poolAlias, []NginxNode{}, protocol}
	for _, originalNode := range(originalNodes){
		state := originalNode.State
		if state == "" {
			state = "Active"
		}

		addr := fmt.Sprintf("%s:%d", originalNode.Host, originalNode.Port)
		if len(originalNode.Host) < 7 || originalNode.Port < 1 {
			logrus.Info(fmt.Sprintf("Ignore node in pool %s, illegal address [%s]", pool.Name, addr))
			continue
		}

		weight := originalNode.Weight
		if weight < 1 {
			weight = 1
		}
		node := NginxNode{
			state,
			addr,
			weight,
		}
		pool.AddNode(node)
	}

	if len(pool.Servers) < 1 {
		logrus.Info("No servers are inside the pool, use default pool instead ", poolAlias)
		pool.Servers = defaultNodeList
	}

	// push data to all openresty instance by rest api
	err = this.doEach(UPDATE, this.urlPool(pool.Name), pool)

	if err != nil {
		return err
	}

	return nil
}

func (this *openresty) AddRule(rules ...*object.RuleObject) error {
	return this.UpdateRule(rules...)
}

// 负责L7相关负载均衡，当某应用被创建或添加自定义域名时该方法会被执行
// 在后端的nginx中创建一个server对象，作用是将该规则包含的自定义域名的请求转发到该应用默认的upstream
// 如果该域名是自定义域名，则跳过创建该server，因为nginx自动根据域名将请求转发到相同名字的upstream
func (this *openresty) UpdateRule(rules ...*object.RuleObject) error {
	var errs []error

	for _, rule := range rules {
		// parse protocol name
		protocol := "http"
		if rule.HTTPS {
			protocol = "https"
		}

		// skip create the server config file if is default domain name
		// voa1i9kc_gr086ce9_3306_9051e614.Rule
		words := strings.Split(rule.Name, "_")
		match := fmt.Sprintf("%s.%s.%s", words[2], words[1], words[0])
		if strings.Contains(rule.DomainName, match) {
			logrus.Info("Ignore update the rule, because its a default app domain name: ", rule.DomainName)
			continue
		}

		defaultDomain, err := getUpstreamNameByPool(rule.PoolName)
		if err != nil {
			logrus.Error(fmt.Sprintf("Failed to update rule %s: %s", rule.Name, err))
			continue
		}

		// custom domain name => default upstream
		// myapp.sycki.com => 5000.grb5060d.vzrd9po6.tvga8.goodrain.org
		err = this.mustCreateUpstream(rule.PoolName, defaultDomain)
		if err != nil {
			logrus.Error("Failed to updata the rule: ", err.Error())
			continue
		}

		port := 80
		var path = "/"
		var cert, key string

		// get cert key pair if https
		if protocol == "https" {
			port = 443

			pair, err := this.ctx.Store.GetCertificate(rule.CertificateName)
			if err != nil {
				logrus.Error("Failed to updata the rule: ", err.Error())
				continue
			}

			cert = pair.Certificate
			key = pair.PrivateKey
		}

		openrestyRule := NginxServer{
			rule.Name,
			rule.DomainName,
			int32(port),
			path,
			protocol,
			cert,
			key,
			map[string]string{},
			defaultDomain,
			rule.TransferHTTP,
		}

		// build json data and request api
		err = this.doEach(UPDATE, this.urlServer(rule.Name), openrestyRule)
		if err != nil {
			errs = append(errs, err)
			logrus.Error(err)
			continue
		}


	}

	return reduceErr(errs)
}

func (this *openresty) DeleteRule(rules ...*object.RuleObject) error {
	var errs []error
	for _, rule := range(rules) {
		protocol := "http"
		if rule.HTTPS {
			protocol = "https"
		}

		err := this.doEach(DELETE, this.urlServer(rule.Name), Options{protocol})

		if err != nil {
			errs = append(errs, err)
			logrus.Error(err)
		}
	}
	return reduceErr(errs)
}

func (this *openresty) GetRule(name string) *object.RuleObject {
	return nil
}

func (this *openresty) AddVirtualService(services ...*object.VirtualServiceObject) error {
	return this.UpdateVirtualService(services...)
}

// 负责L4相关负载均衡，当某应用添加外部端口时该方法会被执行
// 在后端的nginx中创建一个server对象，作用是将该规则包含的自定义域名的请求转发到该应用默认的upstream
func (this *openresty) UpdateVirtualService(services ...*object.VirtualServiceObject) error {
	var errs []error
	for _, service := range services {
		upstreamName, err := getUpstreamNameByVs(service.Name)
		if err != nil {
			logrus.Error(fmt.Sprintf("Failed to update vs %s: %s", service.Name, err))
			continue
		}

		poolName := strings.Replace(strings.Replace(service.Name, "_", "@", 1), "VS", "Pool", 1)

		err = this.mustCreateUpstream(poolName, upstreamName)
		if err != nil {
			logrus.Error("Failed update pool for create vs: ", err.Error())
			errs = append(errs, err)
			continue
		}

		if service.Protocol == "" {
			service.Protocol = "tcp"
		}

		openrestyRule := NginxServer{
			Name: service.Name,
			Port: service.Port,
			Options: map[string]string{},
			Upstream: upstreamName,
			Protocol: service.Protocol,
		}

		// build json data and request api
		err = this.doEach(UPDATE, this.urlServer(openrestyRule.Name), openrestyRule)
		if err != nil {
			logrus.Error("Failed update vs: ", err.Error())
			errs = append(errs, err)
			logrus.Error(err)
			continue
		}

	}

	return reduceErr(errs)
}

func (this *openresty) DeleteVirtualService(services ...*object.VirtualServiceObject) error {
	var errs []error
	for _, service := range services {

		if service.Protocol == "" {
			service.Protocol = "tcp"
		}

		err := this.doEach(DELETE, this.urlServer(service.Name), Options{service.Protocol})
		if err != nil {
			errs = append(errs, err)
			logrus.Error(err)
			continue
		}

	}

	return reduceErr(errs)
}

func (this *openresty) GetVirtualService(name string) *object.VirtualServiceObject {return nil}

func (this *openresty) AddDomain(domains ...*object.DomainObject) error {return nil}
func (this *openresty) UpdateDomain(domains ...*object.DomainObject) error {return nil}
func (this *openresty) DeleteDomain(domains ...*object.DomainObject) error {return nil}
func (this *openresty) GetDomain(name string) *object.DomainObject {return nil}

func (this *openresty) AddCertificate(cas ...*object.Certificate) error {return nil}
func (this *openresty) DeleteCertificate(cas ...*object.Certificate) error {return nil}

func (this *openresty) Stop() error {return nil}
func (this *openresty) GetName() string {return "openresty"}

func (this *openresty) GetPluginStatus() bool {
	health := true
	method := GET

	for _, endpoint := range this.getHealthEndpoints() {
		request, err := http.NewRequest(method, endpoint.Addr + "/health", nil)
		if err != nil {
			health = false
			logrus.Debug(method, fmt.Sprintf(" %s %s", endpoint.Addr, err.Error()))
			continue
		}

		response, err := this.client.Do(request)
		if err != nil {
			health = false
			logrus.Debug(method, fmt.Sprintf(" %s %s", endpoint.Addr, err.Error()))
			continue
		}

		if response.StatusCode != HTTP_OK  {
			health = false
			b, err := ioutil.ReadAll(response.Body)
			if err != nil {
				logrus.Debug(method, fmt.Sprintf(" %s %s", endpoint.Addr, err.Error()))
			} else {
				logrus.Debug(method, fmt.Sprintf(" %s %s", endpoint.Addr, string(b)))
			}
			continue
		}
	}

	return health
}

//Check check openresty plugin optins
func Check(ctx plugin.Context) error {
	for k, v := range ctx.Option {
		switch k {
		case "user":
		case "password":
		case "urls":
			var endpoints []string
			if strings.Contains(v, ",") {
				endpoints = strings.Split(v, ",")
			} else {
				endpoints = append(endpoints, v)
			}
			for _, end := range endpoints {
				url, err := url.Parse(end)
				if err != nil {
					return fmt.Errorf("openresty endpoint url %s is invalid. %s", url, err.Error())
				}
				if url.Scheme != "https" && url.Scheme != "http" {
					return fmt.Errorf("openresty endpoint url %s is invalid. scheme must be https", url)
				}
			}
		case "httpapi":
		case "streamapi":
		default:
			return fmt.Errorf("%s option is not support", k)
		}
	}
	return nil
}

// create entrance plugin for openresty
func New(ctx plugin.Context) (plugin.Plugin, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	plugin := &openresty{
		APIVersion: "v1",
		ctx: ctx,
		client: &http.Client{
			Timeout:   3 * time.Second,
			Transport: tr,
		},
		user: ctx.Option["user"],
		password: ctx.Option["password"],
	}

	for _, url := range strings.Split(ctx.Option["urls"], "-") {
		logrus.Info("Add endpoint for openresty ", url)
		plugin.endpoints = append(plugin.endpoints, NginxInstance{Addr: url, State:"health"})
	}

	return plugin, nil
}
