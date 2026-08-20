// Copyright (C) 2014-2024 Goodrain Co., Ltd.
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
	"errors"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

const (
	syncVolumeAppID               = "consumer-app-id"
	syncVolumeTenantID            = "tenant-id"
	syncVolumeConsumerComponentID = "consumer-component-id"
	syncVolumeProviderComponentID = "provider-component-id"
	syncVolumeName                = "shared-config"
	syncVolumeMountPath           = "/etc/example/config.yaml"
)

type syncVolumeRelationTestManager struct {
	db.Manager
	serviceDao       dbdao.TenantServiceDao
	volumeDao        dbdao.TenantServiceVolumeDao
	mountRelationDao dbdao.TenantServiceMountRelationDao
}

func (m syncVolumeRelationTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.serviceDao
}

func (m syncVolumeRelationTestManager) TenantServiceVolumeDao() dbdao.TenantServiceVolumeDao {
	return m.volumeDao
}

func (m syncVolumeRelationTestManager) TenantServiceMountRelationDaoTransactions(*gorm.DB) dbdao.TenantServiceMountRelationDao {
	return m.mountRelationDao
}

type syncVolumeRelationTenantServiceDao struct {
	dbdao.TenantServiceDao
	services            []*dbmodel.TenantServices
	externalServices    []*dbmodel.TenantServices
	externalServicesErr error
	requestedAppID      string
	requestedServiceIDs []string
}

func (d *syncVolumeRelationTenantServiceDao) ListByAppID(appID string) ([]*dbmodel.TenantServices, error) {
	d.requestedAppID = appID
	return d.services, nil
}

func (d *syncVolumeRelationTenantServiceDao) GetServiceByIDs(serviceIDs []string) ([]*dbmodel.TenantServices, error) {
	d.requestedServiceIDs = append([]string(nil), serviceIDs...)
	return d.externalServices, d.externalServicesErr
}

type syncVolumeRelationVolumeDao struct {
	dbdao.TenantServiceVolumeDao
	providerVolume *dbmodel.TenantServiceVolume
	requestedIDs   []string
}

func (d *syncVolumeRelationVolumeDao) ListVolumesByComponentIDs(componentIDs []string) ([]*dbmodel.TenantServiceVolume, error) {
	d.requestedIDs = append([]string(nil), componentIDs...)
	if d.providerVolume == nil {
		return nil, nil
	}
	for _, componentID := range componentIDs {
		if componentID == d.providerVolume.ServiceID {
			return []*dbmodel.TenantServiceVolume{d.providerVolume}, nil
		}
	}
	return nil, nil
}

type syncVolumeRelationMountDao struct {
	dbdao.TenantServiceMountRelationDao
	deletedComponentIDs []string
	createdRelations    []*dbmodel.TenantServiceMountRelation
}

func (d *syncVolumeRelationMountDao) DeleteByComponentIDs(componentIDs []string) error {
	d.deletedComponentIDs = append([]string(nil), componentIDs...)
	return nil
}

func (d *syncVolumeRelationMountDao) CreateOrUpdateVolumeRelsInBatch(relations []*dbmodel.TenantServiceMountRelation) error {
	d.createdRelations = append([]*dbmodel.TenantServiceMountRelation(nil), relations...)
	return nil
}

func setSyncVolumeRelationTestManager(t *testing.T, manager syncVolumeRelationTestManager) {
	t.Helper()
	originalManager := db.GetManager()
	db.SetTestManager(manager)
	t.Cleanup(func() {
		db.SetTestManager(originalManager)
	})
}

func syncVolumeRelationTestComponent() *apimodel.Component {
	return &apimodel.Component{
		ComponentBase: apimodel.ComponentBase{ComponentID: syncVolumeConsumerComponentID},
		VolumeRelations: []apimodel.VolumeRelation{
			{
				MountPath:        syncVolumeMountPath,
				DependServiceID:  syncVolumeProviderComponentID,
				DependVolumeName: syncVolumeName,
			},
		},
	}
}

// capability_id: rainbond.app-upgrade.cross-app-config-mount
func TestSyncComponentVolumeRelsPreservesCrossApplicationConfigFileMount(t *testing.T) {
	const hostPath = "/grdata/services/provider/shared-config"
	serviceDao := &syncVolumeRelationTenantServiceDao{
		services: []*dbmodel.TenantServices{{ServiceID: syncVolumeConsumerComponentID}},
		externalServices: []*dbmodel.TenantServices{
			{ServiceID: syncVolumeProviderComponentID, TenantID: syncVolumeTenantID},
		},
	}
	volumeDao := &syncVolumeRelationVolumeDao{
		providerVolume: &dbmodel.TenantServiceVolume{
			ServiceID:  syncVolumeProviderComponentID,
			VolumeName: syncVolumeName,
			HostPath:   hostPath,
			VolumeType: dbmodel.ConfigFileVolumeType.String(),
		},
	}
	mountDao := &syncVolumeRelationMountDao{}
	setSyncVolumeRelationTestManager(t, syncVolumeRelationTestManager{
		serviceDao:       serviceDao,
		volumeDao:        volumeDao,
		mountRelationDao: mountDao,
	})

	err := (&ServiceAction{}).SyncComponentVolumeRels(nil, &dbmodel.Application{
		AppID:    syncVolumeAppID,
		TenantID: syncVolumeTenantID,
	}, []*apimodel.Component{syncVolumeRelationTestComponent()})
	if err != nil {
		t.Fatalf("sync component volume relations: %v", err)
	}

	if serviceDao.requestedAppID != syncVolumeAppID {
		t.Fatalf("expected application lookup for %q, got %q", syncVolumeAppID, serviceDao.requestedAppID)
	}
	if !containsSyncVolumeString(serviceDao.requestedServiceIDs, syncVolumeProviderComponentID) {
		t.Fatalf("expected external provider lookup to include %q, got %v", syncVolumeProviderComponentID, serviceDao.requestedServiceIDs)
	}
	if !containsSyncVolumeString(volumeDao.requestedIDs, syncVolumeProviderComponentID) {
		t.Fatalf("expected volume lookup to include cross-application provider %q, got %v", syncVolumeProviderComponentID, volumeDao.requestedIDs)
	}
	if len(mountDao.deletedComponentIDs) != 1 || mountDao.deletedComponentIDs[0] != syncVolumeConsumerComponentID {
		t.Fatalf("expected existing relation deletion for %q, got %v", syncVolumeConsumerComponentID, mountDao.deletedComponentIDs)
	}
	if len(mountDao.createdRelations) != 1 {
		t.Fatalf("expected one recreated mount relation, got %d", len(mountDao.createdRelations))
	}
	relation := mountDao.createdRelations[0]
	if relation.TenantID != syncVolumeTenantID ||
		relation.ServiceID != syncVolumeConsumerComponentID ||
		relation.DependServiceID != syncVolumeProviderComponentID ||
		relation.VolumeName != syncVolumeName ||
		relation.VolumePath != syncVolumeMountPath ||
		relation.HostPath != hostPath ||
		relation.VolumeType != dbmodel.ConfigFileVolumeType.String() {
		t.Fatalf("recreated mount relation does not preserve the shared config-file mount: %#v", relation)
	}
}

// capability_id: rainbond.app-upgrade.cross-app-config-mount
func TestSyncComponentVolumeRelsRejectsInvalidExternalProviders(t *testing.T) {
	testCases := []struct {
		name             string
		externalServices []*dbmodel.TenantServices
	}{
		{
			name: "provider belongs to another tenant",
			externalServices: []*dbmodel.TenantServices{
				{ServiceID: syncVolumeProviderComponentID, TenantID: "other-tenant-id"},
			},
		},
		{
			name:             "provider does not exist",
			externalServices: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serviceDao := &syncVolumeRelationTenantServiceDao{
				services:         []*dbmodel.TenantServices{{ServiceID: syncVolumeConsumerComponentID}},
				externalServices: tc.externalServices,
			}
			volumeDao := &syncVolumeRelationVolumeDao{
				providerVolume: &dbmodel.TenantServiceVolume{
					ServiceID:  syncVolumeProviderComponentID,
					VolumeName: syncVolumeName,
					VolumeType: dbmodel.ConfigFileVolumeType.String(),
				},
			}
			mountDao := &syncVolumeRelationMountDao{}
			setSyncVolumeRelationTestManager(t, syncVolumeRelationTestManager{
				serviceDao:       serviceDao,
				volumeDao:        volumeDao,
				mountRelationDao: mountDao,
			})

			err := (&ServiceAction{}).SyncComponentVolumeRels(nil, &dbmodel.Application{
				AppID:    syncVolumeAppID,
				TenantID: syncVolumeTenantID,
			}, []*apimodel.Component{syncVolumeRelationTestComponent()})
			if err != nil {
				t.Fatalf("sync component volume relations: %v", err)
			}

			if !containsSyncVolumeString(serviceDao.requestedServiceIDs, syncVolumeProviderComponentID) {
				t.Fatalf("expected external provider lookup to include %q, got %v", syncVolumeProviderComponentID, serviceDao.requestedServiceIDs)
			}
			if containsSyncVolumeString(volumeDao.requestedIDs, syncVolumeProviderComponentID) {
				t.Fatalf("expected invalid external provider %q to be excluded from volume lookup, got %v", syncVolumeProviderComponentID, volumeDao.requestedIDs)
			}
			if len(mountDao.createdRelations) != 0 {
				t.Fatalf("expected no relation for invalid external provider, got %#v", mountDao.createdRelations)
			}
		})
	}
}

// capability_id: rainbond.app-upgrade.cross-app-config-mount
func TestSyncComponentVolumeRelsReturnsExternalProviderLookupError(t *testing.T) {
	expectedErr := errors.New("external provider lookup failed")
	serviceDao := &syncVolumeRelationTenantServiceDao{
		services:            []*dbmodel.TenantServices{{ServiceID: syncVolumeConsumerComponentID}},
		externalServicesErr: expectedErr,
	}
	setSyncVolumeRelationTestManager(t, syncVolumeRelationTestManager{serviceDao: serviceDao})

	err := (&ServiceAction{}).SyncComponentVolumeRels(nil, &dbmodel.Application{
		AppID:    syncVolumeAppID,
		TenantID: syncVolumeTenantID,
	}, []*apimodel.Component{syncVolumeRelationTestComponent()})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected external provider lookup error %v, got %v", expectedErr, err)
	}
}

func containsSyncVolumeString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
