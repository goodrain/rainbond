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
	"path"

	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	utilhttp "github.com/goodrain/rainbond/util/http"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

type tenant struct {
	regionImpl
	tenantName string
	prefix     string
}

//TenantInterface TenantInterface
type TenantInterface interface {
	Get() (*dbmodel.Tenants, *util.APIHandleError)
	List() ([]*dbmodel.Tenants, *util.APIHandleError)
	Delete() *util.APIHandleError
	Services(serviceAlias string) ServiceInterface
	// DefineSources(ss *api_model.SourceSpec) DefineSourcesInterface
	// DefineCloudAuth(gt *api_model.GetUserToken) DefineCloudAuthInterface
}

func (t *tenant) Get() (*dbmodel.Tenants, *util.APIHandleError) {
	var decode utilhttp.ResponseBody
	var tenant dbmodel.Tenants
	decode.Bean = &tenant
	code, err := t.DoRequest(t.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	return &tenant, nil
}
func (t *tenant) List() ([]*dbmodel.Tenants, *util.APIHandleError) {
	if t.tenantName != "" {
		return nil, util.CreateAPIHandleErrorf(400, "tenant name must be empty in this api")
	}
	var decode utilhttp.ResponseBody
	code, err := t.DoRequest(t.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if decode.Bean == nil {
		return nil, nil
	}
	bean, ok := decode.Bean.(map[string]interface{})
	if !ok {
		logrus.Warningf("list tenants; wrong data: %v", decode.Bean)
		return nil, nil
	}
	list, ok := bean["list"]
	if !ok {
		return nil, nil
	}
	var tenants []*dbmodel.Tenants
	if err := mapstructure.Decode(list, &tenants); err != nil {
		logrus.Errorf("map: %+v; error decoding to map to []*dbmodel.Tenants: %v", list, err)
		return nil, util.CreateAPIHandleError(500, err)
	}
	return tenants, nil
}
func (t *tenant) Delete() *util.APIHandleError {
	return nil
}
func (t *tenant) Services(serviceAlias string) ServiceInterface {
	return &services{
		prefix: path.Join(t.prefix, "services", serviceAlias),
		tenant: *t,
	}
}
