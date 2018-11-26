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
	"fmt"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/jinzhu/gorm"
	"os"
	"strconv"
)

// GatewayAction -
type GatewayAction struct {
	dbmanager db.Manager
}

//CreateGatewayManager creates gateway manager.
func CreateGatewayManager(dbmanager db.Manager) *GatewayAction {
	return &GatewayAction{
		dbmanager: dbmanager,
	}
}

// AddHTTPRule adds http rule to db if it doesn't exists.
func (g *GatewayAction) AddHTTPRule(req *apimodel.AddHTTPRuleStruct) error {
	httpRule := &model.HTTPRule{
		UUID:          req.HTTPRuleID,
		ServiceID:     req.ServiceID,
		ContainerPort: req.ContainerPort,
		Domain:        req.Domain,
		Path:          req.Path,
		Header:        req.Header,
		Cookie:        req.Cookie,
		IP:            req.IP,
		CertificateID: req.CertificateID,
	}

	// begin transaction
	tx := db.GetManager().Begin()
	if err := db.GetManager().HttpRuleDaoTransactions(tx).AddModel(httpRule); err != nil {
		tx.Rollback()
		return err
	}

	cert := &model.Certificate{
		UUID:            req.CertificateID,
		CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
		Certificate:     req.Certificate,
		PrivateKey:      req.PrivateKey,
	}
	if err := db.GetManager().CertificateDaoTransactions(tx).AddModel(cert); err != nil {
		tx.Rollback()
		return err
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
			return err
		}
	}

	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// UpdateHTTPRule updates http rule
func (g *GatewayAction) UpdateHTTPRule(req *apimodel.UpdateHTTPRuleStruct) error {
	tx := db.GetManager().Begin()
	rule, err := g.dbmanager.HttpRuleDaoTransactions(tx).GetHttpRuleByID(req.HTTPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if rule == nil {
		tx.Rollback()
		return fmt.Errorf("HTTPRule dosen't exist based on uuid(%s)", req.HTTPRuleID)
	}
	if rule.CertificateID != "" {
		// delete old Certificate
		if err := g.dbmanager.CertificateDaoTransactions(tx).DeleteCertificateByID(rule.CertificateID); err != nil {
			tx.Rollback()
			return err
		}
		// add new certificate
		cert := &model.Certificate{
			UUID:            req.CertificateID,
			CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
			Certificate:     req.Certificate,
			PrivateKey:      req.PrivateKey,
		}
		if err := g.dbmanager.CertificateDaoTransactions(tx).AddModel(cert); err != nil {
			tx.Rollback()
			return err
		}
		rule.CertificateID = req.CertificateID
	}
	if len(req.RuleExtensions) > 0 {
		// delete old RuleExtensions
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
			tx.Rollback()
			return err
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
				return err
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
	if req.Path != "" {
		rule.Path = req.Path
	}
	if req.Header != "" {
		rule.Header = req.Header
	}
	if req.Cookie != "" {
		rule.Cookie = req.Cookie
	}
	if req.IP != "" {
		rule.IP = req.IP
	}
	if err := db.GetManager().HttpRuleDaoTransactions(tx).UpdateModel(rule); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// DeleteHTTPRule deletes http rule, including certificate and rule extensions
func (g *GatewayAction) DeleteHTTPRule(req *apimodel.DeleteHTTPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	// delete http rule
	httpRule, err := g.dbmanager.HttpRuleDaoTransactions(tx).GetHttpRuleByID(req.HTTPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := g.dbmanager.HttpRuleDaoTransactions(tx).DeleteHttpRuleByID(httpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// delete certificate
	if err := g.dbmanager.CertificateDaoTransactions(tx).DeleteCertificateByID(httpRule.CertificateID); err != nil {
		tx.Rollback()
		return err
	}
	// delete rule extension
	if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(httpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		return err
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
func (g *GatewayAction) AddTCPRule(req *apimodel.AddTCPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	// add port
	port := &model.TenantServiceLBMappingPort{
		ServiceID:     req.ServiceID,
		Port:          req.Port,
		ContainerPort: req.ContainerPort,
	}
	err := g.dbmanager.TenantServiceLBMappingPortDaoTransactions(tx).AddModel(port)
	if err != nil {
		tx.Rollback()
		return err
	}
	// add tcp rule
	tcpRule := &model.TCPRule{
		UUID:          req.TCPRuleID,
		ServiceID:     req.ServiceID,
		ContainerPort: req.ContainerPort,
		IP:            req.IP,
		Port:          req.Port,
	}
	if err := g.dbmanager.TcpRuleDaoTransactions(tx).AddModel(tcpRule); err != nil {
		tx.Rollback()
		return err
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
			return err
		}
	}

	// end transaction
	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

// UpdateTCPRule updates a tcp rule
func (g *GatewayAction) UpdateTCPRule(req *apimodel.UpdateTCPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	// get old tcp rule
	tcpRule, err := g.dbmanager.TcpRuleDaoTransactions(tx).GetTcpRuleByID(req.TCPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if len(req.RuleExtensions) > 0 {
		// delete old rule extensions
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(tcpRule.UUID); err != nil {
			tx.Rollback()
			return err
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
				return err
			}
		}
	}
	// update tcp rule
	if req.ServiceID != "" {
		tcpRule.ServiceID = req.ServiceID
	}
	if req.ContainerPort != 0 {
		tcpRule.ContainerPort = req.ContainerPort
	}
	if req.IP != "" {
		tcpRule.IP = req.IP
	}
	if req.Port > 20000 {
		// get old port
		port, err := g.dbmanager.TenantServiceLBMappingPortDaoTransactions(tx).GetLBMappingPortByServiceIDAndPort(
			tcpRule.ServiceID, tcpRule.Port)
		if err != nil {
			tx.Rollback()
			return err
		}
		// check
		// update port
		port.Port = req.Port
		if err := g.dbmanager.TenantServiceLBMappingPortDaoTransactions(tx).UpdateModel(port); err != nil {
			tx.Rollback()
			return err
		}
		tcpRule.Port = req.Port
	}
	if err := g.dbmanager.TcpRuleDaoTransactions(tx).UpdateModel(tcpRule); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// DeleteTCPRule deletes a tcp rule
func (g *GatewayAction) DeleteTCPRule(req *apimodel.DeleteTCPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	tcpRule, err := db.GetManager().TcpRuleDaoTransactions(tx).GetTcpRuleByID(req.TCPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	// delete rule extensions
	if err := db.GetManager().RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(tcpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// delete tcp rule
	if err := db.GetManager().TcpRuleDaoTransactions(tx).DeleteTcpRule(tcpRule); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
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
func (g *GatewayAction) GetAvailablePort() (int, error) {
	mapPorts, err := g.dbmanager.TenantServiceLBMappingPortDao().GetLBPortsASC()
	if err != nil {
		return 0, err
	}
	var ports []int
	for _, p := range mapPorts {
		ports = append(ports, p.Port)
	}
	maxPort, _ := strconv.Atoi(os.Getenv("MIN_LB_PORT"))
	minPort, _ := strconv.Atoi(os.Getenv("MAX_LB_PORT"))
	if minPort == 0 {
		minPort = 20001
	}
	if maxPort == 0 {
		maxPort = 35000
	}
	var maxUsePort int
	if len(ports) > 0 {
		maxUsePort = ports[len(ports)-1]
	} else {
		maxUsePort = 20001
	}
	//顺序分配端口
	selectPort := maxUsePort + 1
	if selectPort <= maxPort {
		return selectPort, nil
	}
	//捡漏以前端口
	selectPort = minPort
	for _, p := range ports {
		if p == selectPort {
			selectPort = selectPort + 1
			continue
		}
		if p > selectPort {
			return selectPort, nil
		}
		selectPort = selectPort + 1
	}
	if selectPort <= maxPort {
		return selectPort, nil
	}
	return 0, fmt.Errorf("no more lb port can be use,max port is %d", maxPort)
}

// PortExists returns if the port exists
func (g *GatewayAction) PortExists(port int) bool {
	return g.dbmanager.TenantServiceLBMappingPortDao().PortExists(port)
}
