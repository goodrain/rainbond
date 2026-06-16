package volume

import (
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	appmtypes "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

type volumeManagerStub struct {
	db.Manager
	configFileDao dbdao.TenantServiceConfigFileDao
	volumeDao     dbdao.TenantServiceVolumeDao
}

func (m volumeManagerStub) TenantServiceConfigFileDao() dbdao.TenantServiceConfigFileDao {
	return m.configFileDao
}

func (m volumeManagerStub) TenantServiceVolumeDao() dbdao.TenantServiceVolumeDao {
	return m.volumeDao
}

type tenantServiceVolumeDaoStub struct {
	dbdao.TenantServiceVolumeDao
	volume *dbmodel.TenantServiceVolume
}

func (t tenantServiceVolumeDaoStub) GetVolumeByServiceIDAndName(serviceID, name string) (*dbmodel.TenantServiceVolume, error) {
	return t.volume, nil
}

type tenantServiceConfigFileDaoStub struct {
	file *dbmodel.TenantServiceConfigFile
}

func (t tenantServiceConfigFileDaoStub) AddModel(dbmodel.Interface) error {
	return nil
}

func (t tenantServiceConfigFileDaoStub) UpdateModel(dbmodel.Interface) error {
	return nil
}

func (t tenantServiceConfigFileDaoStub) DeleteModel(interface{}, ...interface{}) error {
	return nil
}

func (t tenantServiceConfigFileDaoStub) GetConfigFileByServiceID(serviceID string) ([]*dbmodel.TenantServiceConfigFile, error) {
	return nil, nil
}

func (t tenantServiceConfigFileDaoStub) GetByVolumeName(sid, volumeName string) (*dbmodel.TenantServiceConfigFile, error) {
	return t.file, nil
}

func (t tenantServiceConfigFileDaoStub) DelByVolumeID(sid string, volumeName string) error {
	return nil
}

func (t tenantServiceConfigFileDaoStub) DelByServiceID(sid string) error {
	return nil
}

func (t tenantServiceConfigFileDaoStub) DeleteByComponentIDs(componentIDs []string) error {
	return nil
}

func (t tenantServiceConfigFileDaoStub) CreateOrUpdateConfigFilesInBatch(configFiles []*dbmodel.TenantServiceConfigFile) error {
	return nil
}

func newVMAppServiceForVolumeTest() *appmtypes.AppService {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	}
	as := &appmtypes.AppService{
		AppServiceBase: appmtypes.AppServiceBase{
			ServiceID:    "service-1",
			ServiceAlias: "demo-vm",
			TenantID:     "tenant-1",
			AppID:        "app-1",
		},
	}
	as.SetTenant(namespace)
	as.SetVirtualMachine(&kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-vm",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
		},
	})
	return as
}

func newDeploymentAppServiceForVolumeTest() *appmtypes.AppService {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	}
	as := &appmtypes.AppService{
		AppServiceBase: appmtypes.AppServiceBase{
			ServiceID:    "service-1",
			ServiceAlias: "demo-web",
			TenantID:     "tenant-1",
			AppID:        "app-1",
		},
	}
	as.SetTenant(namespace)
	return as
}

// capability_id: rainbond.config-file.k8s-volume-name-safe
func TestConfigFileVolumeCreateVolumeForDeploymentUsesK8sSafeVolumeName(t *testing.T) {
	as := newDeploymentAppServiceForVolumeTest()
	serviceVolume := &dbmodel.TenantServiceVolume{
		Model:      dbmodel.Model{ID: 5},
		ServiceID:  "service-1",
		VolumeName: "web.conf",
		VolumePath: "/etc/nginx/conf.d/web.conf",
		VolumeType: "config-file",
	}

	manager := volumeManagerStub{configFileDao: tenantServiceConfigFileDaoStub{
		file: &dbmodel.TenantServiceConfigFile{
			Model:       dbmodel.Model{CreatedAt: time.Now()},
			ServiceID:   "service-1",
			VolumeName:  "web.conf",
			FileContent: "server {}\n",
		},
	}}

	vol := NewVolumeManager(as, serviceVolume, nil, nil, nil, nil, manager, false)
	configVolume, ok := vol.(*ConfigFileVolume)
	if !ok {
		t.Fatalf("expected config-file volume to use ConfigFileVolume, got %T", vol)
	}

	define := &Define{as: as}
	if err := configVolume.CreateVolume(define); err != nil {
		t.Fatalf("create deployment config-file volume: %v", err)
	}

	if len(define.GetVolumes()) != 1 || len(define.GetVolumeMounts()) != 1 {
		t.Fatalf("expected one volume and one mount, got volumes=%#v mounts=%#v", define.GetVolumes(), define.GetVolumeMounts())
	}
	volumeName := define.GetVolumes()[0].Name
	if strings.Contains(volumeName, ".") {
		t.Fatalf("expected k8s volume name to avoid dots for config file volume, got %q", volumeName)
	}
	if define.GetVolumeMounts()[0].Name != volumeName {
		t.Fatalf("expected volume mount name %q to match volume name %q", define.GetVolumeMounts()[0].Name, volumeName)
	}
	if define.GetVolumeMounts()[0].MountPath != "/etc/nginx/conf.d/web.conf" {
		t.Fatalf("expected mount path to preserve file path, got %q", define.GetVolumeMounts()[0].MountPath)
	}
	if define.GetVolumeMounts()[0].SubPath != "web.conf" {
		t.Fatalf("expected subPath to preserve file name, got %q", define.GetVolumeMounts()[0].SubPath)
	}
	if len(as.GetConfigMaps()) != 1 || as.GetConfigMaps()[0].Data["web.conf"] != "server {}\n" {
		t.Fatalf("expected configmap to preserve web.conf content, got %#v", as.GetConfigMaps())
	}
}

// capability_id: rainbond.config-file.k8s-volume-name-safe
func TestConfigFileVolumeCreateDependVolumeForDeploymentUsesK8sSafeVolumeName(t *testing.T) {
	as := newDeploymentAppServiceForVolumeTest()
	depVolume := &dbmodel.TenantServiceVolume{
		Model:      dbmodel.Model{ID: 6},
		ServiceID:  "dep-service-1",
		VolumeName: "web.conf",
		VolumePath: "/source/web.conf",
		VolumeType: "config-file",
	}
	mountRelation := &dbmodel.TenantServiceMountRelation{
		Model:           dbmodel.Model{ID: 7},
		ServiceID:       "service-1",
		DependServiceID: "dep-service-1",
		VolumeName:      "web.conf",
		VolumePath:      "/etc/nginx/conf.d/web.conf",
		VolumeType:      "config-file",
	}
	manager := volumeManagerStub{
		volumeDao: tenantServiceVolumeDaoStub{volume: depVolume},
		configFileDao: tenantServiceConfigFileDaoStub{
			file: &dbmodel.TenantServiceConfigFile{
				Model:       dbmodel.Model{CreatedAt: time.Now()},
				ServiceID:   "dep-service-1",
				VolumeName:  "web.conf",
				FileContent: "server {}\n",
			},
		},
	}

	vol := NewVolumeManager(as, depVolume, mountRelation, nil, nil, nil, manager, true)
	configVolume, ok := vol.(*ConfigFileVolume)
	if !ok {
		t.Fatalf("expected dependent config-file volume to use ConfigFileVolume, got %T", vol)
	}

	define := &Define{as: as}
	if err := configVolume.CreateDependVolume(define); err != nil {
		t.Fatalf("create dependent deployment config-file volume: %v", err)
	}

	if len(define.GetVolumes()) != 1 || len(define.GetVolumeMounts()) != 1 {
		t.Fatalf("expected one volume and one mount, got volumes=%#v mounts=%#v", define.GetVolumes(), define.GetVolumeMounts())
	}
	volumeName := define.GetVolumes()[0].Name
	if strings.Contains(volumeName, ".") {
		t.Fatalf("expected k8s volume name to avoid dots for dependent config file volume, got %q", volumeName)
	}
	if define.GetVolumeMounts()[0].Name != volumeName {
		t.Fatalf("expected volume mount name %q to match volume name %q", define.GetVolumeMounts()[0].Name, volumeName)
	}
	if define.GetVolumeMounts()[0].MountPath != "/etc/nginx/conf.d/web.conf" {
		t.Fatalf("expected dependent mount path to preserve file path, got %q", define.GetVolumeMounts()[0].MountPath)
	}
	if define.GetVolumeMounts()[0].SubPath != "web.conf" {
		t.Fatalf("expected dependent subPath to preserve file name, got %q", define.GetVolumeMounts()[0].SubPath)
	}
}

// capability_id: rainbond.vm-volume-selected-storage-class
func TestNewVolumeManagerUsesSelectedStorageClassForVMDisks(t *testing.T) {
	as := newVMAppServiceForVolumeTest()
	serviceVolume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 1},
		ServiceID:      "service-1",
		VolumeName:     "disk",
		VolumePath:     "/disk",
		VolumeType:     "nfs-storage",
		AccessMode:     "RWX",
		VolumeCapacity: 20,
	}

	manager := NewVolumeManager(as, serviceVolume, nil, nil, nil, nil, nil, false)
	shareVolume, ok := manager.(*ShareFileVolume)
	if !ok {
		t.Fatalf("expected vm storage volume to use ShareFileVolume, got %T", manager)
	}

	define := &Define{as: as}
	if err := shareVolume.CreateVolume(define); err != nil {
		t.Fatalf("create vm volume: %v", err)
	}

	claims := as.GetClaims()
	if len(claims) != 1 {
		t.Fatalf("expected exactly one claim, got %d", len(claims))
	}
	if claims[0].Spec.StorageClassName == nil {
		t.Fatal("expected vm claim to keep selected storage class")
	}
	if *claims[0].Spec.StorageClassName != "nfs-storage" {
		t.Fatalf("expected storage class nfs-storage, got %q", *claims[0].Spec.StorageClassName)
	}
	if len(claims[0].Spec.AccessModes) != 1 || claims[0].Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Fatalf("expected RWX access mode, got %#v", claims[0].Spec.AccessModes)
	}
	if len(define.GetVMDataVolumeTemplates()) != 1 {
		t.Fatalf("expected vm blank root disk to create one data volume template, got %d", len(define.GetVMDataVolumeTemplates()))
	}
	if len(define.vmDisk) != 1 || define.vmDisk[0].DiskDevice.Disk == nil {
		t.Fatalf("expected root vm volume to create one disk target, got %#v", define.vmDisk)
	}
	if define.vmDisk[0].DiskDevice.Disk.Bus != kubevirtv1.DiskBusSATA {
		t.Fatalf("expected root vm disk to keep sata bus, got %q", define.vmDisk[0].DiskDevice.Disk.Bus)
	}
}

// capability_id: rainbond.vm-volume-vm-file-backward-compatible
func TestShareFileVolumeVMStorageClassFallsBackToLocalPathForLegacyVMFile(t *testing.T) {
	as := newVMAppServiceForVolumeTest()
	serviceVolume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 2},
		ServiceID:      "service-1",
		VolumeName:     "disk",
		VolumePath:     "/disk",
		VolumeType:     dbmodel.VMVolumeType.String(),
		AccessMode:     "RWO",
		VolumeCapacity: 10,
	}

	manager := NewVolumeManager(as, serviceVolume, nil, nil, nil, nil, nil, false)
	shareVolume, ok := manager.(*ShareFileVolume)
	if !ok {
		t.Fatalf("expected legacy vm-file volume to use ShareFileVolume, got %T", manager)
	}

	define := &Define{as: as}
	if err := shareVolume.CreateVolume(define); err != nil {
		t.Fatalf("create legacy vm volume: %v", err)
	}

	claims := as.GetClaims()
	if len(claims) != 1 {
		t.Fatalf("expected exactly one claim, got %d", len(claims))
	}
	if claims[0].Spec.StorageClassName == nil || *claims[0].Spec.StorageClassName != "local-path" {
		t.Fatalf("expected legacy vm-file to keep local-path fallback, got %#v", claims[0].Spec.StorageClassName)
	}
}

func TestShareFileVolumeCreateVolumeTreatsIndexedDiskPathAsDiskDevice(t *testing.T) {
	as := newVMAppServiceForVolumeTest()
	serviceVolume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 3},
		ServiceID:      "service-1",
		VolumeName:     "data-1",
		VolumePath:     "/disk-1",
		VolumeType:     "nfs-storage",
		AccessMode:     "RWX",
		VolumeCapacity: 20,
	}

	manager := NewVolumeManager(as, serviceVolume, nil, nil, nil, nil, nil, false)
	shareVolume, ok := manager.(*ShareFileVolume)
	if !ok {
		t.Fatalf("expected indexed vm disk volume to use ShareFileVolume, got %T", manager)
	}

	define := &Define{as: as}
	if err := shareVolume.CreateVolume(define); err != nil {
		t.Fatalf("create indexed vm volume: %v", err)
	}

	if len(define.vmDisk) != 1 {
		t.Fatalf("expected exactly one vm disk, got %d", len(define.vmDisk))
	}
	if define.vmDisk[0].DiskDevice.Disk == nil {
		t.Fatalf("expected indexed vm disk path to keep disk target, got %#v", define.vmDisk[0].DiskDevice)
	}
	if define.vmDisk[0].DiskDevice.Disk.Bus != kubevirtv1.DiskBusSCSI {
		t.Fatalf("expected indexed vm disk path to use scsi bus for hotplug-safe data disks, got %q", define.vmDisk[0].DiskDevice.Disk.Bus)
	}
	if len(define.GetVMDataVolumeTemplates()) != 1 {
		t.Fatalf("expected indexed vm disk path to create one data volume template, got %d", len(define.GetVMDataVolumeTemplates()))
	}
}

// capability_id: rainbond.vm-config-file-injected-as-configmap-volume
func TestConfigFileVolumeCreateVolumeForVMBuildsGuestVisibleConfigDisk(t *testing.T) {
	as := newVMAppServiceForVolumeTest()
	serviceVolume := &dbmodel.TenantServiceVolume{
		Model:      dbmodel.Model{ID: 4},
		ServiceID:  "service-1",
		VolumeName: "rainbond-env-file",
		VolumePath: "/rainbond/env/rainbond.env",
		VolumeType: "config-file",
	}

	manager := volumeManagerStub{configFileDao: tenantServiceConfigFileDaoStub{
		file: &dbmodel.TenantServiceConfigFile{
			Model:       dbmodel.Model{CreatedAt: time.Now()},
			ServiceID:   "service-1",
			VolumeName:  "rainbond-env-file",
			FileContent: "DEMO_HOST=${DEMO_HOST}\n",
		},
	}}

	vol := NewVolumeManager(as, serviceVolume, nil, nil, []corev1.EnvVar{
		{Name: "DEMO_HOST", Value: "demo-service"},
	}, nil, manager, false)
	configVolume, ok := vol.(*ConfigFileVolume)
	if !ok {
		t.Fatalf("expected config-file volume to use ConfigFileVolume, got %T", vol)
	}

	define := &Define{as: as}
	if err := configVolume.CreateVolume(define); err != nil {
		t.Fatalf("create vm config-file volume: %v", err)
	}

	if len(as.GetConfigMaps()) != 1 {
		t.Fatalf("expected one configmap attached to app service, got %d", len(as.GetConfigMaps()))
	}
	if as.GetConfigMaps()[0].Data["rainbond.env"] != "DEMO_HOST=demo-service\n" {
		t.Fatalf("unexpected rendered config-file content: %q", as.GetConfigMaps()[0].Data["rainbond.env"])
	}
	if len(define.GetVMVolume()) != 1 {
		t.Fatalf("expected one guest-visible vm volume, got %d", len(define.GetVMVolume()))
	}
	if define.GetVMVolume()[0].ConfigMap == nil {
		t.Fatalf("expected vm config-file to be injected as configmap volume, got %#v", define.GetVMVolume()[0].VolumeSource)
	}
	if define.GetVMVolume()[0].ConfigMap.VolumeLabel == "" || define.GetVMVolume()[0].ConfigMap.VolumeLabel == "RBDCFG" {
		t.Fatalf("expected vm config-file volume label to be stable and specific, got %q", define.GetVMVolume()[0].ConfigMap.VolumeLabel)
	}
	if len(define.GetVMDisk()) != 1 {
		t.Fatalf("expected one guest-visible config disk, got %d", len(define.GetVMDisk()))
	}
	if define.GetVMDisk()[0].DiskDevice.CDRom == nil {
		t.Fatalf("expected guest-visible config disk to be a cdrom, got %#v", define.GetVMDisk()[0].DiskDevice)
	}
	if define.GetVMDisk()[0].Name != define.GetVMVolume()[0].Name {
		t.Fatalf("expected guest-visible config disk to match volume name, got disk=%q volume=%q", define.GetVMDisk()[0].Name, define.GetVMVolume()[0].Name)
	}
	if files := define.GetVMGuestFiles(); len(files) != 1 {
		t.Fatalf("expected one guest sync file mapping, got %#v", files)
	} else {
		if files[0].VolumeLabel != define.GetVMVolume()[0].ConfigMap.VolumeLabel {
			t.Fatalf("expected guest file volume label %q, got %q", define.GetVMVolume()[0].ConfigMap.VolumeLabel, files[0].VolumeLabel)
		}
		if files[0].TargetPath != "/rainbond/env/rainbond.env" {
			t.Fatalf("expected guest file target path /rainbond/env/rainbond.env, got %q", files[0].TargetPath)
		}
		if files[0].SourceFile != "rainbond.env" {
			t.Fatalf("expected guest file source file rainbond.env, got %q", files[0].SourceFile)
		}
	}
	if len(define.GetVolumeMounts()) != 0 {
		t.Fatalf("expected vm config-file not to rely on container volumeMounts, got %#v", define.GetVolumeMounts())
	}
}
