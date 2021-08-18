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

package handler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ThirdPartyServiceHanlder handles business logic for all third-party services
type ThirdPartyServiceHanlder struct {
	logger    *logrus.Entry
	dbmanager db.Manager
	statusCli *client.AppRuntimeSyncClient
}

// Create3rdPartySvcHandler creates a new *ThirdPartyServiceHanlder.
func Create3rdPartySvcHandler(dbmanager db.Manager, statusCli *client.AppRuntimeSyncClient) *ThirdPartyServiceHanlder {
	return &ThirdPartyServiceHanlder{
		logger:    logrus.WithField("WHO", "ThirdPartyServiceHanlder"),
		dbmanager: dbmanager,
		statusCli: statusCli,
	}
}

// AddEndpoints adds endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) AddEndpoints(sid string, d *model.AddEndpiontsReq) error {
	address, port := convertAddressPort(d.Address)
	if port == 0 {
		//set default port by service port
		ports, _ := t.dbmanager.TenantServicesPortDao().GetPortsByServiceID(sid)
		if len(ports) > 0 {
			port = ports[0].ContainerPort
		}
	}
	ep := &dbmodel.Endpoint{
		UUID:      util.NewUUID(),
		ServiceID: sid,
		IP:        address,
		Port:      port,
	}
	if err := t.dbmanager.EndpointsDao().AddModel(ep); err != nil {
		return err
	}

	logrus.Debugf("add new endpoint[address: %s, port: %d]", address, port)
	t.statusCli.AddThirdPartyEndpoint(ep)
	return nil
}

// UpdEndpoints updates endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) UpdEndpoints(d *model.UpdEndpiontsReq) error {
	ep, err := t.dbmanager.EndpointsDao().GetByUUID(d.EpID)
	if err != nil {
		logrus.Warningf("EpID: %s; error getting endpoints: %v", d.EpID, err)
		return err
	}
	if d.Address != "" {
		address, port := convertAddressPort(d.Address)
		ep.IP = address
		ep.Port = port
	}
	if err := t.dbmanager.EndpointsDao().UpdateModel(ep); err != nil {
		return err
	}

	t.statusCli.UpdThirdPartyEndpoint(ep)

	return nil
}

func convertAddressPort(s string) (address string, port int) {
	prefix := ""
	if strings.HasPrefix(s, "https://") {
		s = strings.Split(s, "https://")[1]
		prefix = "https://"
	}
	if strings.HasPrefix(s, "http://") {
		s = strings.Split(s, "http://")[1]
		prefix = "http://"
	}

	if strings.Contains(s, ":") {
		sp := strings.Split(s, ":")
		address = prefix + sp[0]
		port, _ = strconv.Atoi(sp[1])
	} else {
		address = prefix + s
	}

	return address, port
}

// DelEndpoints deletes endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) DelEndpoints(epid, sid string) error {
	ep, err := t.dbmanager.EndpointsDao().GetByUUID(epid)
	if err != nil {
		logrus.Warningf("EpID: %s; error getting endpoints: %v", epid, err)
		return err
	}
	if err := t.dbmanager.EndpointsDao().DelByUUID(epid); err != nil {
		return err
	}
	t.statusCli.DelThirdPartyEndpoint(ep)

	return nil
}

// ListEndpoints lists third-party service endpoints.
func (t *ThirdPartyServiceHanlder) ListEndpoints(componentID string) ([]*model.ThirdEndpoint, error) {
	logger := t.logger.WithField("Method", "ListEndpoints").
		WithField("ComponentID", componentID)

	runtimeEndpoints, err := t.listRuntimeEndpoints(componentID)
	if err != nil {
		logger.Warning(err.Error())
	}

	staticEndpoints, err := t.listStaticEndpoints(componentID)
	if err != nil {
		staticEndpoints = map[string]*model.ThirdEndpoint{}
		logger.Warning(err.Error())
	}

	// Merge runtimeEndpoints with staticEndpoints
	for _, ep := range runtimeEndpoints {
		sep, ok := staticEndpoints[ep.EpID]
		if !ok {
			continue
		}
		ep.IsStatic = sep.IsStatic
		ep.Address = sep.Address
		delete(staticEndpoints, ep.EpID)
	}

	// Add offline static endpoints
	for _, ep := range staticEndpoints {
		runtimeEndpoints = append(runtimeEndpoints, ep)
	}

	sort.Sort(model.ThirdEndpoints(runtimeEndpoints))
	return runtimeEndpoints, nil
}

func (t *ThirdPartyServiceHanlder) listRuntimeEndpoints(componentID string) ([]*model.ThirdEndpoint, error) {
	runtimeEndpoints, err := t.statusCli.ListThirdPartyEndpoints(componentID)
	if err != nil {
		return nil, errors.Wrap(err, "list runtime third endpoints")
	}

	var endpoints []*model.ThirdEndpoint
	for _, item := range runtimeEndpoints.Items {
		endpoints = append(endpoints, &model.ThirdEndpoint{
			EpID:    item.Name,
			Address: item.Address,
			Status:  item.Status,
		})
	}
	return endpoints, nil
}

func (t *ThirdPartyServiceHanlder) listStaticEndpoints(componentID string) (map[string]*model.ThirdEndpoint, error) {
	staticEndpoints, err := t.dbmanager.EndpointsDao().List(componentID)
	if err != nil {
		return nil, errors.Wrap(err, "list static endpoints")
	}

	endpoints := make(map[string]*model.ThirdEndpoint)
	for _, item := range staticEndpoints {
		address := func(ip string, p int) string {
			if p != 0 {
				return fmt.Sprintf("%s:%d", ip, p)
			}
			return ip
		}(item.IP, item.Port)
		endpoints[item.UUID] = &model.ThirdEndpoint{
			EpID:     item.UUID,
			Address:  address,
			Status:   "-",
			IsStatic: true,
		}
	}
	return endpoints, nil
}
