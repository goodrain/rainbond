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
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/rafrombrc/gomock/gomock"
	"testing"
)

func TestTenantServiceBase(t *testing.T) {
	t.Run("third-party service", func(t *testing.T) {
		as := &v1.AppService{}
		as.ServiceID = util.NewUUID()
		as.TenantID = util.NewUUID()
		as.TenantName = "abcdefg"

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		dbm := db.NewMockManager(ctrl)
		// TenantServiceDao
		tenantServiceDao := dao.NewMockTenantServiceDao(ctrl)
		tenantService := &model.TenantServices{
			TenantID: as.TenantID,
			ServiceID: as.ServiceID,
			Kind: model.ServiceKindThirdParty.String(),
		}
		tenantServiceDao.EXPECT().GetServiceByID(as.ServiceID).Return(tenantService, nil)
		dbm.EXPECT().TenantServiceDao().Return(tenantServiceDao)
		// TenantDao
		tenantDao := dao.NewMockTenantDao(ctrl)
		tenant := &model.Tenants{
			UUID: as.TenantID,
			Name: as.TenantName,
		}
		tenantDao.EXPECT().GetTenantByUUID(as.TenantID).Return(tenant, nil)
		dbm.EXPECT().TenantDao().Return(tenantDao)
		if err := TenantServiceBase(as, dbm); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}
