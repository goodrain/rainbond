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
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/appm/volume"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTenantServiceVersion(t *testing.T) {
	var as v1.AppService
	TenantServiceVersion(&as, nil)
}

func TestConvertRulesToEnvs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dbmanager := db.NewMockManager(ctrl)

	as := &v1.AppService{}
	as.ServiceID = "dummy service id"
	as.TenantName = "dummy tenant name"
	as.ServiceAlias = "dummy service alias"

	httpRuleDao := dao.NewMockHTTPRuleDao(ctrl)
	httpRuleDao.EXPECT().GetHTTPRuleByServiceIDAndContainerPort(as.ServiceID, 0).Return(nil, nil)
	dbmanager.EXPECT().HTTPRuleDao().Return(httpRuleDao)

	port := &model.TenantServicesPort{
		TenantID:       "dummy tenant id",
		ServiceID:      as.ServiceID,
		ContainerPort:  0,
		Protocol:       "http",
		PortAlias:      "GRD835895000",
		IsInnerService: func() *bool { b := false; return &b }(),
		IsOuterService: func() *bool { b := true; return &b }(),
	}

	renvs := convertRulesToEnvs(as, dbmanager, []*model.TenantServicesPort{port})
	if len(renvs) > 0 {
		t.Errorf("Expected 0 for the length rule envs, but return %d", len(renvs))
	}
}

func TestCreateVolume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	db.CreateManager(config.Config{DBType: "mysql", MysqlConnectionInfo: "oc6Poh:noot6Mea@tcp(192.168.2.203:3306)/region"})
	dbmanager := db.GetManager()

	as := &v1.AppService{}
	as.ServiceID = "dummy service id"
	as.TenantName = "dummy tenant name"
	as.ServiceAlias = "dummy service alias"
	var replicas int32
	as.SetStatefulSet(&appv1.StatefulSet{Spec: appv1.StatefulSetSpec{Replicas: &replicas, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "version"}}}}})

	serviceVolume, err := db.GetManager().TenantServiceVolumeDao().GetVolumeByID(25)
	if err != nil {
		t.Log(err)
		return
	}
	version := &model.VersionInfo{}

	vol := volume.NewVolumeManager(as, serviceVolume, nil, version, nil, nil, dbmanager)
	var define = &volume.Define{}
	vol.CreateVolume(define)
}
func TestFoobar(t *testing.T) {
	memory := 64
	cpuRequest, cpuLimit := int64(memory)/128*30, int64(memory)/128*80
	t.Errorf("request: %d; limit: %d", cpuRequest, cpuLimit)
}
