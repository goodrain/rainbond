package handler

import (
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

type volumeDependencyDeleteTestManager struct {
	db.Manager
	mountRelationDao dbdao.TenantServiceMountRelationDao
}

func (m volumeDependencyDeleteTestManager) TenantServiceMountRelationDao() dbdao.TenantServiceMountRelationDao {
	return m.mountRelationDao
}

type preciseVolumeDependencyDeleteDao struct {
	dbdao.TenantServiceMountRelationDao
	serviceID       string
	dependServiceID string
	volumeName      string
}

func (d *preciseVolumeDependencyDeleteDao) DeleteTenantServiceMountRelation(serviceID, dependServiceID, volumeName string) error {
	d.serviceID = serviceID
	d.dependServiceID = dependServiceID
	d.volumeName = volumeName
	return nil
}

// capability_id: rainbond.volume-dependency.precise-delete
func TestVolumeDependencyDeleteUsesConsumerProviderAndVolumeName(t *testing.T) {
	dao := &preciseVolumeDependencyDeleteDao{}
	originalManager := db.GetManager()
	db.SetTestManager(volumeDependencyDeleteTestManager{mountRelationDao: dao})
	t.Cleanup(func() { db.SetTestManager(originalManager) })

	relation := &dbmodel.TenantServiceMountRelation{
		ServiceID:       "consumer-service",
		DependServiceID: "provider-service",
		VolumeName:      "shared-config",
	}
	if apiErr := (&ServiceAction{}).VolumeDependency(relation, "delete"); apiErr != nil {
		t.Fatalf("delete volume dependency: %v", apiErr)
	}
	if dao.serviceID != relation.ServiceID || dao.dependServiceID != relation.DependServiceID || dao.volumeName != relation.VolumeName {
		t.Fatalf("expected exact delete (%q, %q, %q), got (%q, %q, %q)",
			relation.ServiceID, relation.DependServiceID, relation.VolumeName,
			dao.serviceID, dao.dependServiceID, dao.volumeName)
	}
}
