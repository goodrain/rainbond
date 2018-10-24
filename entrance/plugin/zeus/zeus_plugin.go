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

package zeus

//模块需要完成的功能说明
//1.需要完成通用数据类型对插件数据类型的转化。
//2.需要使用通用数据类型转化插件所需的数据类型
//3.完成插件的操作
//4.需要缓存数据，可以从ctx.Store中获取

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/plugin"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
)

func init() {
	plugin.RegistPlugin("zeus", New)
	plugin.RegistPluginOptionCheck("zeus", Check)
}

//New create zeus plugin
func New(ctx plugin.Context) (plugin.Plugin, error) {
	//跳过证书检测
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	z := &zeus{
		ctx: ctx,
		client: &http.Client{
			Timeout:   3 * time.Second,
			Transport: tr,
		},
		APIVersion: "3.5",
	}
	z.User = ctx.Option["user"]
	z.Password = ctx.Option["password"]
	v := ctx.Option["urls"]

	for _, endpoint := range strings.Split(v, "-") {
		logrus.Info("Add endpoint for zeus ", endpoint)
		z.Endpoints = append(z.Endpoints, endpoint)
	}

	return z, nil
}

//Check check zeus plugin optins
func Check(ctx plugin.Context) error {
	for k, v := range ctx.Option {
		switch k {
		case "user":
			if v == "" {
				return errors.New("zeus user can not be empty")
			}
		case "password":
			if v == "" {
				return errors.New("zeus password can not be empty")
			}
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
					return fmt.Errorf("zeus endpoint url %s is invalid. %s", url, err.Error())
				}
				if url.Scheme != "https" {
					return fmt.Errorf("zeus endpoint url %s is invalid. scheme must be https", url)
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

//zeus 负载均衡控制器
type zeus struct {
	Endpoints  []string
	User       string
	Password   string
	APIVersion string //默认3.5
	ctx        plugin.Context
	client     *http.Client
}

type ZeusError struct {
	Code    int
	Message string
	Err     error
}

func (e *ZeusError) Error() string {
	if e.Message == "" {
		return e.Err.Error()
	}
	return e.Message
}

//Err 创建错误
func Err(err error, msg string, code int) error {
	if err == nil {
		return nil
	}
	return &ZeusError{
		Err:     err,
		Message: msg,
		Code:    code,
	}
}
func handleErr(errs []error) error {
	if errs == nil || len(errs) == 0 {
		return nil
	}
	var msg string
	for _, e := range errs {
		msg += e.Error() + ";"
	}
	return &ZeusError{
		Message: msg,
	}
}

var httpsRule = "httpsproxy"
var httpRule = "httpproxy"

func (z *zeus) put(url string, body []byte) error {
	var err error
	var req *http.Response
	var res *http.Request
	for _, end := range z.Endpoints {
		res, err = http.NewRequest("PUT", end+url, bytes.NewReader(body))
		if err != nil {
			continue
		}
		res.SetBasicAuth(z.User, z.Password)
		if strings.Contains(url, "rule") {
			res.Header.Add("Content-Type", "application/octet-stream")
		} else {
			res.Header.Add("Content-Type", "application/json")
		}

		res = res.WithContext(z.ctx.Ctx)
		req, err = z.client.Do(res)
		if err != nil {
			if err == context.Canceled {
				return Err(err, "", 0)
			}
			continue
		}
		break
	}
	if err != nil {
		return Err(err, "", 0)
	}
	if req.Body != nil {
		defer req.Body.Close()
		result, err := ioutil.ReadAll(req.Body)
		if err == nil {
			logrus.Debug("PUT:" + string(result))
		}
	}
	if req.StatusCode >= 300 {
		return Err(fmt.Errorf("put zeus error code %d", req.StatusCode), "Zeus put request error", req.StatusCode)
	}
	return Err(err, "", 0)
}

func (z *zeus) delete(url string) error {
	var err error
	var req *http.Response
	var res *http.Request
	for _, end := range z.Endpoints {
		res, err = http.NewRequest("DELETE", end+url, nil)
		if err != nil {
			continue
		}
		res.SetBasicAuth(z.User, z.Password)
		res.Header.Add("Content-Type", "application/json")
		res = res.WithContext(z.ctx.Ctx)
		req, err = z.client.Do(res)
		if err != nil {
			if err == context.Canceled {
				return Err(err, "", 0)
			}
			continue
		}
		break
	}
	if err != nil {
		return Err(err, "", 0)
	}
	if req.Body != nil {
		defer req.Body.Close()
		result, err := ioutil.ReadAll(req.Body)
		if err == nil {
			logrus.Debug("DELETE:" + string(result))
		}
	}
	if req.StatusCode >= 300 {
		return Err(fmt.Errorf("delete zeus error code %d", req.StatusCode), "Zeus delete request error", req.StatusCode)
	}
	return Err(err, "", 0)
}

func (z *zeus) get(url string) ([]byte, error) {
	var err error
	var req *http.Response
	var res *http.Request
	for _, end := range z.Endpoints {
		res, err = http.NewRequest("GET", end+url, nil)
		if err != nil {
			continue
		}
		res.SetBasicAuth(z.User, z.Password)
		res.Header.Add("Content-Type", "application/json")
		res = res.WithContext(z.ctx.Ctx)
		req, err = z.client.Do(res)
		if err != nil {
			if err == context.Canceled {
				return nil, Err(err, "", 0)
			}
			continue
		}
		break
	}
	if err != nil {
		return nil, Err(err, "", 0)
	}
	if req.StatusCode != 200 {
		return nil, Err(fmt.Errorf("delete zeus error code %d", req.StatusCode), "Zeus delete request error", req.StatusCode)
	}
	if req.Body != nil {
		defer req.Body.Close()
		result, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, Err(err, "", 0)
		}
		return result, nil
	}
	return nil, Err(err, "", 0)
}
func (z *zeus) getPoolURL(name string) string {
	return fmt.Sprintf("/api/tm/%s/config/active/pools/%s", z.APIVersion, name)
}
func (z *zeus) getRuleURL(name string) string {
	return fmt.Sprintf("/api/tm/%s/config/active/rules/%s", z.APIVersion, name)
}
func (z *zeus) getVSURL(name string) string {
	return fmt.Sprintf("/api/tm/%s/config/active/vservers/%s", z.APIVersion, name)
}
func (z *zeus) getSSLURL(name string) string {
	return fmt.Sprintf("/api/tm/%s/config/active/ssl/server_keys/%s", z.APIVersion, name)
}

//AddPool 添加池
//添加池
func (z *zeus) AddPool(pools ...*object.PoolObject) error {
	for _, pool := range pools {
		poolBasic := PoolBasic{
			Note:       pool.Note,
			NodesTable: []*ZeusNode{},
			Monitors:   []string{"Connect"},
		}
		zeusSource := Source{
			Properties: PoolProperties{
				Basic: poolBasic,
				Connection: PoolConnection{
					MaxReplyTime: 100,
				},
			},
		}
		body, err := zeusSource.GetJSON()
		if err != nil {
			return err
		}
		err = z.put(z.getPoolURL(pool.Name), body)
		if err != nil {
			return err
		}
	}
	return nil
}

func (z *zeus) UpdatePool(pools ...*object.PoolObject) error {
	var errs []error
	for _, pool := range pools {
		nodes, err := z.ctx.Store.GetNodeByPool(pool.Name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		//if nodes is null ,will delete the pool
		if len(nodes) == 0 {
			logrus.Info("the pool don't have node, will delete the pool.")
			err = z.DeletePool(pool)
			if err != nil {
				logrus.Error("delete the pool when pool dont't have node error.", err.Error())
			}
			continue
		}
		var nodesTable []*ZeusNode
		for _, node := range nodes {
			//添加对node资源可用性判断
			//停止应用时的更新操作可能更新空host和0port进入etcd
			if node.Ready && node.Port != 0 {
				z := &ZeusNode{
					Node:   fmt.Sprintf("%s:%d", node.Host, node.Port),
					State:  node.State,
					Weight: node.Weight,
				}
				if z.State == "" {
					z.State = "active"
				}
				if z.Weight == 0 {
					z.Weight = 100
				}
				nodesTable = append(nodesTable, z)
			}
		}
		if nodesTable == nil {
			nodesTable = []*ZeusNode{}
		}
		poolBasic := PoolBasic{
			Note:       pool.Note,
			NodesTable: nodesTable,
			Monitors:   []string{"Connect"},
		}
		zeusSource := Source{
			Properties: PoolProperties{
				Basic: poolBasic,
				Connection: PoolConnection{
					MaxReplyTime: 100,
				},
			},
		}
		body, err := zeusSource.GetJSON()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		err = z.put(z.getPoolURL(pool.Name), body)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return handleErr(errs)
}
func (z *zeus) DeletePool(pools ...*object.PoolObject) error {
	var errs []error
	for _, pool := range pools {
		err := z.delete(z.getPoolURL(pool.Name))
		if err != nil {
			errs = append(errs, err)
		}
	}
	return handleErr(errs)
}

func (z *zeus) GetPool(name string) *object.PoolObject {
	return nil
}

func (z *zeus) UpdateNode(nodes ...*object.NodeObject) error {
	poolNames := make(map[string]string, 0)
	for _, node := range nodes {
		poolNames[node.PoolName] = node.NodeName
	}
	pools, err := z.ctx.Store.GetPools(poolNames)
	if err != nil {
		return err
	}
	return z.UpdatePool(pools...)
}

func (z *zeus) DeleteNode(nodes ...*object.NodeObject) error {
	return z.UpdateNode(nodes...)
}

func (z *zeus) AddNode(nodes ...*object.NodeObject) error {
	return z.UpdateNode(nodes...)
}
func (z *zeus) GetNode(name string) *object.NodeObject {
	return nil
}

func (z *zeus) UpdateRule(rules ...*object.RuleObject) error {
	var https, http bool
	var err error
	for _, rule := range rules {
		if rule.CertificateName != "" && rule.HTTPS {
			https = true
		} else {
			http = true
		}
	}
	if err != nil {
		return err
	}
	if https {
		err = z.updateRule("https")
	}
	if http {
		err = z.updateRule("http")
	}
	return err
}

func (z *zeus) updateRule(scheme string) error {
	rs, err := z.ctx.Store.GetAllRule(scheme)
	if err != nil {
		return err
	}
	if scheme == "https" {
		rules := CreateHTTPSRule(rs)
		err = z.put(z.getRuleURL(rules.Name()), rules.Bytes())
	}
	if scheme == "http" {
		rules := CreateHTTPRule(rs)
		err = z.put(z.getRuleURL(rules.Name()), rules.Bytes())
	}
	return err
}

func (z *zeus) addSSL(name string, ssl SSL) error {
	body, err := Source{Properties: SSLProperties{Basic: ssl}}.GetJSON()
	if err != nil {
		return errors.New("ssl json string get error")
	}
	return z.put(z.getSSLURL(name), body)
}
func (z *zeus) deleteSSL(name string) error {
	return z.delete(z.getSSLURL(name))
}

func (z *zeus) DeleteRule(rules ...*object.RuleObject) error {
	var https, http bool
	var err error
	for _, rule := range rules {
		if rule.CertificateName != "" && rule.HTTPS {
			https = true
		}
	}
	if https {
		err = z.updateRule("https")
	}
	if http {
		err = z.updateRule("http")
	}
	return err
}
func (z *zeus) AddRule(rules ...*object.RuleObject) error {
	return z.UpdateRule(rules...)
}
func (z *zeus) GetRule(name string) *object.RuleObject {
	return nil
}

func (z *zeus) AddDomain(domains ...*object.DomainObject) error {
	return z.UpdateDomain(domains...)
}
func (z *zeus) UpdateDomain(domains ...*object.DomainObject) error {
	// var https, http bool
	// for _, do := range domains {
	// 	if do.Protocol == "https" {
	// 		https = true
	// 	} else {
	// 		http = true
	// 	}
	// }
	// if https {
	// 	return z.updateRule("https")
	// }
	// if http {
	// 	return z.updateRule("http")
	// }
	return nil
}
func (z *zeus) DeleteDomain(domains ...*object.DomainObject) error {
	return z.UpdateDomain(domains...)
}
func (z *zeus) GetDomain(name string) *object.DomainObject {
	return nil
}

func (z *zeus) GetName() string {
	return "zeus"
}
func (z *zeus) Stop() error {
	return nil
}

func (z *zeus) AddVirtualService(services ...*object.VirtualServiceObject) error {
	return z.UpdateVirtualService(services...)
}
func (z *zeus) createVSssl() (VSssl, error) {
	var ssl VSssl
	rules, err := z.ctx.Store.GetAllRule("https")
	if err != nil {
		return ssl, err
	}
	var maps []*HostMaping
	for _, rule := range rules {
		if rule.DomainName != "" && rule.CertificateName != "" {
			m := HostMaping{
				Host:            rule.DomainName,
				CertificateName: rule.CertificateName,
			}
			maps = append(maps, &m)
		}
	}
	ssl.ServerCertHostMapping = maps
	//默认证书名
	ssl.ServerCertDefault = "goodrain.com"
	return ssl, nil
}

func (z *zeus) closeSSl() VSssl {
	var ssl VSssl
	ssl.ServerCertHostMapping = []*HostMaping{}
	return ssl
}

func (z *zeus) UpdateVirtualService(services ...*object.VirtualServiceObject) error {
	for _, vs := range services {
		basic := VSBasic{
			Note:             vs.Note,
			Port:             vs.Port,
			DefaultPoolName:  vs.DefaultPoolName,
			Enabled:          true,
			AddXForwardedFor: true,
			ConnectTimeout:   300,
		}
		if vs.Name == "HTTPS.VS" {
			basic.ConnectTimeout = 10
		}
		if vs.Listening == nil || len(vs.Listening) == 0 {
			basic.ListenONAny = true
		} else {
			basic.ListenONHosts = vs.Listening
		}
		if vs.Protocol == "udp" {
			basic.Protocol = "udp"
		} else if vs.Protocol != "http" {
			basic.Protocol = "stream"
		} else {
			basic.Protocol = "http"
		}
		vsPro := VSProperties{
			Basic: basic,
		}
		//特殊处理HTTPS.VS设备，增加证书域名映射关系
		if vs.Name == "HTTPS.VS" {
			ssl, err := z.createVSssl()
			if err != nil {
				return err
			}
			vsPro.SSL = ssl
			vsPro.Basic.RequestRules = vs.Rules
			vsPro.Basic.SSLDecrypt = true
			vsPro.Log = VsLog{
				Enabled:  true,
				Format:   `%{X-Forwarded-For}i %a %{%Y-%m-%d %T}t %m "%f" %s %T %b "%{Referer}i" "%{User-agent}i" "%{host}i" %o`,
				Filename: "/logs/zxtm/%v_%{%Y-%m-%d-%H}t.log",
			}
		} else {
			vsPro.SSL = z.closeSSl()
		}
		zeusVS := Source{
			Properties: vsPro,
		}
		body, err := zeusVS.GetJSON()
		if err != nil {
			return err
		}
		err = z.put(z.getVSURL(vs.Name), body)
		if err != nil {
			logrus.Errorf("put Virtual Service error. %s", err.Error())
			return err
		}
	}
	return nil
}
func (z *zeus) DeleteVirtualService(services ...*object.VirtualServiceObject) error {
	var errs []error
	for _, vs := range services {
		err := z.delete(z.getVSURL(vs.Name))
		if err != nil {
			errs = append(errs, err)
		}
	}
	return handleErr(errs)
}
func (z *zeus) GetVirtualService(name string) *object.VirtualServiceObject {
	return nil
}

func (z *zeus) GetPluginStatus() bool {
	_, err := z.get("/")
	if err != nil {
		logrus.Error("zeus status error.", err.Error())
		return false
	}
	return true
}
func (z *zeus) AddCertificate(cas ...*object.Certificate) error {
	var errs []error
	for _, ca := range cas {
		ssl := SSL{
			Note:    ca.Name,
			Private: ca.PrivateKey,
			Public:  ca.Certificate,
			Request: "",
		}
		err := z.addSSL(ca.Name, ssl)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return handleErr(errs)
}
func (z *zeus) DeleteCertificate(cas ...*object.Certificate) error {
	var errs []error
	for _, ca := range cas {
		err := z.deleteSSL(ca.Name)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return handleErr(errs)
}
