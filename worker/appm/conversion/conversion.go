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

package conversion

import (
	"fmt"

	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/componentdefinition"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
)

func init() {
	// core component conversion
	// convert config group to env secrets
	RegistConversion("TenantServiceConfigGroup", TenantServiceConfigGroup)
	//step1 conv service pod base info
	RegistConversion("TenantServiceVersion", TenantServiceVersion)
	//step2 conv service plugin
	RegistConversion("TenantServicePlugin", TenantServicePlugin)
	//step3 -
	RegistConversion("TenantServiceAutoscaler", TenantServiceAutoscaler)
	//step4 conv service monitor
	RegistConversion("TenantServiceMonitor", TenantServiceMonitor)
}

//Conversion conversion function
//Any application attribute implementation is similarly injected
type Conversion func(*v1.AppService, db.Manager) error

//CacheConversion conversion cache struct
type CacheConversion struct {
	Name       string
	Conversion Conversion
}

//conversionList conversion function list
var conversionList []CacheConversion

//RegistConversion regist conversion function list
func RegistConversion(name string, fun Conversion) {
	conversionList = append(conversionList, CacheConversion{Name: name, Conversion: fun})
}

//InitAppService init a app service
func InitAppService(dbmanager db.Manager, serviceID string, configs map[string]string, enableConversionList ...string) (*v1.AppService, error) {
	if configs == nil {
		configs = make(map[string]string)
	}

	appService := &v1.AppService{
		AppServiceBase: v1.AppServiceBase{
			ServiceID:      serviceID,
			ExtensionSet:   configs,
			GovernanceMode: model.GovernanceModeBuildInServiceMesh,
		},
		UpgradePatch: make(map[string][]byte, 2),
	}

	// setup governance mode
	app, err := dbmanager.ApplicationDao().GetByServiceID(serviceID)
	if err != nil && err != bcode.ErrApplicationNotFound {
		return nil, fmt.Errorf("get app based on service id(%s)", serviceID)
	}
	if app != nil {
		appService.AppServiceBase.GovernanceMode = app.GovernanceMode
		appService.AppServiceBase.K8sApp = app.K8sApp
	}
	if err := TenantServiceBase(appService, dbmanager); err != nil {
		logrus.Errorf("init component base config failure %s", err.Error())
		return nil, err
	}
	// all component can regist server.
	if err := TenantServiceRegist(appService, dbmanager); err != nil {
		logrus.Errorf("init component server regist config failure %s", err.Error())
		return nil, err
	}
	if appService.IsCustomComponent() {
		if err := componentdefinition.GetComponentDefinitionBuilder().BuildWorkloadResource(appService, dbmanager); err != nil {
			logrus.Errorf("init component by component definition build failure %s", err.Error())
			return nil, err
		}
		return appService, nil
	}
	for _, c := range conversionList {
		if len(enableConversionList) == 0 || util.StringArrayContains(enableConversionList, c.Name) {
			if err := c.Conversion(appService, dbmanager); err != nil {
				return nil, err
			}
		}
	}
	return appService, nil
}

//InitCacheAppService init cache app service.
//if store manager receive a kube model belong with service and not find in store,will create
func InitCacheAppService(dbm db.Manager, serviceID, creatorID string) (*v1.AppService, error) {
	appService := &v1.AppService{
		AppServiceBase: v1.AppServiceBase{
			ServiceID:      serviceID,
			CreaterID:      creatorID,
			ExtensionSet:   make(map[string]string),
			GovernanceMode: model.GovernanceModeBuildInServiceMesh,
		},
		UpgradePatch: make(map[string][]byte, 2),
	}

	// setup governance mode
	app, err := dbm.ApplicationDao().GetByServiceID(serviceID)
	if err != nil && err != bcode.ErrApplicationNotFound {
		return nil, fmt.Errorf("get app based on service id(%s)", serviceID)
	}
	if app != nil {
		appService.AppServiceBase.GovernanceMode = app.GovernanceMode
		appService.AppServiceBase.K8sApp = app.K8sApp
	}

	if err := TenantServiceBase(appService, dbm); err != nil {
		return nil, err
	}
	svc, err := dbm.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, err
	}
	if svc.Kind == model.ServiceKindThirdParty.String() {
		if err := TenantServiceRegist(appService, dbm); err != nil {
			return nil, err
		}
	}

	return appService, nil
}
