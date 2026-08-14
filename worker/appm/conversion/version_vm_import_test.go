package conversion

import (
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/appm/volume"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

type vmImportConversionManager struct {
	db.Manager
	volumeDao    dbdao.TenantServiceVolumeDao
	mountDao     dbdao.TenantServiceMountRelationDao
	attributeDao dbdao.ComponentK8sAttributeDao
}

func (m vmImportConversionManager) TenantServiceVolumeDao() dbdao.TenantServiceVolumeDao {
	return m.volumeDao
}

func (m vmImportConversionManager) TenantServiceMountRelationDao() dbdao.TenantServiceMountRelationDao {
	return m.mountDao
}

func (m vmImportConversionManager) ComponentK8sAttributeDao() dbdao.ComponentK8sAttributeDao {
	return m.attributeDao
}

type vmImportVolumeDao struct {
	dbdao.TenantServiceVolumeDao
	volumes []*dbmodel.TenantServiceVolume
}

func (d vmImportVolumeDao) GetTenantServiceVolumesByServiceID(string) ([]*dbmodel.TenantServiceVolume, error) {
	return d.volumes, nil
}

type vmImportMountDao struct {
	dbdao.TenantServiceMountRelationDao
}

func (vmImportMountDao) GetTenantServiceMountRelationsByService(string) ([]*dbmodel.TenantServiceMountRelation, error) {
	return nil, nil
}

type vmImportAttributeDao struct {
	dbdao.ComponentK8sAttributeDao
	attribute *dbmodel.ComponentK8sAttributes
}

func (d vmImportAttributeDao) GetByComponentIDAndName(_, _ string) (*dbmodel.ComponentK8sAttributes, error) {
	return d.attribute, nil
}

func newVMAppServiceForImportValidationTest() *typesv1.AppService {
	as := &typesv1.AppService{
		AppServiceBase: typesv1.AppServiceBase{ServiceID: "service-1", ServiceAlias: "demo-vm"},
	}
	as.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	as.SetVirtualMachine(&kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-vm"},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}},
			},
		},
	})
	return as
}

// capability_id: rainbond.vm-import.validation-aborts-vm-definition
func TestCreateVolumesRejectsInvalidVMRegistryImport(t *testing.T) {
	as := newVMAppServiceForImportValidationTest()
	manager := vmImportConversionManager{
		volumeDao: vmImportVolumeDao{volumes: []*dbmodel.TenantServiceVolume{{
			Model:          dbmodel.Model{ID: 11},
			ServiceID:      "service-1",
			VolumeName:     "disk",
			VolumePath:     "/disk",
			VolumeType:     "nfs-storage",
			AccessMode:     "RWX",
			VolumeCapacity: 20,
		}}},
		mountDao: vmImportMountDao{},
		attributeDao: vmImportAttributeDao{attribute: &dbmodel.ComponentK8sAttributes{
			AttributeValue: `{"disk":{"volume_name":"disk","image_url":"ceshi:DBServer(TongYong)","source_type":"registry"}}`,
		}},
	}

	_, err := createVolumes(as, &dbmodel.VersionInfo{ServiceID: "service-1"}, nil, nil, manager)
	if err == nil {
		t.Fatal("expected invalid VM registry import to abort VM definition creation")
	}
	if !strings.Contains(err.Error(), "invalid VM registry image reference") {
		t.Fatalf("expected invalid registry error, got %v", err)
	}
	if !errors.Is(err, volume.ErrInvalidVMRegistryImport) {
		t.Fatalf("expected invalid VM registry import sentinel, got %v", err)
	}
	if len(as.GetClaims()) != 0 || len(as.GetClaimsManually()) != 0 {
		t.Fatalf("expected aborted VM definition to leave claims untouched, claims=%#v manualClaims=%#v", as.GetClaims(), as.GetClaimsManually())
	}
}
