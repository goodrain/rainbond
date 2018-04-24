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

package nginx

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/plugin"

	"time"

	"github.com/Sirupsen/logrus"
)

func init() {
	plugin.RegistPlugin("nginx", New)
	plugin.RegistPluginOptionCheck("nginx", Check)
}

func New(ctx plugin.Context) (plugin.Plugin, error) {
	n := &nginxAPI{
		ctx: ctx,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
	return n, nil
}

func checkURLS(s string, errs string) error {
	var urls []string
	if strings.Contains(s, ",") {
		urls = strings.Split(s, ",")
	} else {
		urls = append(urls, s)
	}
	for _, url := range urls {
		if !bytes.HasPrefix([]byte(url), []byte("http://")) {
			return errors.New(errs)
		}
	}
	return nil
}

//Check Check
func Check(ctx plugin.Context) error {
	errMsg := "Nginx httpapi can not be empty, Eg: http://10.12.23.11:10002;http://10.12.23.12:10002"
	for k, v := range ctx.Option {
		switch k {
		case "httpapi":
			if v == "" {
				return errors.New(errMsg)
			}
			checkURLS(v, errMsg)
		case "streamapi":
			if v == "" {
				return errors.New(errMsg)
			}
			checkURLS(v, errMsg)
		}
	}
	return nil
}

func (n *nginxAPI) AddPool(pools ...*object.PoolObject) error {
	return nil
}

func (n *nginxAPI) UpdatePool(pools ...*object.PoolObject) error {
	return nil
}

func (n *nginxAPI) DeletePool(pools ...*object.PoolObject) error {
	var errs []error
	var dps DrainPoolS
	for _, pool := range pools {
		dps.PoolName = pool.Name
	}
	if !n.drainPool(&dps) {
		errs = append(errs, errors.New("drainPool error"))
	}
	return handleErr(errs)
}

func (n *nginxAPI) GetPool(name string) *object.PoolObject {
	return nil
}

func (n *nginxAPI) UpdateNode(nodes ...*object.NodeObject) error {
	return n.AddNode(nodes...)
}

func (n *nginxAPI) AddNode(nodes ...*object.NodeObject) error {
	var errs []error
	var sns StreamNodeS
	for _, node := range nodes {
		sns.PoolName = node.PoolName
		var nodeList []string
		nodeList = append(nodeList, fmt.Sprintf("%s:%d", node.Host, node.Port))
		sns.NodeList = nodeList
		//TODO: "stream" to !http
		if node.Protocol != "http" {
			logrus.Debugf("node protocol --stream  %s", node.PoolName)
			if !n.addStreamNode(&sns) {
				errs = append(errs, errors.New("addPoolNode strem error"))
			}
		} else if node.Protocol == "http" {
			ruleLists, err := n.ctx.Store.GetRuleByPool("http", node.PoolName)
			if err != nil {
				return handleErr(append(errs, err))
			}
			for _, rule := range ruleLists {
				logrus.Debugf("node protocol --http %s", node.PoolName)
				var ads AddDomainS
				ads.PoolName = rule.PoolName
				ads.Domain = rule.DomainName
				nodeLists, err := n.ctx.Store.GetNodeByPool(node.PoolName)
				if err != nil {
					return handleErr(append(errs, err))
				}
				for _, node := range nodeLists {
					ads.NodeList = append(ads.NodeList, fmt.Sprintf("%s:%d", node.Host, node.Port))
				}
				logrus.Debugf("ads. nodelist is %v", nodeList)
				if !n.addDomain(&ads) {
					errs = append(errs, errors.New("addDomain error"))
				}
			}
		}
	}
	return handleErr(errs)
}

func (n *nginxAPI) DeleteNode(nodes ...*object.NodeObject) error {
	var errs []error
	var sns StreamNodeS
	for _, node := range nodes {
		sns.PoolName = node.PoolName
		var nodeList []string
		nodeList = append(nodeList, fmt.Sprintf("%s:%d", node.Host, node.Port))
		logrus.Debugf("in DeleteNode nodelist is %v", nodeList)
		sns.NodeList = nodeList
		//TODO: "stream" to !http
		if node.Protocol != "http" {
			if !n.deleteStreamNode(&sns) {
				errs = append(errs, errors.New("addPoolNode error"))
			}
		} else if node.Protocol == "http" {
			ruleLists, err := n.ctx.Store.GetRuleByPool("http", node.PoolName)
			if err != nil {
				logrus.Warnf("Rule has been deleted,don't need to delete the node")
				return handleErr(append(errs, errors.New("Rule has been deleted,don't need to delete the node")))
			}
			for _, rule := range ruleLists {
				logrus.Debugf("node protocol --http %s", node.PoolName)
				var ads AddDomainS
				ads.PoolName = rule.PoolName
				ads.Domain = rule.DomainName
				ads.NodeList = nodeList
				if !n.delDomainNode(&ads) {
					errs = append(errs, errors.New("delDomain error"))
				}
			}
		}
	}
	if len(errs) > 0 {
		return handleErr(errs)
	}
	return nil
}

func (n *nginxAPI) GetNode(name string) *object.NodeObject {
	return nil
}

func (n *nginxAPI) UpdateRule(rules ...*object.RuleObject) error {
	return n.AddRule(rules...)
}

func (n *nginxAPI) DeleteRule(rules ...*object.RuleObject) error {
	var errs []error
	var dds DeleteDomainS
	for _, rule := range rules {
		dds.Domain = rule.DomainName
		dds.PoolName = rule.PoolName
	}
	if !n.deleteDomain(&dds) {
		errs = append(errs, errors.New("deleteDomain error"))
	}
	return handleErr(errs)
}

func (n *nginxAPI) AddRule(rules ...*object.RuleObject) error {
	var errs []error
	var ads AddDomainS
	for _, rule := range rules {
		ads.PoolName = rule.PoolName
		ads.Domain = rule.DomainName
		ads.TransferHTTP = rule.TransferHTTP
		ads.HTTPS = rule.HTTPS
		ads.CertificateName = rule.CertificateName
		nodes, err := n.ctx.Store.GetNodeByPool(ads.PoolName)
		if err != nil {
			return handleErr(append(errs, errors.New("Getnodebypool error")))
		}
		var nodeList []string
		for _, node := range nodes {
			nodeList = append(nodeList, fmt.Sprintf("%s:%d", node.Host, node.Port))
		}
		if len(nodeList) == 0 {
			nodeList = append(nodeList, "128.0.0.1:65533")
		}
		ads.NodeList = nodeList
		if !n.addDomain(&ads) {
			errs = append(errs, errors.New("addDomain error"))
		}
	}
	return handleErr(errs)
}

func (n *nginxAPI) GetRule(name string) *object.RuleObject {
	return nil
}

func (n *nginxAPI) AddDomain(domains ...*object.DomainObject) error {
	return n.UpdateDomain(domains...)
}

func (n *nginxAPI) UpdateDomain(domains ...*object.DomainObject) error {

	return nil
}

func (n *nginxAPI) DeleteDomain(domains ...*object.DomainObject) error {
	return n.UpdateDomain(domains...)
}

func (n *nginxAPI) GetDomain(name string) *object.DomainObject {
	return nil
}

func (n *nginxAPI) GetName() string {
	return "nginx"
}

func (n *nginxAPI) Stop() error {
	return nil
}

func (n *nginxAPI) AddVirtualService(services ...*object.VirtualServiceObject) error {
	var errs []error
	var ass AddVirtualServerS
	for _, service := range services {
		ass.PoolName = service.DefaultPoolName
		ass.Port = fmt.Sprintf("%d", service.Port)
		logrus.Debugf("AddVitrual port is %d", ass.Port)
		ass.virtual_server_name = service.Name
		nodes, err := n.ctx.Store.GetNodeByPool(ass.PoolName)
		if err != nil {
			return handleErr(append(errs, err))
		}
		var nodeList []string
		for _, node := range nodes {
			nodeList = append(nodeList, fmt.Sprintf("%s:%d", node.Host, node.Port))
		}
		if len(nodeList) == 0 {
			nodeList = append(nodeList, "128.0.0.1:65533")
		}
		ass.NodeList = nodeList
		if !n.addVirtualServer(&ass) {
			errs = append(errs, errors.New("addVirtualServer error"))
		}
	}
	return handleErr(errs)
}

func (n *nginxAPI) UpdateVirtualService(services ...*object.VirtualServiceObject) error {
	return nil
}

func (n *nginxAPI) DeleteVirtualService(services ...*object.VirtualServiceObject) error {
	var errs []error
	var dvss DeleteVirtualServerS
	for _, service := range services {
		dvss.VirtualServerName = service.Name
		dvss.PoolName = service.DefaultPoolName
		dvss.Port = fmt.Sprintf("%d", service.Port)
		if !n.deleteVirtualServer(&dvss) {
			errs = append(errs, errors.New("deleteVirtualServer error"))
		}

	}
	return handleErr(errs)
}

func (n *nginxAPI) GetVirtualService(name string) *object.VirtualServiceObject {
	return nil
}

func (n *nginxAPI) GetPluginStatus() bool {
	return true
}

func (n *nginxAPI) AddCertificate(cas ...*object.Certificate) error {
	var errs []error
	for _, ca := range cas {
		ssl := SSLCert{
			CertName:   ca.Name,
			Key:        ca.PrivateKey,
			CA:         ca.Certificate,
			HTTPMethod: MethodPOST,
		}
		errs = n.pHTTPSCert(&ssl, errs)
	}
	return handleErr(errs)
}
func (n *nginxAPI) DeleteCertificate(cas ...*object.Certificate) error {
	var errs []error
	for _, ca := range cas {
		ssl := SSLCert{
			CertName:   ca.Name,
			HTTPMethod: MethodDELETE,
		}
		errs = n.pHTTPSCert(&ssl, errs)
	}
	return handleErr(errs)
}

type nginxAPI struct {
	ctx    plugin.Context
	client *http.Client
}

type NginxError struct {
	Code    int
	Message string
	Err     error
}

func (e *NginxError) Error() string {
	if e.Message == "" {
		return e.Err.Error()
	}
	return e.Message
}

func Err(err error, msg string, code int) error {
	if err == nil {
		return nil
	}
	return &NginxError{
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
	return &NginxError{
		Message: msg,
	}
}

func (n *nginxAPI) readPoolName(poolname string) (*PoolName, error) {
	// %s@%s_%d.Pool poolname format
	var p PoolName
	poolExp := regexp.MustCompile(PoolExpString)
	lPoolExp := poolExp.FindStringSubmatch(poolname)
	if len(lPoolExp) != 4 {
		return &p, errors.New("PoolName is unexpect.Please check")
	}
	logrus.Debugf("%v", lPoolExp)
	p.Tenantname = lPoolExp[1]
	p.Servicename = lPoolExp[2]
	p.Port = lPoolExp[3]
	return &p, nil
}

func (n *nginxAPI) StreamPoolInfo(poolname, mapport string) (*PoolName, error) {
	var p PoolName
	poolExp := regexp.MustCompile(PoolExpString)
	lPoolExp := poolExp.FindStringSubmatch(poolname)
	if len(lPoolExp) != 4 {
		return &p, errors.New("PoolName is unexpect.Please check")
	}
	p.Tenantname = lPoolExp[1]
	p.Servicename = lPoolExp[2]
	p.Port = mapport
	return &p, nil
}

func reUpStream(nodelist []string) []byte {
	upstream := bytes.NewBuffer(nil)
	for key, node := range nodelist {
		if key > 0 {
			upstream.WriteString(`&`)
		}
		upstream.WriteString(fmt.Sprintf(`upstream=%s`, node))
	}
	return upstream.Bytes()
}

func (n *nginxAPI) addDomain(ads *AddDomainS) bool {
	// 添加域名
	logrus.Debugf("<LBNGINX>[addDomain]add domain:%s, pool_name:%s, update_nodes:%v",
		ads.Domain,
		ads.PoolName,
		ads.NodeList)
	if len(ads.NodeList) == 0 {
		logrus.Warnf("<LBNGINX>[addDomain]domain %s node is None", ads.Domain)
		return true
	}
	logrus.Debugf("domain is %s", ads.Domain)
	p, err := n.readPoolName(ads.PoolName)
	if err != nil {
		logrus.Error(err)
		return false
	}
	upstream := reUpStream(ads.NodeList)
	logrus.Debugf("<LBNGINX>[addDomain]post_http, tenant:%s, service:%s, upstream:%s",
		p.Tenantname,
		p.Servicename,
		upstream)
	pha := &MethodHTTPArgs{
		PoolName: p,
		UpStream: upstream,
		Method:   MethodPOST,
	}
	n.pHTTP(pha)
	if !strings.HasPrefix(ads.Domain, fmt.Sprintf("%s.%s", p.Port, p.Servicename)) {
		if ads.HTTPS && ads.CertificateName != "" {
			httpsInfo := bytes.NewBuffer(nil)
			httpsInfo.WriteString(`https=https`)
			httpsInfo.WriteString(fmt.Sprintf(`&cert_name=%s&`, ads.CertificateName))
			httpsInfo.WriteString(string(upstream))
			logrus.Debugf("https info is %v", string(httpsInfo.Bytes()))
			pha.UpStream = httpsInfo.Bytes()
		} else if ads.TransferHTTP && ads.CertificateName != "" {
			httpsInfo := bytes.NewBuffer(nil)
			httpsInfo.WriteString(`https=tran_https`)
			httpsInfo.WriteString(fmt.Sprintf(`&cert_name=%s&`, ads.CertificateName))
			httpsInfo.WriteString(string(upstream))
			logrus.Debugf("trans https info is %v", string(httpsInfo.Bytes()))
			pha.UpStream = httpsInfo.Bytes()
		}
		n.pHTTPDomain(ads.Domain, pha)
	}
	return true
}

func (n *nginxAPI) delDomainNode(ads *AddDomainS) bool {
	// 添加域名
	logrus.Debugf("<LBNGINX>[addDomain]del domain:%s, pool_name:%s, update_nodes:%v",
		ads.Domain,
		ads.PoolName,
		ads.NodeList)
	if len(ads.NodeList) == 0 {
		logrus.Warnf("<LBNGINX>[addDomain]domain %s node is None", ads.Domain)
		return true
	}
	logrus.Debugf("domain is %s", ads.Domain)
	p, err := n.readPoolName(ads.PoolName)
	if err != nil {
		logrus.Error(err)
		return false
	}
	upstream := reUpStream(ads.NodeList)
	logrus.Debugf("<LBNGINX>[addDomain]post_http, tenant:%s, service:%s, upstream:%s",
		p.Tenantname,
		p.Servicename,
		upstream)
	pha := &MethodHTTPArgs{
		PoolName: p,
		UpStream: upstream,
		Method:   MethodDELETE,
	}
	n.pUpStreamServer(pha)
	if !bytes.HasPrefix([]byte(ads.Domain), []byte(fmt.Sprintf("%s.%s", p.Port, p.Servicename))) {
		//n.pHTTPDomain(ads.Domain, pha)
		n.pUpStreamDomainServer(pha)
	}
	return true
}

func (n *nginxAPI) addStreamNode(sns *StreamNodeS) bool {
	logrus.Debugf("<LBNGINX>[addStreamNode]pool_name:%s,node:%v",
		sns.PoolName,
		sns.NodeList)
	if len(sns.NodeList) < 1 {
		logrus.Warnf("<LBNGINX>[addStreanNode to upstream %s node is none", sns.PoolName)
		return true
	}
	vs, err := n.ctx.Store.GetVSByPoolName(sns.PoolName)
	if err != nil {
		logrus.Error("<LBNGINX> get vs info when add stream node error,", err)
		return false
	}
	// loglolol
	p, err := n.StreamPoolInfo(sns.PoolName, fmt.Sprintf("%d", vs.Port))
	if err != nil {
		logrus.Error(err)
		return false
	}
	upstream := reUpStream(sns.NodeList)
	logrus.Debugf("<LBNGINX>[addStreamNode], tenant:%s, service:%s, upstream:%s",
		p.Tenantname,
		p.Servicename,
		upstream)
	pha := &MethodHTTPArgs{
		PoolName:     p,
		UpStreamName: sns.PoolName,
		UpStream:     upstream,
		Method:       MethodPOST,
	}
	n.pUpStreamStream(pha)
	return true
}

func (n *nginxAPI) addHttpNode(sns *StreamNodeS) bool {
	logrus.Debugf("<LBNGINX>[addStreamNode]pool_name:%s,node:%v",
		sns.PoolName,
		sns.NodeList)
	if len(sns.NodeList) < 1 {
		logrus.Warnf("<LBNGINX>[addStreanNode to upstream %s node is none", sns.PoolName)
		return true
	}
	p, err := n.readPoolName(sns.PoolName)
	if err != nil {
		logrus.Error(err)
		return false
	}
	upstream := reUpStream(sns.NodeList)
	logrus.Debugf("<LBNGINX>[addStreamNode], tenant:%s, service:%s, upstream:%s",
		p.Tenantname,
		p.Servicename,
		upstream)
	pha := &MethodHTTPArgs{
		PoolName: p,
		UpStream: upstream,
		Method:   MethodPUT,
	}
	n.pUpStreamServer(pha)
	return true
}

func (n *nginxAPI) deleteStreamNode(sns *StreamNodeS) bool {
	logrus.Debugf("<LBNGINX>[addStreamNode]pool_name:%s,node:%v",
		sns.PoolName,
		sns.NodeList)
	if len(sns.NodeList) < 1 {
		logrus.Warnf("<LBNGINX>[addStreanNode to upstream %s node is none", sns.PoolName)
		return true
	}
	upstream := reUpStream(sns.NodeList)
	pha := &MethodHTTPArgs{
		UpStreamName: sns.PoolName,
		UpStream:     upstream,
		Method:       MethodDELETE,
	}
	n.pUpStreamStream(pha)
	return true
}

func (n *nginxAPI) addUserDomain(auds *AddUserDomainS) bool {
	logrus.Debugf("<LBNGINX>[addUserDomain]oldDomain:%s,newDomain:%s,pool_name:%s,node_list:%v",
		auds.OldDomain,
		auds.NewDomain,
		auds.PoolName,
		auds.NodeList)
	var upstream []byte
	if auds.NewDomain != "" {
		if len(auds.NodeList) > 0 {
			upstream = reUpStream(auds.NodeList)
		}
		logrus.Debug("<LBNGINX>[addUserDomain]post_http_domain, domain,%s, upstream:%s", auds.NewDomain, upstream)
		pha := &MethodHTTPArgs{
			UpStream: upstream,
			Method:   MethodPUT,
		}
		n.pHTTPDomain(auds.NewDomain, pha)
	}
	return true
}

func (n *nginxAPI) deleteDomain(dds *DeleteDomainS) bool {
	logrus.Debugf("<LBNGINX>[deleteDomain]domainlist:%v, pool_name:%s,domain:%s",
		dds.DomainList,
		dds.PoolName,
		dds.Domain)
	logrus.Debugf("domain is %s", dds.Domain)
	p, err := n.readPoolName(dds.PoolName)
	if err != nil {
		logrus.Error(err)
		return false
	}
	pha := &MethodHTTPArgs{
		PoolName: p,
		Method:   MethodDELETE,
	}
	if len(dds.DomainList) > 0 {
		for _, domain := range dds.DomainList {
			logrus.Debugf("<LBNGINX>[deleteDomain]delete_http_domain, domain:%s", domain)
			n.pHTTPDomain(domain, pha)
		}
	}
	n.pHTTPDomain(dds.Domain, pha)
	return true
}

func (n *nginxAPI) addVirtualServer(avss *AddVirtualServerS) bool {
	logrus.Debugf("<LBNGINX>[addVirtualServer]virtual_server_name=%s,port=%s,pool_name=%s,updated_nodes=%v",
		avss.virtual_server_name,
		avss.Port,
		avss.PoolName,
		avss.NodeList)
	if len(avss.NodeList) > 0 {
		p, err := n.StreamPoolInfo(avss.PoolName, avss.Port)
		if err != nil {
			logrus.Error(err)
			return false
		}
		upstream := reUpStream(avss.NodeList)
		logrus.Debugf("<LBNGINX>[addVirtualServer]put %s to nginx", upstream)
		pha := &MethodHTTPArgs{
			PoolName:     p,
			UpStreamName: avss.PoolName,
			UpStream:     upstream,
			Method:       MethodPOST,
		}
		n.pStream(pha)
	}
	return true
}

func (n *nginxAPI) updateVirtualServer(avss *AddVirtualServerS) bool {
	logrus.Debugf("<LBNGINX>[addVirtualServer]virtual_server_name=%s,port=%s,pool_name=%s,updated_nodes=%v",
		avss.virtual_server_name,
		avss.Port,
		avss.PoolName,
		avss.NodeList)
	if len(avss.NodeList) > 0 {
		p, err := n.readPoolName(avss.PoolName)
		if err != nil {
			logrus.Error(err)
			return false
		}
		// stream port 为宿主机映射端口
		p.Port = avss.Port
		upstream := reUpStream(avss.NodeList)
		logrus.Debugf("<LBNGINX>[addVirtualServer]put %s to nginx", upstream)
		pha := &MethodHTTPArgs{
			PoolName:     p,
			UpStreamName: avss.PoolName,
			UpStream:     upstream,
			Method:       MethodPUT,
		}
		n.pStream(pha)
	}
	return true
}

func (n *nginxAPI) deleteVirtualServer(dvss *DeleteVirtualServerS) bool {
	logrus.Debugf("<LBNGINX>[deleteVirtualServer]virtual_server_name:%s,pool_name:%s",
		dvss.VirtualServerName,
		dvss.PoolName)
	p, err := n.StreamPoolInfo(dvss.PoolName, dvss.Port)
	if err != nil {
		logrus.Error(err)
		return false
	}
	pha := &MethodHTTPArgs{
		PoolName:     p,
		UpStreamName: dvss.PoolName,
		Method:       MethodDELETE,
	}
	n.pStream(pha)
	return true
}

func (n *nginxAPI) drainNode(dns *DrainNodeS) bool {
	logrus.Debugf("<LBNGINX>[drainNode]pool_name: %s, node: %v", dns.PoolName, dns.NodeList)
	logrus.Debugf("<LBNGINX>[drainNode]tln_type: %s, domain_list:%v", dns.TlnType, dns.DomainList)
	if len(dns.NodeList) == 0 {
		logrus.Warnf("<LBNGINX>[drainNode]pool_name:%s,node is none", dns.PoolName)
		return true
	}
	upstream := reUpStream(dns.NodeList)
	p, err := n.readPoolName(dns.PoolName)
	if err != nil {
		logrus.Error(err)
		return false
	}
	pha := &MethodHTTPArgs{
		PoolName: p,
		Method:   MethodDELETE,
		UpStream: upstream,
	}
	if dns.TlnType == "http" {
		logrus.Debugf("<LBNGINX>[drainNode]delete_http_upstream, tenant:%s, service:%s, upstream:%s",
			p.Tenantname,
			p.Servicename,
			upstream)
		n.pUpStreamServer(pha)
		if len(dns.DomainList) > 0 {
			for _, domain := range dns.DomainList {
				logrus.Debugf("<LBNGINX>[drainNode]delete_http_upstream_domain, domain:%s,upstream:%s",
					domain,
					upstream)
			}
		}
	} else {
		logrus.Debugf("<LBNGINX>[drainNode]delete_stream_upstream, tenant:%s, service:%s, port:%s,upstream:%s",
			p.Tenantname,
			p.Servicename,
			p.Port)
		n.pUpStreamStream(pha)
	}
	return true
}

func (n *nginxAPI) drainPool(dps *DrainPoolS) bool {
	logrus.Debugf("<LBNGINX>[drainPool]pool_name:%s,tln_type:%s,domain_list:%v",
		dps.PoolName,
		dps.TlnType,
		dps.DomainList)
	p, err := n.readPoolName(dps.PoolName)
	if err != nil {
		logrus.Error(err)
		return false
	}
	pha := &MethodHTTPArgs{
		PoolName: p,
		Method:   MethodDELETE,
	}
	if dps.TlnType == "http" {
		logrus.Debugf("<LBNGINX>[drainPool]delete_http, tenant:%s, service:%s", p.Tenantname, p.Servicename)
		n.pHTTP(pha)
		if len(dps.DomainList) > 0 {
			for _, domain := range dps.DomainList {
				logrus.Debugf("<LBNGINX>[drainPool]delete_http_domain, domain:%s", domain)
				n.pHTTPDomain(domain, pha)
			}
		}
	} else {
		logrus.Debugf("<LBNGINX>[drainPool]delete_stream, tenant:%s, service:%s, port:%s",
			p.Tenantname,
			p.Servicename,
			p.Port)
		n.pStream(pha)
	}

	return true
}

func (n *nginxAPI) pHTTPDomain(domain string, p *MethodHTTPArgs) {
	for _, baseURL := range splitURL(n.ctx.Option["httpapi"]) {
		url := fmt.Sprintf("%s/server/%s/%s", baseURL, domain, p.PoolName)
		resp, err := n.urlPPAction(p.Method, url, p.UpStream)
		if err != nil {
			logrus.Error(err)
		}
		logrus.Debug(resp)
	}
}

func (n *nginxAPI) pHTTPSCert(ssl *SSLCert, errs []error) []error {
	for _, baseURL := range splitURL(n.ctx.Option["httpapi"]) {
		url := fmt.Sprintf("%s/ssl/cert/%s", baseURL, ssl.CertName)
		logrus.Debugf("phttps cert url is %s, method is %v", url, ssl.HTTPMethod)
		certInfo := bytes.NewBuffer(nil)
		certInfo.WriteString(fmt.Sprintf(`cert_name=%s`, ssl.CertName))
		if ssl.HTTPMethod == MethodPOST {
			transCA := strings.Replace(ssl.CA, "+", "%2B", -1)
			transKey := strings.Replace(ssl.Key, "+", "%2B", -1)
			certInfo.WriteString(fmt.Sprintf(`&ca=%s`, transCA))
			certInfo.WriteString(fmt.Sprintf(`&key=%s`, transKey))
		}
		logrus.Debugf("cert info is %v", string(certInfo.Bytes()))
		resp, err := n.urlPPAction(ssl.HTTPMethod, url, certInfo.Bytes())
		if err != nil {
			errs = append(errs, err)
			logrus.Error(err)
		}
		logrus.Debug(resp)
	}
	return errs
}

func (n *nginxAPI) pUpStreamServer(p *MethodHTTPArgs) {
	for _, baseURL := range splitURL(n.ctx.Option["streamapi"]) {
		url := fmt.Sprintf("%s/upstream/server/%s/%s/%s", baseURL, p.PoolName.Port, p.PoolName.Servicename, p.PoolName.Tenantname)
		logrus.Debug("pupstreamserver url is %s, method is %v", url, p.Method)
		resp, err := n.urlPPAction(p.Method, url, p.UpStream)
		if err != nil {
			logrus.Error(err)
		}
		logrus.Debug(resp)
	}
}

func (n *nginxAPI) pUpStreamDomainServer(p *MethodHTTPArgs) {
	for _, baseURL := range splitURL(n.ctx.Option["streamapi"]) {
		url := fmt.Sprintf("%s/upstream/server/%s", baseURL, p.Domain)
		logrus.Debug("pupstreamserver url is %s, method is %v", url, p.Method)
		resp, err := n.urlPPAction(p.Method, url, p.UpStream)
		if err != nil {
			logrus.Error(err)
		}
		logrus.Debug(resp)
	}
}

func (n *nginxAPI) pUpStreamStream(p *MethodHTTPArgs) {
	for _, baseURL := range splitURL(n.ctx.Option["streamapi"]) {
		port := "66666"
		if p.PoolName != nil && p.PoolName.Port != "" {
			port = p.PoolName.Port
		}
		url := fmt.Sprintf("%s/upstream/stream/%s/%s", baseURL, p.UpStreamName, port)
		resp, err := n.urlPPAction(p.Method, url, p.UpStream)
		if err != nil {
			logrus.Error(err)
		}
		logrus.Debug(resp)
	}
}

func (n *nginxAPI) pStream(p *MethodHTTPArgs) {
	for _, baseURL := range splitURL(n.ctx.Option["streamapi"]) {
		url := fmt.Sprintf("%s/stream/%s/%s", baseURL, p.UpStreamName, p.PoolName.Port)
		logrus.Debugf("pupstream url is %s, method is %v", url, p.Method)
		resp, err := n.urlPPAction(p.Method, url, p.UpStream)
		if err != nil {
			logrus.Error(err)
		}
		logrus.Debug(resp)
	}
}

func (n *nginxAPI) pHTTP(p *MethodHTTPArgs) {
	for _, baseURL := range splitURL(n.ctx.Option["httpapi"]) {
		url := fmt.Sprintf("%s/server/%s/%s/%s", baseURL, p.PoolName.Port, p.PoolName.Servicename, p.PoolName.Tenantname)
		logrus.Debugf("phttp url is %s, method is %v", url, p.Method)
		resp, err := n.urlPPAction(p.Method, url, p.UpStream)
		if err != nil {
			logrus.Error(err)
		}
		logrus.Debug(resp)
	}
}

func (n *nginxAPI) urlPPAction(method HTTPMETHOD, url string, stream []byte) (*http.Response, error) {
	req, err := http.NewRequest(string(method), url, bytes.NewBuffer(stream))
	if err != nil {
		hr := &http.Response{
			Status: "500",
		}
		return hr, fmt.Errorf("create new request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		hr := &http.Response{
			Status: "500",
		}
		return hr, fmt.Errorf("client do failed")
	}
	return resp, nil
}

func splitURL(urlstr string) []string {
	var urls []string
	if strings.Contains(urlstr, ";") {
		urls = strings.Split(urlstr, ";")
	} else if strings.Contains(urlstr, ",") {
		urls = strings.Split(urlstr, ",")
	} else {
		urls = append(urls, urlstr)
	}
	return urls
}
