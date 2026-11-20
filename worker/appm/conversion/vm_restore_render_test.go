package conversion

import (
	"testing"

	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
)

type renderVolumeDAO struct{ dbdao.TenantServiceVolumeDao }

func (renderVolumeDAO) GetTenantServiceVolumesByServiceID(string) ([]*dbmodel.TenantServiceVolume, error) {
	return []*dbmodel.TenantServiceVolume{}, nil
}
func (renderVolumeDAO) GetVolumeByServiceIDAndName(string, string) (*dbmodel.TenantServiceVolume, error) {
	return nil, gorm.ErrRecordNotFound
}

type renderMountDAO struct {
	dbdao.TenantServiceMountRelationDao
	deleted int
}

func (*renderMountDAO) GetTenantServiceMountRelationsByService(string) ([]*dbmodel.TenantServiceMountRelation, error) {
	return []*dbmodel.TenantServiceMountRelation{{ServiceID: "service-1", DependServiceID: "dependency", VolumeName: "missing"}}, nil
}
func (d *renderMountDAO) DElTenantServiceMountRelationByDepService(string, string) error {
	d.deleted++
	return nil
}

// capability_id: rainbond.vm-restore.readonly-rollback-render
func TestRestoreRenderDoesNotDeleteMissingDependencyMetadata(t *testing.T) {
	as := newVMAppServiceForImportValidationTest()
	as.RestoreRender = true
	mounts := &renderMountDAO{}
	manager := vmImportConversionManager{volumeDao: renderVolumeDAO{}, mountDao: mounts, attributeDao: vmImportAttributeDao{}}
	_, err := createVolumes(as, &dbmodel.VersionInfo{ServiceID: as.ServiceID}, nil, nil, manager)
	if err != vmrestore.ErrReferenceUnavailable || mounts.deleted != 0 {
		t.Fatal("rollback render changed dependency metadata instead of rejecting an unavailable source", err, mounts.deleted)
	}
}
