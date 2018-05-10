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

package controller

import (
	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/store"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/goodrain/rainbond/entrance/core"

	"github.com/goodrain/rainbond/entrance/api/model"
	apistore "github.com/goodrain/rainbond/entrance/api/store"

	"github.com/coreos/etcd/client"
	"github.com/twinj/uuid"

	"github.com/Sirupsen/logrus"
	restful "github.com/emicklei/go-restful"
)

//DomainSource 域名接口
type DomainSource struct {
	coreManager     core.Manager
	readStore       store.ReadStore
	apiStoreManager *apistore.Manager
	index           int64
}

//Register 注册
func (u DomainSource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.Path("/v2/tenants/{tenant_name}/services/{service_alias}/domains").
		Doc("Manage User Domains").
		Param(ws.PathParameter("tenant_name", "tenant name").DataType("string")).
		Param(ws.PathParameter("service_alias", "service alias name").DataType("string")).
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.GET("/{domain_name}").To(u.findDomain).
		// docs
		Doc("get a domain").
		Operation("findDomain").
		Param(ws.PathParameter("domain_name", "domain name").DataType("string")).
		Writes(ResponseType{
			Body: ResponseBody{
				Bean: model.Domain{},
			},
		})) // on the response

	ws.Route(ws.POST("").To(u.createDomain).
		// docs
		Doc("create a domain").
		Operation("createDomain").
		Reads(model.Domain{}).Returns(201, "", ResponseType{}))

	ws.Route(ws.GET("").To(u.getDomainByService).
		// docs
		Doc("get all domain of service").
		Operation("getDomainByService").
		Writes(ResponseType{
			Body: ResponseBody{
				List: []interface{}{model.Domain{}},
			},
		})) // from the request

	ws.Route(ws.DELETE("/{domain_name}").To(u.removeDomain).
		// docs
		Doc("delete a domain").
		Param(ws.PathParameter("domain_name", "domain name").DataType("string")).
		Operation("removeDomain").
		Writes(ResponseType{}))

	container.Add(ws)
}

func (u *DomainSource) findDomain(request *restful.Request, response *restful.Response) {
	domainName := request.PathParameter("domain_name")
	if domainName == "" {
		NewFaliResponse(400, "domain name can not be empty", "域名的名称不能为空", response)
		return
	}
	tenantName := request.PathParameter("tenant_name")
	if tenantName == "" {
		NewFaliResponse(400, "tenant name can not be empty", "租户名称不能为空", response)
		return
	}
	serviceName := request.PathParameter("service_alias")
	if serviceName == "" {
		NewFaliResponse(400, "service name can not be empty", "应用的名称不能为空", response)
		return
	}
	domain := model.Domain{}
	err := u.apiStoreManager.GetSource(u.apiStoreManager.GetDomainKey(tenantName, serviceName, domainName), &domain)
	if err != nil {
		if client.IsKeyNotFound(err) {
			NewFaliResponse(404, "domain not found by uuid "+domainName, "域名不存在", response)
		} else {
			NewFaliResponse(500, "find domain error"+err.Error(), "获取域名错误", response)
		}
	} else {
		NewSuccessResponse(domain, nil, response)
	}
}

func (u *DomainSource) getDomainByService(request *restful.Request, response *restful.Response) {
	tenantName := request.PathParameter("tenant_name")
	if tenantName == "" {
		NewFaliResponse(400, "tenant name can not be empty", "租户的名称不能为空", response)
		return
	}
	serviceName := request.PathParameter("service_alias")
	if serviceName == "" {
		NewFaliResponse(400, "service name can not be empty", "应用的名称不能为空", response)
		return
	}
	list, err := u.apiStoreManager.GetDomainList(u.apiStoreManager.GetDomainKey(tenantName, serviceName, ""))
	if err != nil {
		if client.IsKeyNotFound(err) {
			NewFaliResponse(404, "domain not found ", "域名不存在", response)
		} else {
			NewFaliResponse(500, "find domain list error"+err.Error(), "获取域名错误", response)
		}
		return
	}
	NewSuccessResponse(nil, list, response)
}

func (u *DomainSource) createDomain(request *restful.Request, response *restful.Response) {
	domain := new(model.Domain)
	err := request.ReadEntity(domain)
	if err != nil {
		NewFaliResponse(400, "request body error."+err.Error(), "读取请求数据错误，数据不合法", response)
		return
	}
	tenantName := request.PathParameter("tenant_name")
	if tenantName == "" {
		NewFaliResponse(400, "tenant name can not be empty", "租户名称不能为空", response)
		return
	}
	domain.TenantName = tenantName
	serviceName := request.PathParameter("service_alias")
	if serviceName == "" {
		NewFaliResponse(400, "service name can not be empty", "应用的名称不能为空", response)
		return
	}
	domain.ServiceAlias = serviceName
	if domain.UUID == "" {
		domain.UUID = uuid.NewV4().String()
	}
	if domain.AddTime == "" {
		domain.AddTime = time.Now().Format(time.RFC3339)
	}
	err = u.apiStoreManager.AddSource(u.apiStoreManager.GetDomainKey(domain.TenantName, domain.ServiceAlias, domain.DomainName), domain)
	if err != nil {
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeNodeExist {
				NewFaliResponse(400, "domain is exist.", "域名已存在", response)
				return
			}
		}
		NewFaliResponse(500, err.Error(), "存储域名失败", response)
		return
	}
	//TODO:
	//判断应用状态是否为已在集群部署
	//如果是：创建以下资源
	domainObj := &object.DomainObject{
		Name:     domain.DomainName,
		Index:    atomic.AddInt64(&u.index, 1),
		Protocol: domain.Protocol,
		Domain:   domain.DomainName,
	}
	u.coreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: domainObj}
	ruleObj := &object.RuleObject{
		Name:            RuleName(domain.TenantName, domain.ServiceAlias, domain.DomainName, domain.ServicePort),
		Index:           atomic.AddInt64(&u.index, 1),
		PoolName:        RePoolName(domain.TenantName, domain.ServiceAlias, domain.ServicePort),
		Namespace:       domain.TenantID,
		CertificateName: domain.CertificateName,
		DomainName:      domain.DomainName,
	}
	switch domain.Protocol {
	case "http":
		ruleObj.HTTPS = false
		u.coreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: ruleObj}
	case "https":
		ruleObj.HTTPS = true
		u.coreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: ruleObj}
	case "httptohttps":
		ruleObj.HTTPS = true
		u.coreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: ruleObj}
		rulehttp := ruleObj.Copy()
		rulehttp.HTTPS = false
		rulehttp.TransferHTTP = true
		u.coreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: rulehttp}
	case "httpandhttps":
		ruleObj.HTTPS = true
		u.coreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: ruleObj}
		rulehttp := ruleObj.Copy()
		rulehttp.HTTPS = false
		u.coreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: rulehttp}
	}
	if domain.CertificateName != "" {
		ca := &object.Certificate{
			Name:        domain.CertificateName,
			Index:       100001,
			Certificate: domain.Certificate,
			PrivateKey:  domain.PrivateKey,
		}
		u.coreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: ca}
	}
	NewPostSuccessResponse(domain, nil, response)
}

func (u *DomainSource) removeDomain(request *restful.Request, response *restful.Response) {
	domainName := request.PathParameter("domain_name")
	if domainName == "" {
		NewFaliResponse(400, "domain name can not be empty", "域名的名称不能为空", response)
		return
	}
	tenantName := request.PathParameter("tenant_name")
	if tenantName == "" {
		NewFaliResponse(400, "tenant name can not be empty", "租户名称不能为空", response)
		return
	}
	serviceName := request.PathParameter("service_alias")
	if serviceName == "" {
		NewFaliResponse(400, "service name can not be empty", "应用的名称不能为空", response)
		return
	}
	domain := model.Domain{}
	err := u.apiStoreManager.GetSource(u.apiStoreManager.GetDomainKey(tenantName, serviceName, domainName), &domain)
	if err != nil {
		if client.IsKeyNotFound(err) {
			NewFaliResponse(404, "domain not found by  "+domainName, "域名不存在", response)
		} else {
			NewFaliResponse(500, "find domain error"+err.Error(), "获取域名信息错误", response)
		}
		return
	}
	ruleObj := &object.RuleObject{
		Name:            RuleName(domain.TenantName, domain.ServiceAlias, domain.DomainName, domain.ServicePort),
		Index:           atomic.AddInt64(&u.index, 1),
		PoolName:        RePoolName(domain.TenantName, domain.ServiceAlias, domain.ServicePort),
		Namespace:       domain.TenantID,
		CertificateName: domain.CertificateName,
		DomainName:      domain.DomainName,
	}
	switch domain.Protocol {
	case "http":
		ruleObj.HTTPS = false
		u.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: ruleObj}
	case "https":
		ruleObj.HTTPS = true
		u.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: ruleObj}
	case "httptohttps":
		ruleObj.HTTPS = true
		u.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: ruleObj}
		rulehttp := ruleObj.Copy()
		rulehttp.HTTPS = false
		rulehttp.TransferHTTP = true
		u.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: rulehttp}
	case "httpandhttps":
		ruleObj.HTTPS = true
		u.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: ruleObj}
		rulehttp := ruleObj.Copy()
		rulehttp.HTTPS = false
		u.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: rulehttp}
	}
	logrus.Infof("domain rule delete method already send. %s", ruleObj.Name)
	domainObj := &object.DomainObject{
		Name:     domain.DomainName,
		Index:    atomic.AddInt64(&u.index, 1),
		Protocol: domain.Protocol,
		Domain:   domain.DomainName,
	}
	u.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: domainObj}
	logrus.Infof("domain delete method already send. %s", domainObj.Name)

	err = u.apiStoreManager.DeleteSource(u.apiStoreManager.GetDomainKey(tenantName, serviceName, domainName), false)
	if err != nil {
		if !client.IsKeyNotFound(err) {
			logrus.Error("API delete domain error.", err.Error())
			NewFaliResponse(500, "delete domain error"+err.Error(), "删除域名信息错误", response)
		} else {
			NewFaliResponse(404, "domain not found by  "+domainName, "域名不存在", response)
		}
		return
	}
	NewSuccessResponse(domain, nil, response)
}

func sha8(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	return hex.EncodeToString(bs[:4])
}

//RuleName rule name
func RuleName(Tenant, Service, domain string, port int32) string {
	return fmt.Sprintf("%s_%s_%d_%s.Rule",
		Tenant,
		Service,
		port,
		sha8(domain),
	)
}

//RePoolName pool name
func RePoolName(Tenant, Service string, Port int32) string {
	return fmt.Sprintf("%s@%s_%d.Pool",
		Tenant,
		Service,
		Port,
	)
}
