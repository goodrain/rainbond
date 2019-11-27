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

package handler

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/util"
	"github.com/jinzhu/gorm"
)

// GatewayAction -
type GatewayAction struct {
	dbmanager db.Manager
	mqclient  client.MQClient
	etcdCli   *clientv3.Client
}

//CreateGatewayManager creates gateway manager.
func CreateGatewayManager(dbmanager db.Manager, mqclient client.MQClient, etcdCli *clientv3.Client) *GatewayAction {
	return &GatewayAction{
		dbmanager: dbmanager,
		mqclient:  mqclient,
		etcdCli:   etcdCli,
	}
}

// AddHTTPRule adds http rule to db if it doesn't exists.
func (g *GatewayAction) AddHTTPRule(req *apimodel.AddHTTPRuleStruct) (string, error) {
	httpRule := &model.HTTPRule{
		UUID:          req.HTTPRuleID,
		ServiceID:     req.ServiceID,
		ContainerPort: req.ContainerPort,
		Domain:        req.Domain,
		Path: func() string {
			if !strings.HasPrefix(req.Path, "/") {
				return "/" + req.Path
			}
			return req.Path
		}(),
		Header:        req.Header,
		Cookie:        req.Cookie,
		Weight:        req.Weight,
		IP:            req.IP,
		CertificateID: req.CertificateID,
	}

	// begin transaction
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	if err := db.GetManager().HTTPRuleDaoTransactions(tx).AddModel(httpRule); err != nil {
		tx.Rollback()
		return "", err
	}

	if strings.Replace(req.CertificateID, " ", "", -1) != "" {
		cert := &model.Certificate{
			UUID:            req.CertificateID,
			CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
			Certificate:     req.Certificate,
			PrivateKey:      req.PrivateKey,
		}
		if err := db.GetManager().CertificateDaoTransactions(tx).AddOrUpdate(cert); err != nil {
			tx.Rollback()
			return "", err
		}
	}

	for _, ruleExtension := range req.RuleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: httpRule.UUID,
			Key:    ruleExtension.Key,
			Value:  ruleExtension.Value,
		}
		if err := db.GetManager().RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
			tx.Rollback()
			return "", err
		}
	}

	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return "", err
	}
	return httpRule.ServiceID, nil
}

// UpdateHTTPRule updates http rule
func (g *GatewayAction) UpdateHTTPRule(req *apimodel.UpdateHTTPRuleStruct) (string, error) {
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	rule, err := g.dbmanager.HTTPRuleDaoTransactions(tx).GetHTTPRuleByID(req.HTTPRuleID)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	if rule == nil || rule.UUID == "" { // rule won't be nil
		tx.Rollback()
		return "", fmt.Errorf("HTTPRule dosen't exist based on uuid(%s)", req.HTTPRuleID)
	}
	if strings.Replace(req.CertificateID, " ", "", -1) != "" {
		// delete old Certificate
		if err := g.dbmanager.CertificateDaoTransactions(tx).DeleteCertificateByID(rule.CertificateID); err != nil {
			tx.Rollback()
			return "", err
		}
		// add new certificate
		cert := &model.Certificate{
			UUID:            req.CertificateID,
			CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
			Certificate:     req.Certificate,
			PrivateKey:      req.PrivateKey,
		}
		if err := g.dbmanager.CertificateDaoTransactions(tx).AddOrUpdate(cert); err != nil {
			tx.Rollback()
			return "", err
		}
		rule.CertificateID = req.CertificateID
	} else {
		rule.CertificateID = ""
	}
	if len(req.RuleExtensions) > 0 {
		// delete old RuleExtensions
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
			tx.Rollback()
			return "", err
		}
		// add new rule extensions
		for _, ruleExtension := range req.RuleExtensions {
			re := &model.RuleExtension{
				UUID:   util.NewUUID(),
				RuleID: rule.UUID,
				Key:    ruleExtension.Key,
				Value:  ruleExtension.Value,
			}
			if err := db.GetManager().RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
				tx.Rollback()
				return "", err
			}
		}
	}
	// update http rule
	if req.ServiceID != "" {
		rule.ServiceID = req.ServiceID
	}
	if req.ContainerPort != 0 {
		rule.ContainerPort = req.ContainerPort
	}
	if req.Domain != "" {
		rule.Domain = req.Domain
	}
	rule.Path = func() string {
		if !strings.HasPrefix(req.Path, "/") {
			return "/" + req.Path
		}
		return req.Path
	}()
	rule.Header = req.Header
	rule.Cookie = req.Cookie
	rule.Weight = req.Weight
	if req.IP != "" {
		rule.IP = req.IP
	}
	if err := db.GetManager().HTTPRuleDaoTransactions(tx).UpdateModel(rule); err != nil {
		tx.Rollback()
		return "", err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return "", err
	}
	return rule.ServiceID, nil
}

// DeleteHTTPRule deletes http rule, including certificate and rule extensions
func (g *GatewayAction) DeleteHTTPRule(req *apimodel.DeleteHTTPRuleStruct) (string, error) {
	// begin transaction
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	// delete http rule
	httpRule, err := g.dbmanager.HTTPRuleDaoTransactions(tx).GetHTTPRuleByID(req.HTTPRuleID)
	svcID := httpRule.ServiceID
	if err != nil {
		tx.Rollback()
		return "", err
	}
	if err := g.dbmanager.HTTPRuleDaoTransactions(tx).DeleteHTTPRuleByID(httpRule.UUID); err != nil {
		tx.Rollback()
		return "", err
	}
	// delete certificate
	if err := g.dbmanager.CertificateDaoTransactions(tx).DeleteCertificateByID(httpRule.CertificateID); err != nil {
		tx.Rollback()
		return "", err
	}
	// delete rule extension
	if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(httpRule.UUID); err != nil {
		tx.Rollback()
		return "", err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return svcID, nil
}

// DeleteHTTPRuleByServiceIDWithTransaction deletes http rule, including certificate and rule extensions
func (g *GatewayAction) DeleteHTTPRuleByServiceIDWithTransaction(sid string, tx *gorm.DB) error {
	// delete http rule
	rules, err := g.dbmanager.HTTPRuleDaoTransactions(tx).ListByServiceID(sid)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if err := g.dbmanager.CertificateDaoTransactions(tx).DeleteCertificateByID(rule.CertificateID); err != nil {
			return err
		}
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
			return err
		}
		if err := g.dbmanager.GwRuleConfigDaoTransactions(tx).DeleteByRuleID(rule.UUID); err != nil {
			return err
		}
		if err := g.dbmanager.HTTPRuleDaoTransactions(tx).DeleteHTTPRuleByID(rule.UUID); err != nil {
			return err
		}
	}

	return nil
}

// AddCertificate adds certificate to db if it doesn't exists
func (g *GatewayAction) AddCertificate(req *apimodel.AddHTTPRuleStruct, tx *gorm.DB) error {
	cert := &model.Certificate{
		UUID:            req.CertificateID,
		CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
		Certificate:     req.Certificate,
		PrivateKey:      req.PrivateKey,
	}

	return g.dbmanager.CertificateDaoTransactions(tx).AddModel(cert)
}

// UpdateCertificate updates certificate for http rule
func (g *GatewayAction) UpdateCertificate(req apimodel.AddHTTPRuleStruct, httpRule *model.HTTPRule,
	tx *gorm.DB) error {
	// delete old certificate
	cert, err := g.dbmanager.CertificateDaoTransactions(tx).GetCertificateByID(req.CertificateID)
	if err != nil {
		return err
	}
	if cert == nil {
		return fmt.Errorf("Certificate doesn't exist based on certificateID(%s)", req.CertificateID)
	}

	cert.CertificateName = fmt.Sprintf("cert-%s", util.NewUUID()[0:8])
	cert.Certificate = req.Certificate
	cert.PrivateKey = req.PrivateKey
	return g.dbmanager.CertificateDaoTransactions(tx).UpdateModel(cert)
}

// AddTCPRule adds tcp rule.
func (g *GatewayAction) AddTCPRule(req *apimodel.AddTCPRuleStruct) (string, error) {
	// begin transaction
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	// add tcp rule
	tcpRule := &model.TCPRule{
		UUID:          req.TCPRuleID,
		ServiceID:     req.ServiceID,
		ContainerPort: req.ContainerPort,
		IP:            req.IP,
		Port:          req.Port,
	}
	if err := g.dbmanager.TCPRuleDaoTransactions(tx).AddModel(tcpRule); err != nil {
		tx.Rollback()
		return "", err
	}
	// add rule extensions
	for _, ruleExtension := range req.RuleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: tcpRule.UUID,
			Value:  ruleExtension.Value,
		}
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
			tx.Rollback()
			return "", err
		}
	}

	// end transaction
	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return tcpRule.ServiceID, nil
}

// UpdateTCPRule updates a tcp rule
func (g *GatewayAction) UpdateTCPRule(req *apimodel.UpdateTCPRuleStruct, minPort int) (string, error) {
	// begin transaction
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	// get old tcp rule
	tcpRule, err := g.dbmanager.TCPRuleDaoTransactions(tx).GetTCPRuleByID(req.TCPRuleID)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	if len(req.RuleExtensions) > 0 {
		// delete old rule extensions
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(tcpRule.UUID); err != nil {
			logrus.Debugf("TCP rule id: %s;error delete rule extension: %v", tcpRule.UUID, err)
			tx.Rollback()
			return "", err
		}
		// add new rule extensions
		for _, ruleExtension := range req.RuleExtensions {
			re := &model.RuleExtension{
				UUID:   util.NewUUID(),
				RuleID: tcpRule.UUID,
				Value:  ruleExtension.Value,
			}
			if err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
				tx.Rollback()
				logrus.Debugf("TCP rule id: %s;error add rule extension: %v", tcpRule.UUID, err)
				return "", err
			}
		}
	}
	// update tcp rule
	if req.ContainerPort != 0 {
		tcpRule.ContainerPort = req.ContainerPort
	}
	if req.IP != "" {
		tcpRule.IP = req.IP
	}
	tcpRule.Port = req.Port
	if req.ServiceID != "" {
		tcpRule.ServiceID = req.ServiceID
	}
	if err := g.dbmanager.TCPRuleDaoTransactions(tx).UpdateModel(tcpRule); err != nil {
		logrus.Debugf("TCP rule id: %s;error updating tcp rule: %v", tcpRule.UUID, err)
		tx.Rollback()
		return "", err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		logrus.Debugf("TCP rule id: %s;error end transaction %v", tcpRule.UUID, err)
		return "", err
	}
	return tcpRule.ServiceID, nil
}

// DeleteTCPRule deletes a tcp rule
func (g *GatewayAction) DeleteTCPRule(req *apimodel.DeleteTCPRuleStruct) (string, error) {
	// begin transaction
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	tcpRule, err := db.GetManager().TCPRuleDaoTransactions(tx).GetTCPRuleByID(req.TCPRuleID)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	// delete rule extensions
	if err := db.GetManager().RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(tcpRule.UUID); err != nil {
		tx.Rollback()
		return "", err
	}
	// delete tcp rule
	if err := db.GetManager().TCPRuleDaoTransactions(tx).DeleteByID(tcpRule.UUID); err != nil {
		tx.Rollback()
		return "", err
	}
	// delete LBMappingPort
	err = db.GetManager().TenantServiceLBMappingPortDaoTransactions(tx).DELServiceLBMappingPortByServiceIDAndPort(
		tcpRule.ServiceID, tcpRule.Port)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return "", err
	}
	return tcpRule.ServiceID, nil
}

// DeleteTCPRuleByServiceIDWithTransaction deletes a tcp rule
func (g *GatewayAction) DeleteTCPRuleByServiceIDWithTransaction(sid string, tx *gorm.DB) error {
	rules, err := db.GetManager().TCPRuleDaoTransactions(tx).GetTCPRuleByServiceID(sid)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		// delete rule extensions
		if err := db.GetManager().RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
			return err
		}
		// delete tcp rule
		if err := db.GetManager().TCPRuleDaoTransactions(tx).DeleteByID(rule.UUID); err != nil {
			return err
		}
	}
	return nil
}

// AddRuleExtensions adds rule extensions to db if any of they doesn't exists
func (g *GatewayAction) AddRuleExtensions(ruleID string, ruleExtensions []*apimodel.RuleExtensionStruct,
	tx *gorm.DB) error {
	for _, ruleExtension := range ruleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: ruleID,
			Value:  ruleExtension.Value,
		}
		err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetAvailablePort returns a available port
func (g *GatewayAction) GetAvailablePort(ip string) (int, error) {
	roles, err := g.dbmanager.TCPRuleDao().GetUsedPortsByIP(ip)
	if err != nil {
		return 0, err
	}
	var ports []int
	for _, p := range roles {
		ports = append(ports, p.Port)
	}
	port := selectAvailablePort(ports)
	if port != 0 {
		return port, nil
	}
	return 0, fmt.Errorf("no more lb port can be use with ip %s", ip)
}

func selectAvailablePort(used []int) int {
	maxPort, _ := strconv.Atoi(os.Getenv("MAX_LB_PORT"))
	minPort, _ := strconv.Atoi(os.Getenv("MIN_LB_PORT"))
	if minPort == 0 {
		minPort = 10000
	}
	if maxPort == 0 {
		maxPort = 65535
	}
	if len(used) == 0 {
		return minPort
	}

	sort.Ints(used)
	selectPort := used[len(used)-1] + 1
	if selectPort < minPort {
		selectPort = minPort
	}
	//顺序分配端口
	if selectPort <= maxPort {
		return selectPort
	}
	//捡漏以前端口
	selectPort = minPort
	for _, p := range used {
		if p == selectPort {
			selectPort = selectPort + 1
			continue
		}
		if p > selectPort {
			return selectPort
		}
		selectPort = selectPort + 1
	}
	if selectPort <= maxPort {
		return selectPort
	}
	return 0
}

// TCPIPPortExists returns if the port exists
func (g *GatewayAction) TCPIPPortExists(host string, port int) bool {
	roles, _ := db.GetManager().TCPRuleDao().GetUsedPortsByIP(host)
	for _, role := range roles {
		if role.Port == port {
			return true
		}
	}
	return false
}

// SendTask sends apply rules task
func (g *GatewayAction) SendTask(in map[string]interface{}) error {
	sid := in["service_id"].(string)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(sid)
	if err != nil {
		return fmt.Errorf("Unexpected error occurred while getting Service by ServiceID(%s): %v", sid, err)
	}
	body := make(map[string]interface{})
	body["deploy_version"] = service.DeployVersion
	for k, v := range in {
		body[k] = v
	}
	err = g.mqclient.SendBuilderTopic(client.TaskStruct{
		Topic:    client.WorkerTopic,
		TaskType: "apply_rule",
		TaskBody: body,
	})
	if err != nil {
		return fmt.Errorf("Unexpected error occurred while sending task: %v", err)
	}
	return nil
}

// RuleConfig -
func (g *GatewayAction) RuleConfig(req *apimodel.RuleConfigReq) error {
	var configs []*model.GwRuleConfig
	// TODO: use reflect to read the field of req, huangrh
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-connect-timeout",
		Value:  strconv.Itoa(req.Body.ProxyConnectTimeout),
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-send-timeout",
		Value:  strconv.Itoa(req.Body.ProxySendTimeout),
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-read-timeout",
		Value:  strconv.Itoa(req.Body.ProxyReadTimeout),
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-body-size",
		Value:  strconv.Itoa(req.Body.ProxyBodySize),
	})
	setheaders := make(map[string]string)
	for _, item := range req.Body.SetHeaders {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		if strings.TrimSpace(item.Value) == "" {
			item.Value = "empty"
		}
		// filter same key
		setheaders["set-header-"+item.Key] = item.Value
	}
	for k, v := range setheaders {
		configs = append(configs, &model.GwRuleConfig{
			RuleID: req.RuleID,
			Key:    k,
			Value:  v,
		})
	}

	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	if err := g.dbmanager.GwRuleConfigDaoTransactions(tx).DeleteByRuleID(req.RuleID); err != nil {
		tx.Rollback()
		return err
	}
	for _, cfg := range configs {
		if err := g.dbmanager.GwRuleConfigDaoTransactions(tx).AddModel(cfg); err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

//IPAndAvailablePort ip and advice available port
type IPAndAvailablePort struct {
	IP            string `json:"ip"`
	AvailablePort int    `json:"available_port"`
}

//GetGatewayIPs get all gateway node ips
func (g *GatewayAction) GetGatewayIPs() []IPAndAvailablePort {
	defaultAvailablePort, _ := g.GetAvailablePort("0.0.0.0")
	defaultIps := []IPAndAvailablePort{IPAndAvailablePort{
		IP:            "0.0.0.0",
		AvailablePort: defaultAvailablePort,
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	res, err := clientv3.NewKV(g.etcdCli).Get(ctx, "/rainbond/gateway/ips", clientv3.WithPrefix())
	if err != nil {
		return defaultIps
	}
	gatewayIps := []string{}
	for _, v := range res.Kvs {
		gatewayIps = append(gatewayIps, string(v.Value))
	}
	sort.Strings(gatewayIps)
	for _, v := range gatewayIps {
		availablePort, _ := g.GetAvailablePort(v)
		defaultIps = append(defaultIps, IPAndAvailablePort{
			IP:            v,
			AvailablePort: availablePort,
		})
	}
	return defaultIps
}
