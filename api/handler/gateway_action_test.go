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
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	"github.com/rafrombrc/gomock/gomock"
	"testing"
)

func TestGatewayAction_TCPAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dbmanager := db.NewMockManager(ctrl)

	ipPortDao := dao.NewMockIPPortDao(ctrl)
	ipport := &model.IPPort{
		IP: "172.16.0.106",
		Port: 8888,
	}
	ipPortDao.EXPECT().GetIPPortByIPAndPort("172.16.0.106", 8888).Return(ipport, nil)
	dbmanager.EXPECT().IPPortDao().Return(ipPortDao)

	g := GatewayAction{
		dbmanager: dbmanager,
	}
	if g.TCPAvailable("172.16.0.106", 8888) {
		t.Errorf("expected false for tcp available, but returned true")
	}
}
