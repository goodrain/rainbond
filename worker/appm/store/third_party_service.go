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

package store

import (
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/appm/conversion"
)

func (a *appRuntimeStore) initThirdPartyService() error {
	logrus.Debugf("begin initializing third-party services.")
	// TODO: list third party services that have open ports directly.
	svcs, err := a.dbmanager.TenantServiceDao().ListThirdPartyServices()
	if err != nil {
		logrus.Errorf("error listing third-party services: %v", err)
		return err
	}
	for _, svc := range svcs {
		// ignore service without open port.
		if !a.dbmanager.TenantServicesPortDao().HasOpenPort(svc.ServiceID) {
			continue
		}

		appService, err := conversion.InitCacheAppService(a.dbmanager, svc.ServiceID, "Rainbond")
		if err != nil {
			logrus.Errorf("error initializing cache app service: %v", err)
			return err
		}
		a.RegistAppService(appService)
		err = appm.ApplyOne(a.clientset, appService)
		if err != nil {
			logrus.Errorf("error applying rule: %v", err)
			return err
		}
	}
	return nil
}
