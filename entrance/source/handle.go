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

package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/entrance/api/controller"
	"github.com/goodrain/rainbond/entrance/api/model"
	"github.com/goodrain/rainbond/entrance/core"
	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/source/config"

	"github.com/Sirupsen/logrus"
	"k8s.io/api/core/v1"
)

func chargeMethod(op config.Operation) core.EventMethod {
	var method core.EventMethod
	switch op {
	case config.ADD:
		method = core.ADDEventMethod
	case config.UPDATE:
		method = core.UPDATEEventMethod
	case config.REMOVE:
		method = core.DELETEEventMethod
	}
	return method
}

func (m *Manager) addPodSource(s *config.SourceBranch) {
	s.State = "active"
	if s.IsMidonet {
		report, err := m.getHostPort(s.PodName, s.ContainerPort)
		if err != nil {
			logrus.Warn("get pod host port error. " + err.Error())
			s.PodStatus = false
			//continue
		} else {
			if report != 0 {
				s.NodePort = int32(report)
			} else {
				s.PodStatus = false
			}
		}
	} else {
		s.NodePort = s.ContainerPort
	}
	// event pool first
	m.RcPool(s)
	// event node
	m.RcNode(s)
}

func (m *Manager) updatePodSource(s *config.SourceBranch) {
	if s.IsMidonet {
		report, err := m.getHostPort(s.PodName, s.ContainerPort)
		if err != nil {
			logrus.Warn("get pod host port error. " + err.Error())
			s.PodStatus = false
			//continue
		} else {
			if report != 0 {
				s.NodePort = int32(report)
			} else {
				s.PodStatus = false
			}
		}
	} else {
		s.NodePort = s.ContainerPort
	}
	// event node
	m.RcNode(s)
}

func (m *Manager) deletePodSource(s *config.SourceBranch) {
	// event node
	if !s.IsMidonet {
		s.NodePort = s.ContainerPort
	}
	// event node first
	m.RcNode(s)
	// event pool
	m.RcPool(s)
}

func (m *Manager) podSource(pods *v1.Pod, method core.EventMethod) {
	//if pod do not have ip and method is update,ignore it
	if pods.Status.PodIP == "" && method == core.UPDATEEventMethod {
		return
	}
	index, _ := strconv.ParseInt(pods.ResourceVersion, 10, 64)
	var flagHost bool
	for _, envs := range pods.Spec.Containers[0].Env {
		if envs.Name == "CUR_NET" && (envs.Value == "midonet" || envs.Value == "midolnet") {
			flagHost = true
			break
		} else {
			flagHost = false
			continue
		}
	}
	mapPP := make(map[string]string)
	ppInfo, ok := pods.Labels["protocols"]
	if ok {
		if ppInfo == "" {
			ppInfo = "1234_._ptth"
		}
		infoList := strings.Split(ppInfo, "-.-")
		if len(infoList) > 0 {
			for _, pps := range infoList {
				portInfo := strings.Split(pps, "_._")
				mapPP[portInfo[0]] = portInfo[1]
			}
		}
	} else {
		protocolsNumber := pods.Labels["protocols_number"]
		number, err := strconv.Atoi(protocolsNumber)
		if err != nil {
			logrus.Errorf("ports number converted to int failed：", err.Error())
		}
		for i := 0; i < number; i++ {
			protocol := pods.Labels[fmt.Sprintf("%s_%d", "protocol", i)]
			portInfo := strings.Split(protocol, "_._")
			mapPP[portInfo[0]] = portInfo[1]
		}
	}

	//protocols: 5000_._http-.-8080_._stream
	s := &config.SourceBranch{
		Tenant:    pods.Labels["tenant_name"],
		Service:   pods.Labels["name"],
		EventID:   pods.Labels["event_id"],
		Index:     index,
		Method:    method,
		IsMidonet: flagHost,
		Version:   pods.Labels["version"],
		Namespace: pods.Namespace,
	}
	for _, statusInfo := range pods.Status.Conditions {
		if statusInfo.Type == "Ready" && statusInfo.Status == "True" {
			s.PodStatus = true
		}
	}
	if flagHost {
		s.Host = pods.Status.HostIP
	} else {
		s.Host = pods.Status.PodIP
	}

	for _, containersInfo := range pods.Spec.Containers {
		for _, portInfo := range containersInfo.Ports {
			if portInfo.HostPort != 1 && flagHost {
				continue
			}
			s.PodName = pods.Name
			s.ContainerPort = portInfo.ContainerPort
			s.Port = portInfo.ContainerPort
			s.Note = mapPP[fmt.Sprintf("%d", s.Port)]
			switch method {
			case core.ADDEventMethod:
				m.addPodSource(s)
			case core.UPDATEEventMethod:
				m.updatePodSource(s)
			case core.DELETEEventMethod:
				m.deletePodSource(s)
			}
		}
	}
}

//RcPool RcPool
func (m *Manager) RcPool(s *config.SourceBranch) {
	// 159dfa_grf2f1e2_3306
	poolobj := &object.PoolObject{
		Namespace:      s.Namespace,
		ServiceID:      s.ReServiceId(),
		ServiceVersion: s.Version,
		Index:          s.Index,
		Note:           s.Note,
		Name:           s.RePoolName(),
		EventID:        s.EventID,
	}
	etPool := core.Event{
		Method: s.Method,
		Source: poolobj,
	}
	logrus.Debugf("%s a pool source %s", s.Method, poolobj.GetName())
	m.CoreManager.EventChan() <- etPool
}

func (m *Manager) RcNode(s *config.SourceBranch) {
	nodeobj := &object.NodeObject{
		Namespace: s.Namespace,
		Index:     s.Index,
		Host:      s.Host,
		Port:      s.NodePort,
		Protocol:  s.Note,
		State:     s.State,
		PoolName:  s.RePoolName(),
		NodeName:  s.ReNodeName(),
		Ready:     s.PodStatus,
		EventID:   s.EventID,
	}
	etNode := core.Event{
		Method: s.Method,
		Source: nodeobj,
	}
	logrus.Debugf("%s a node source %s", s.Method, nodeobj.GetName())
	m.CoreManager.EventChan() <- etNode
}

//RcVS RcVS
func (m *Manager) RcVS(s *config.SourceBranch) {
	lbMapPort, err := strconv.Atoi(s.LBMapPort)
	if err != nil || lbMapPort == 0 {
		return
	}
	vsobj := &object.VirtualServiceObject{
		Namespace:       s.Namespace,
		Name:            s.ReVSName(),
		Index:           s.Index,
		Port:            int32(lbMapPort),
		Protocol:        s.Protocol,
		DefaultPoolName: s.RePoolName(),
		EventID:         s.EventID,
	}
	et := core.Event{
		Method: s.Method,
		Source: vsobj,
	}
	logrus.Debugf("%s a vs source %s", s.Method, vsobj.Name)
	m.CoreManager.EventChan() <- et
}

//ResponseBody 返回主要内容体
type ResponseBody struct {
	Bean     interface{}    `json:"bean,omitempty"`
	List     []model.Domain `json:"list,omitempty"`
	PageNum  int            `json:"pageNumber,omitempty"`
	PageSize int            `json:"pageSize,omitempty"`
	Total    int            `json:"total,omitempty"`
}

//ResponseType 返回内容
type ResponseType struct {
	Code      int          `json:"code"`
	Message   string       `json:"msg"`
	MessageCN string       `json:"msgcn"`
	Body      ResponseBody `json:"body,omitempty"`
}

func (m *Manager) getDomainInfo(s *config.SourceBranch) ([]model.Domain, error) {
	domainURL := fmt.Sprintf(config.DomainAPIURI, m.LBAPIPort, s.Tenant, s.Service)
	var ldomain []model.Domain
	client := &http.Client{}
	request, err := http.NewRequest("GET", domainURL, nil)
	if err != nil {
		return ldomain, errors.New("Request regisonAPI failed. " + err.Error())
	}
	response, err := client.Do(request)
	if err != nil {
		return ldomain, errors.New("Request regisonAPI client failed. " + err.Error())
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if response.StatusCode == 404 {
		return nil, nil
	}
	if response.StatusCode == 200 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return ldomain, err
		}
		domainInfo := ResponseType{
			Body: ResponseBody{
				List: []model.Domain{model.Domain{}},
			},
		}
		if err := json.Unmarshal(body, &domainInfo); err != nil {
			return ldomain, err
		}
		if domainInfo.Code != 200 {
			return ldomain, errors.New(domainInfo.Message)
		}
		return domainInfo.Body.List, nil
	}
	return ldomain, fmt.Errorf("get domain info status is %d", response.StatusCode)
}
func (m *Manager) getHostPort(podName string, port int32) (int, error) {

	url := fmt.Sprintf("http://127.0.0.1:%s/pods/%s/ports/%d/hostport", m.LBAPIPort, podName, port)
	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, errors.New("Request api failed. " + err.Error())
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, errors.New("Request API client failed. " + err.Error())
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if response.StatusCode == 404 {
		return 0, fmt.Errorf("port can not found")
	}
	if response.StatusCode == 200 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return 0, err
		}
		hostport := ResponseType{
			Body: ResponseBody{
				Bean: map[string]interface{}{},
			},
		}
		if err := json.Unmarshal(body, &hostport); err != nil {
			return 0, err
		}
		if hp, ok := hostport.Body.Bean.(map[string]interface{})["host_port"]; ok {
			return strconv.Atoi(hp.(string))
		}
	}
	return 0, fmt.Errorf("get host port info faild, respon status(%d)", response.StatusCode)
}

//RcRule TODO:
// FROM API GET USER DOAMINS CREATE RULE
func (m *Manager) RcRule(s *config.SourceBranch) {
	for _, domain := range s.Domain {
		ruleobj := &object.RuleObject{
			Namespace:       s.Namespace,
			Name:            s.ReRuleName(domain),
			DomainName:      domain,
			Index:           s.Index,
			HTTPS:           s.Protocol == "https",
			PoolName:        s.RePoolName(),
			CertificateName: s.CertificateName,
			EventID:         s.EventID,
		}
		et := core.Event{
			Method: s.Method,
			Source: ruleobj,
		}
		logrus.Debugf("%s a rule source %s", s.Method, ruleobj.GetName())
		m.CoreManager.EventChan() <- et
	}
}

func (m *Manager) replaceDomain(domains []string, s *config.SourceBranch) []string {
	for i := range domains {
		domain := domains[i]
		domainL := strings.Split(domain, ".")
		if s.OriginPort != "" && domainL[0] == fmt.Sprintf("%d", s.Port) {
			domainL[0] = s.OriginPort
			domain = strings.Join(domainL, ".")
		}
		domains[i] = domain
		break
	}
	return domains
}

//RcDomain  RcDomain
// FROM API GET USER DOAMINS
func (m *Manager) RcDomain(s *config.SourceBranch) {
	for _, domain := range s.Domain {
		domainobj := &object.DomainObject{
			Name:     domain,
			Domain:   domain,
			Protocol: s.Protocol,
			Index:    s.Index,
			EventID:  s.EventID,
		}
		etDomain := core.Event{
			Method: s.Method,
			Source: domainobj,
		}
		m.CoreManager.EventChan() <- etDomain
	}
	//处理扩展域名
	domainList, err := m.getDomainInfo(s)
	if err != nil {
		logrus.Debugf("get domainlist err is %s", err)
	}
	if domainList != nil && len(domainList) > 0 {
		for _, domain := range domainList {
			if domain.Certificate != "" && domain.PrivateKey != "" && s.Method != core.DELETEEventMethod {
				ca := &object.Certificate{
					Name:        domain.CertificateName,
					Index:       100001,
					Certificate: domain.Certificate,
					PrivateKey:  domain.PrivateKey,
				}
				m.CoreManager.EventChan() <- core.Event{Method: core.ADDEventMethod, Source: ca}
			}
			ruleObj := &object.RuleObject{
				Name:            controller.RuleName(domain.TenantName, domain.ServiceAlias, domain.DomainName, domain.ServicePort),
				Index:           s.Index,
				PoolName:        controller.RePoolName(domain.TenantName, domain.ServiceAlias, domain.ServicePort),
				Namespace:       domain.TenantID,
				CertificateName: domain.CertificateName,
				DomainName:      domain.DomainName,
			}
			switch domain.Protocol {
			case "http":
				ruleObj.HTTPS = false
				m.CoreManager.EventChan() <- core.Event{Method: s.Method, Source: ruleObj}
			case "https":
				ruleObj.HTTPS = true
				m.CoreManager.EventChan() <- core.Event{Method: s.Method, Source: ruleObj}
			case "httptohttps":
				ruleObj.HTTPS = true
				m.CoreManager.EventChan() <- core.Event{Method: s.Method, Source: ruleObj}
				rulehttp := ruleObj.Copy()
				rulehttp.HTTPS = false
				rulehttp.TransferHTTP = true
				m.CoreManager.EventChan() <- core.Event{Method: s.Method, Source: rulehttp}
			case "httpandhttps":
				ruleObj.HTTPS = true
				m.CoreManager.EventChan() <- core.Event{Method: s.Method, Source: ruleObj}
				rulehttp := ruleObj.Copy()
				rulehttp.HTTPS = false
				m.CoreManager.EventChan() <- core.Event{Method: s.Method, Source: rulehttp}
			}
		}
	}
}

//TODO:
// domain支持多个，即 domain字段可能传入多个域名
func (m *Manager) serviceSource(services *v1.Service, method core.EventMethod) {
	index, _ := strconv.ParseInt(services.ResourceVersion, 10, 64)
	s := &config.SourceBranch{
		Tenant:     services.Labels["tenant_name"],
		Service:    services.Spec.Selector["name"],
		EventID:    services.Labels["event_id"],
		Port:       services.Spec.Ports[0].TargetPort.IntVal,
		Index:      index,
		LBMapPort:  services.Labels["lbmap_port"],
		Domain:     strings.Split(services.Labels["domain"], "___"),
		Method:     method,
		OriginPort: services.Labels["origin_port"],
	}
	// event domain
	s.Domain = m.replaceDomain(s.Domain, s)
	m.RcDomain(s)
	//TODO: "stream" to !http
	if services.Labels["protocol"] != "http" && services.Labels["protocol"] != "https" {
		// event vs
		m.RcVS(s)
	} else {
		// event rule
		m.RcRule(s)
	}
}

//PodsLW watch
//TODO:
//监听服务需要更健壮
//如果退出了需要进程退出
func (m *Manager) PodsLW() {
	defer func() {
		if err := recover(); err != nil {
			m.ErrChan <- fmt.Errorf("%v", err)
			debug.PrintStack()
		}
	}()
loop:
	for {
		select {
		case <-m.Ctx.Done():
			break loop
		case event, ok := <-m.PodUpdateChan:
			if !ok {
				break loop
			}
			if event.Pod != nil {
				m.podSource(event.Pod, chargeMethod(event.Op))
			}
		}
	}
	close(m.PodUpdateChan)
}

//ServicesLW service watch
//TODO:
//监听服务需要更健壮
//如果退出了需要进程退出
func (m *Manager) ServicesLW() {
	defer func() {
		if err := recover(); err != nil {
			m.ErrChan <- fmt.Errorf("%v", err)
			debug.PrintStack()
		}
	}()
loop:
	for {
		select {
		case <-m.Ctx.Done():
			break loop
		case event, ok := <-m.ServiceUpdateChan:
			if !ok {
				break loop
			}
			if event.Service != nil {
				m.serviceSource(event.Service, chargeMethod(event.Op))
			}
		}
	}
	close(m.ServiceUpdateChan)
}
