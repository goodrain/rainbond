package conversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	apisixclient "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"
	k8scomponent "github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kv "kubevirt.io/api/core/v1"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type restoreRenderSQL struct {
	db.Manager
	database *gorm.DB
}

func (m restoreRenderSQL) DB() *gorm.DB               { return m.database }
func (m restoreRenderSQL) TenantDao() dbdao.TenantDao { return &mysqldao.TenantDaoImpl{DB: m.database} }
func (m restoreRenderSQL) ApplicationDao() dbdao.ApplicationDao {
	return &mysqldao.ApplicationDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) TenantServiceDao() dbdao.TenantServiceDao {
	return &mysqldao.TenantServicesDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) TenantServiceLabelDao() dbdao.TenantServiceLabelDao {
	return &mysqldao.ServiceLabelDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) VersionInfoDao() dbdao.VersionInfoDao {
	return &mysqldao.VersionInfoDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) AppConfigGroupDao() dbdao.AppConfigGroupDao {
	return &mysqldao.AppConfigGroupDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) TenantServiceRelationDao() dbdao.TenantServiceRelationDao {
	return &mysqldao.TenantServiceRelationDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) TenantServiceEnvVarDao() dbdao.TenantServiceEnvVarDao {
	return &mysqldao.TenantServiceEnvVarDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) TenantServicesPortDao() dbdao.TenantServicesPortDao {
	return &mysqldao.TenantServicesPortDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) TenantServiceVolumeDao() dbdao.TenantServiceVolumeDao {
	return &mysqldao.TenantServiceVolumeDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) TenantServiceMountRelationDao() dbdao.TenantServiceMountRelationDao {
	return &mysqldao.TenantServiceMountRelationDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) ComponentK8sAttributeDao() dbdao.ComponentK8sAttributeDao {
	return &mysqldao.ComponentK8sAttributeDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) ServiceProbeDao() dbdao.ServiceProbeDao {
	return &mysqldao.ServiceProbeDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) HTTPRuleDao() dbdao.HTTPRuleDao {
	return &mysqldao.HTTPRuleDaoImpl{DB: m.database}
}
func (m restoreRenderSQL) TenantServicePluginRelationDao() dbdao.TenantServicePluginRelationDao {
	return &mysqldao.TenantServicePluginRelationDaoImpl{DB: m.database}
}

// capability_id: rainbond.vm-restore.render-success-integration
func TestRestoreRendererCompleteSuccessPath(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "render.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&dbmodel.Application{}, &dbmodel.ApplicationConfigGroup{}, &dbmodel.ConfigGroupService{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, model := range []interface{}{&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.TenantServiceLable{}, &dbmodel.ComponentK8sAttributes{}, &dbmodel.VersionInfo{}, &dbmodel.TenantServiceRelation{}, &dbmodel.TenantServiceEnvVar{}, &dbmodel.TenantServicesPort{}, &dbmodel.TenantServiceVolume{}, &dbmodel.TenantServiceMountRelation{}, &dbmodel.TenantServiceProbe{}, &dbmodel.HTTPRule{}, &dbmodel.TenantServicePluginRelation{}} {
		if err := database.AutoMigrate(model).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Create(&dbmodel.Tenants{UUID: "tenant", Name: "team", Namespace: "team"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.Application{AppID: "app-id", TenantID: "tenant", K8sApp: "windows-app", GovernanceMode: dbmodel.GovernanceModeBuildInServiceMesh}).Error; err != nil {
		t.Fatal(err)
	}
	core := kubefake.NewSimpleClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "rainbond-env", Namespace: "team", UID: "legacy-env"}})
	for _, serviceID := range []string{"service", "another-service"} {
		for _, row := range []interface{}{
			&dbmodel.TenantServices{TenantID: "tenant", AppID: "app-id", ServiceID: serviceID, ServiceAlias: serviceID, K8sComponentName: serviceID, DeployVersion: "render-version", ExtendMethod: string(dbmodel.ServiceTypeVM), ContainerCPU: 2000, ContainerMemory: 2048},
			&dbmodel.VersionInfo{ServiceID: serviceID, BuildVersion: "render-version", ImageName: "example.invalid/base", DeliveredType: "image"},
			&dbmodel.TenantServiceEnvVar{TenantID: "tenant", ServiceID: serviceID, AttrName: "EXAMPLE_SETTING", AttrValue: "same-value", Scope: "inner"},
		} {
			if err := database.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
		app, err := RenderVMForRestore(restoreRenderSQL{database: database}, serviceID)
		if err != nil {
			t.Fatal(err)
		}
		vm := app.GetVirtualMachine()
		if vm == nil {
			t.Fatal("VM not rendered")
		}
		labels := vm.Spec.Template.ObjectMeta.Labels
		if vmName := "windows-app-" + serviceID; vm.Name != vmName || labels["kubevirt.io/domain"] != vmName || labels["rainbond_app"] != "windows-app" {
			t.Fatalf("restore render lost application identity: VM %q, template domain %q, application %q", vm.Name, labels["kubevirt.io/domain"], labels["rainbond_app"])
		}
		declaration, err := RenderVMForNativeRestore(restoreRenderSQL{database: database}, serviceID)
		if err != nil {
			t.Fatal(err)
		}
		nativeVM := declaration.GetVirtualMachine()
		if nativeVM == nil || nativeVM.Name != vm.Name || nativeVM.Spec.Template.ObjectMeta.Labels["rainbond_app"] != "windows-app" || nativeVM.Spec.Template.ObjectMeta.Labels["kubevirt.io/domain"] != vm.Name {
			t.Fatal("native configuration render lost application identity")
		}
		if len(app.VMStartupHints.ConfigMaps) != 1 {
			t.Fatal("renderer did not identify the generated initialization disk")
		}
		published := 0
		for _, cm := range app.GetConfigMaps() {
			if cm.Name != app.VMStartupHints.ConfigMaps[0].Name {
				continue
			}
			if cm.Name == "rainbond-env" {
				t.Fatal("new VM reuses a shared initialization disk and conflicts with existing VMs")
			}
			cm.Namespace = "team"
			if _, err := core.CoreV1().ConfigMaps("team").Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
				t.Fatalf("VM %s cannot publish its own initialization disk: %v", serviceID, err)
			}
			published++
		}
		if published != 1 {
			t.Fatalf("expected one independently owned initialization disk, got %d", published)
		}
	}
}

func (m restoreRenderSQL) TenantServiceConfigFileDao() dbdao.TenantServiceConfigFileDao {
	return &mysqldao.TenantServiceConfigFileDaoImpl{DB: m.database}
}
func renderDigest(t *testing.T, value interface{}) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type renderLifecycleFixture struct {
	database      *gorm.DB
	manager       restoreRenderSQL
	store         *vmrestore.Store
	scope         vmrestore.Scope
	before, after vmrestore.RestoreConfig
	op, manifest  string
	core          *kubefake.Clientset
	dynamic       *dynamicfake.FakeDynamicClient
	routes        *atomic.Int32
	clusterWrites *atomic.Int32
}

func newRenderLifecycleFixture(t *testing.T) renderLifecycleFixture {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "lifecycle.db"))
	if err != nil {
		t.Fatalf("fixture setup stage 4 failed: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	database.LogMode(false)
	if err := database.AutoMigrate(&dbmodel.Application{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range []interface{}{&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.TenantServiceLable{}, &dbmodel.ComponentK8sAttributes{}, &dbmodel.VersionInfo{}, &dbmodel.TenantServiceRelation{}, &dbmodel.TenantServiceEnvVar{}, &dbmodel.TenantServicesPort{}, &dbmodel.TenantServiceVolume{}, &dbmodel.TenantServiceMountRelation{}, &dbmodel.TenantServiceProbe{}, &dbmodel.HTTPRule{}, &dbmodel.TenantServicePluginRelation{}, &dbmodel.ApplicationConfigGroup{}, &dbmodel.ConfigGroupService{}, &dbmodel.TenantServiceConfigFile{}} {
		if err := database.AutoMigrate(m).Error; err != nil {
			t.Fatalf("fixture setup stage 10 failed: %v", err)
		}
	}
	scope := vmrestore.Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	for _, row := range []interface{}{&dbmodel.Tenants{UUID: scope.TenantID, Name: "team", Namespace: scope.Namespace}, &dbmodel.TenantServices{TenantID: scope.TenantID, ServiceID: scope.ServiceID, ServiceAlias: "vm", K8sComponentName: "vm", DeployVersion: "version", ExtendMethod: string(dbmodel.ServiceTypeVM), ContainerCPU: 4000, ContainerMemory: 8192}, &dbmodel.VersionInfo{ServiceID: scope.ServiceID, BuildVersion: "version", ImageName: "example.invalid/unused", DeliveredType: "image"}} {
		if err := database.Create(row).Error; err != nil {
			t.Fatalf("fixture setup stage 16 failed: %v", err)
		}
	}
	before := vmrestore.RestoreConfig{Hardware: vmrestore.SnapshotConfig{CPUMilli: 4000, MemoryMiB: 8192, BootMode: "bios", DiskLayout: []vmrestore.SnapshotLayout{}}, Volumes: []vmrestore.RestoreVolumeConfig{}}
	old := map[string]vmrestore.Binding{}
	for i, id := range []uint{7, 8, 9} {
		key := "data-" + strconv.Itoa(i)
		role := "data"
		path := "/disk-" + strconv.Itoa(i)
		if i == 0 {
			key = "disk"
			role = "root"
			path = "/disk"
		}
		name := "manual" + strconv.Itoa(int(id))
		row := dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: id}, ServiceID: scope.ServiceID, VolumeName: key, VolumePath: path, VolumeType: "cephfs-vm-snapshot", VolumeCapacity: int64(40 - i*10), AccessMode: "ReadWriteMany"}
		if err := database.Create(&row).Error; err != nil {
			t.Fatalf("fixture setup stage 33 failed: %v", err)
		}
		before.Hardware.DiskLayout = append(before.Hardware.DiskLayout, vmrestore.SnapshotLayout{VolumeID: id, DiskKey: key, CanonicalName: name, Role: role, Device: "disk", Bus: "virtio", BootOrder: uint(i + 1)})
		before.Volumes = append(before.Volumes, vmrestore.RestoreVolumeConfig{VolumeID: id, DiskKey: key, CapacityGiB: row.VolumeCapacity, StorageClass: row.VolumeType, AccessMode: row.AccessMode})
		old[strconv.Itoa(int(id))] = vmrestore.Binding{VolumeID: id, DiskKey: key, CanonicalName: name, Role: role, Attached: true, ActivePVC: vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: name, UID: "b-" + name}}
	}
	after := before
	after.Hardware = before.Hardware
	after.Hardware.CPUMilli = 2000
	after.Hardware.MemoryMiB = 4096
	after.Hardware.BootMode = "uefi"
	after.Hardware.DiskLayout = append([]vmrestore.SnapshotLayout(nil), before.Hardware.DiskLayout[:2]...)
	after.Volumes = append([]vmrestore.RestoreVolumeConfig(nil), before.Volumes[:2]...)
	after.Volumes[0].CapacityGiB = 20
	after.Volumes[0].AccessMode = "ReadWriteOnce"
	after.Volumes[1].CapacityGiB = 10
	layout := []map[string]interface{}{}
	for _, d := range before.Hardware.DiskLayout {
		layout = append(layout, map[string]interface{}{"disk_key": d.DiskKey, "disk_name": d.CanonicalName, "disk_role": d.Role, "device_type": d.Device, "source_kind": "volume", "order_index": int(d.BootOrder) - 1, "boot": d.BootOrder > 0, "bus": d.Bus, "boot_order": d.BootOrder})
	}
	raw, _ := json.Marshal(layout)
	for _, row := range []dbmodel.ComponentK8sAttributes{{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_boot_mode", AttributeValue: "bios"}, {TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_disk_layout", AttributeValue: string(raw)}, {TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_os_name", AttributeValue: "linux"}} {
		copy := row
		if err := database.Create(&copy).Error; err != nil {
			t.Fatalf("fixture setup stage 57 failed: %v", err)
		}
	}
	// An advanced configuration disk is not initialization media; it must survive.
	if err := database.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 10}, ServiceID: scope.ServiceID, VolumeName: "advanced-config", VolumePath: "/opt/app/config.ini", VolumeType: dbmodel.ConfigFileVolumeType.String()}).Error; err != nil {
		t.Fatalf("fixture setup stage 62 failed: %v", err)
	}
	if err := database.Create(&dbmodel.TenantServiceConfigFile{ServiceID: scope.ServiceID, VolumeName: "advanced-config", FileContent: "USER_ADVANCED_SENTINEL"}).Error; err != nil {
		t.Fatalf("fixture setup stage 65 failed: %v", err)
	}
	store := vmrestore.NewStore(database)
	op := "render-restore"
	manifest := strings.Repeat("a", 64)
	afterHash, _ := vmrestore.RestoreConfigHash(after)
	intent := vmrestore.RestoreIntent{CurrentVM: vmrestore.ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: "vm", UID: "vm-uid"}, SourceSnapshot: vmrestore.ObjectRef{Kind: "VirtualMachineSnapshot", Namespace: scope.Namespace, Name: "snapshot", UID: "snapshot-uid"}, SourceContent: vmrestore.ObjectRef{Kind: "VirtualMachineSnapshotContent", Namespace: scope.Namespace, Name: "content", UID: "content-uid"}, SourceVMUID: "historical-a", SourceCoreHash: manifest, SourceDocumentHash: manifest, Before: before, ConfigAfterHash: afterHash, WasRunning: true, PowerAfter: "previous"}
	expect := vmrestore.Expectation{OperationID: op, ManifestHash: manifest, Step: "restore-prepare"}
	if _, err := store.PrepareRestoreDB(scope, expect, intent, old); err != nil {
		t.Fatalf("fixture setup stage 74 failed: %v", err)
	}
	aStartup := vmrestore.RestoreStartupPolicy{FirmwareUUID: "11111111-1111-4111-8111-111111111111", BootstrapMode: "restored", DataSourceKind: "CloudInitNoCloud", NoCloudVolumeName: "restored-cloud", NoCloudDiskName: "restored-cloud", ProducerUID: "a-producer", GeneratedMedia: []vmrestore.RestoreGeneratedMedia{}}
	bStartup := vmrestore.RestoreStartupPolicy{FirmwareUUID: "22222222-2222-4222-8222-222222222222", BootstrapMode: "platform", DataSourceKind: "None", ProducerUID: "b-producer", GeneratedMedia: []vmrestore.RestoreGeneratedMedia{}}
	frame := vmrestore.RestoreResourcePlan{Scope: scope, OperationID: op, CoreHash: manifest, WorkerImage: "example.invalid/copy@sha256:" + strings.Repeat("b", 64), RuntimeImageID: "containerd://sha256:" + strings.Repeat("b", 64), DeadlineSeconds: 60, MaxDisks: 8, TargetUID: 107, TargetGID: 107, Disks: []vmrestore.RestoreDiskResource{}}
	for _, id := range []uint{7, 8} {
		frame.Disks = append(frame.Disks, vmrestore.RestoreDiskResource{VolumeID: id, SnapshotContent: vmrestore.ObjectRef{Kind: "VolumeSnapshotContent", Name: "content-" + strconv.Itoa(int(id)), UID: "content-uid-" + strconv.Itoa(int(id))}, Snapshot: vmrestore.ObjectRef{Kind: "VolumeSnapshot", Namespace: scope.Namespace, Name: "snapshot-" + strconv.Itoa(int(id)), UID: "snapshot-uid-" + strconv.Itoa(int(id))}, PreviousPVC: old[strconv.Itoa(int(id))].ActivePVC, PreviousMaxBytes: 40 << 30, ROXClass: "rox", ROXClassUID: "rox-uid", TargetClass: "cephfs-vm-snapshot", TargetClassUID: "target-uid", TargetAccessMode: "ReadWriteMany", ROXRequestBytes: 20 << 30, TargetRequestBytes: 20 << 30})
	}
	frame.FrameHash, _ = vmrestore.RestoreResourceFrameHash(frame)
	network, _ := store.NetworkIdentityHash(scope)
	if _, err := store.InitializeRestoreExecution(scope, op, vmrestore.RestoreExecution{ConfigAfter: &after, Frame: frame, Intents: map[string]vmrestore.RestoreObjectIntent{}, Evidence: map[string]restoreplan.CopyEvidence{}, NetworkHash: network, IdentityHash: renderDigest(t, aStartup), BeforeStartup: bStartup, SourceStartup: aStartup}); err != nil {
		t.Fatalf("fixture setup stage 85 failed: %v", err)
	}
	targets := map[uint]vmrestore.ObjectRef{7: {Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "a-root", UID: "a-root-uid"}, 8: {Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "a-data", UID: "a-data-uid"}}
	for _, step := range []string{"restore-writers-clear", "restore-copy-verified"} {
		current, _ := store.Read(scope, op)
		ex := expect
		ex.ControlRevision = current.State.ControlRevision
		ex.BindingGeneration = current.State.BindingGeneration
		ex.Step = step
		ex.RequestHash = manifest
		if _, err := store.Mutate(scope, ex, func(c vmrestore.Current) (vmrestore.Mutation, error) {
			c.State.Fence.Phase = "copy-verified"
			if step == "restore-copy-verified" {
				for _, ref := range targets {
					c.Receipt.Resources = append(c.Receipt.Resources, vmrestore.ResourceRef{ObjectRef: ref, Purpose: "restore-target"})
				}
			}
			return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
		}); err != nil {
			t.Fatalf("fixture setup stage 104 failed: %v", err)
		}
	}
	current, _ := store.Read(scope, op)
	expect.ControlRevision = current.State.ControlRevision
	expect.BindingGeneration = current.State.BindingGeneration
	expect.Step = vmrestore.RestoreCommitStep
	bindings, err := vmrestore.BuildRestoreBindings(scope, op, intent.SourceSnapshot.UID, old, after, targets)
	if err != nil {
		t.Fatalf("fixture setup stage 113 failed: %v", err)
	}
	if _, err := store.CommitRestoreDB(scope, expect, vmrestore.RestoreCommitRequest{Config: after, Bindings: bindings}); err != nil {
		t.Fatalf("fixture setup stage 116 failed: %v", err)
	}
	core := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.Namespace, UID: "namespace-uid"}})
	for _, ref := range []vmrestore.ObjectRef{old["7"].ActivePVC, old["8"].ActivePVC, old["9"].ActivePVC, targets[7], targets[8]} {
		class := "cephfs-vm-snapshot"
		if _, err := core.CoreV1().PersistentVolumeClaims(scope.Namespace).Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: scope.Namespace, UID: types.UID(ref.UID), Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &class, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("40Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{}); err != nil {
			t.Fatalf("fixture setup stage 122 failed: %v", err)
		}
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}: "VirtualMachineList", {Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}: "VirtualMachineInstanceList"})
	clients := k8scomponent.Default()
	if clients == nil {
		clients = k8scomponent.New()
	}
	saved := *clients
	t.Cleanup(func() { *clients = saved })
	clusterWrites := &atomic.Int32{}
	kubeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			clusterWrites.Add(1)
			http.Error(w, "render attempted cluster mutation", 500)
			return
		}
		var value interface{}
		prefix := "/api/v1/namespaces/" + scope.Namespace + "/persistentvolumeclaims/"
		if strings.HasPrefix(r.URL.Path, prefix) {
			pvc, err := core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(r.Context(), strings.TrimPrefix(r.URL.Path, prefix), metav1.GetOptions{})
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			pvc.APIVersion = "v1"
			pvc.Kind = "PersistentVolumeClaim"
			value = pvc
		} else if r.URL.Path == "/api/v1/namespaces/"+scope.Namespace {
			ns, _ := core.CoreV1().Namespaces().Get(r.Context(), scope.Namespace, metav1.GetOptions{})
			ns.APIVersion = "v1"
			ns.Kind = "Namespace"
			value = ns
		} else {
			http.Error(w, "unexpected read "+r.URL.Path, 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
	}))
	t.Cleanup(kubeServer.Close)
	realCore, err := kubernetes.NewForConfig(&rest.Config{Host: kubeServer.URL})
	if err != nil {
		t.Fatalf("fixture setup stage 163 failed: %v", err)
	}
	realDynamic, err := dynamic.NewForConfig(&rest.Config{Host: kubeServer.URL})
	if err != nil {
		t.Fatalf("fixture setup stage 167 failed: %v", err)
	}
	clients.Clientset = realCore
	clients.DynamicClient = realDynamic
	routes := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routes.Add(1)
		http.Error(w, "unexpected route operation", 500)
	}))
	t.Cleanup(server.Close)
	api, err := apisixclient.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("fixture setup stage 179 failed: %v", err)
	}
	clients.ApiSixClient = api
	return renderLifecycleFixture{database, restoreRenderSQL{database: database}, store, scope, before, after, op, manifest, core, dyn, routes, clusterWrites}
}

func (f renderLifecycleFixture) databaseImage(t *testing.T) string {
	t.Helper()
	image := []interface{}{}
	for _, model := range []interface{}{&dbmodel.TenantServices{}, &dbmodel.TenantServiceVolume{}, &dbmodel.ComponentK8sAttributes{}, &dbmodel.TenantServiceMountRelation{}, &dbmodel.TenantServicesPort{}, &dbmodel.ApplicationConfigGroup{}, &dbmodel.ConfigGroupService{}, &dbmodel.TenantServiceConfigFile{}} {
		table := f.database.NewScope(model).TableName()
		rows, err := f.database.DB().Query("SELECT * FROM " + table + " ORDER BY 1")
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			t.Fatal(err)
		}
		data := [][]interface{}{}
		for rows.Next() {
			values := make([]interface{}, len(columns))
			pointers := make([]interface{}, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			for i, v := range values {
				if b, ok := v.([]byte); ok {
					values[i] = string(b)
				}
			}
			data = append(data, values)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		image = append(image, []interface{}{table, columns, data})
	}
	return renderDigest(t, image)
}

func assertRenderedConfig(t *testing.T, vm *kv.VirtualMachine, config vmrestore.RestoreConfig, bindings map[string]vmrestore.Binding) {
	t.Helper()
	disks := []vmrestore.SnapshotDisk{}
	for _, layout := range config.Hardware.DiskLayout {
		b := bindings[strconv.Itoa(int(layout.VolumeID))]
		disks = append(disks, vmrestore.SnapshotDisk{VolumeID: b.VolumeID, DiskKey: b.DiskKey, CanonicalName: b.CanonicalName, Role: b.Role, SourcePVC: b.ActivePVC, SourceDV: b.ActiveDV, ReadOnly: layout.ReadOnly})
	}
	hardware, err := vmrestore.SnapshotSafeConfig(vm, disks)
	if err != nil {
		t.Fatal(err)
	}
	observed := config
	observed.Hardware = hardware
	want, _ := vmrestore.RestoreConfigHash(config)
	actual, _ := vmrestore.RestoreConfigHash(observed)
	if want != actual {
		t.Fatal("real rendered hardware/layout differs from the complete sealed config")
	}
}

// capability_id: rainbond.vm-restore.render-success-integration
func TestRestoreRendererSealedACompleteSuccess(t *testing.T) {
	f := newRenderLifecycleFixture(t)
	before, _ := f.store.Read(f.scope, f.op)
	dbBefore := f.databaseImage(t)
	f.core.ClearActions()
	f.dynamic.ClearActions()
	writes := 0
	f.database.Callback().Create().Before("gorm:create").Register("render-no-create", func(s *gorm.Scope) { writes++; s.Err(fmt.Errorf("unexpected domain create")) })
	f.database.Callback().Update().Before("gorm:update").Register("render-no-update", func(s *gorm.Scope) { writes++; s.Err(fmt.Errorf("unexpected domain update")) })
	f.database.Callback().Delete().Before("gorm:delete").Register("render-no-delete", func(s *gorm.Scope) { writes++; s.Err(fmt.Errorf("unexpected domain delete")) })
	app, err := RenderVMForRestore(f.manager, f.scope.ServiceID)
	if err != nil {
		t.Fatal(err)
	}
	vm := app.GetVirtualMachine()
	if vm == nil || vm.Spec.Template == nil {
		t.Fatal("no VM output")
	}
	assertRenderedConfig(t, vm, f.after, before.State.Bindings)
	if vm.Spec.Template.Spec.Domain.CPU.Cores != 2 || vm.Spec.Template.Spec.Domain.Memory.Guest.Value() != 4<<30 || vm.Spec.Template.Spec.Domain.Firmware.UUID != "11111111-1111-4111-8111-111111111111" || vm.Spec.Template.Spec.Domain.Firmware.Bootloader.EFI == nil {
		t.Fatal("sealed A hardware or startup identity not rendered")
	}
	found := map[string]string{}
	advanced := false
	cloud := false
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if v.PersistentVolumeClaim != nil {
			found[v.Name] = v.PersistentVolumeClaim.ClaimName
		}
		if v.ConfigMap != nil {
			for _, cm := range app.GetConfigMaps() {
				if cm.Name == v.ConfigMap.Name && cm.Data["config.ini"] == "USER_ADVANCED_SENTINEL" {
					advanced = true
				}
			}
		}
		if v.CloudInitNoCloud != nil {
			cloud = v.Name == "restored-cloud" && v.CloudInitNoCloud.UserData == "#cloud-config\n{}\n"
		}
	}
	if found["manual7"] != "a-root" || found["manual8"] != "a-data" || found["manual9"] != "" || !advanced || !cloud || len(vm.Spec.DataVolumeTemplates) != 0 {
		t.Fatal("A disk set/media was incomplete or reimported", found, advanced, cloud)
	}
	after, _ := f.store.Read(f.scope, f.op)
	if !reflect.DeepEqual(before, after) || writes != 0 || f.routes.Load() != 0 || f.clusterWrites.Load() != 0 || dbBefore != f.databaseImage(t) {
		t.Fatal("render mutated DB, route or sealed power intent", writes, f.routes.Load())
	}
	for _, a := range f.core.Actions() {
		if a.GetVerb() != "get" && a.GetVerb() != "list" {
			t.Fatal("unexpected core mutation", a.GetVerb())
		}
	}
	if !reflect.DeepEqual(app.VMRestoredRuntime.Config, f.after) || app.VMRestoredRuntime.Config.Volumes[0].AccessMode != "ReadWriteOnce" || !after.Receipt.Restore.WasRunning || after.Receipt.Restore.PowerAfter != "previous" {
		t.Fatal("A access mode or sealed original power changed")
	}
	for _, cm := range app.GetConfigMaps() {
		if cm.Data["config.ini"] != "USER_ADVANCED_SENTINEL" {
			t.Fatal("B initialization environment was reintroduced into A")
		}
	}
	if len(app.GetClaimsManually()) != 0 || len(f.dynamic.Actions()) != 0 {
		t.Fatal("A render scheduled a volume import or executed a VM operation")
	}
	t.Run("native configuration check survives replacement of previous PVCs", func(t *testing.T) {
		for _, name := range []string{"manual7", "manual8", "manual9", "a-root", "a-data"} {
			if err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := RenderVMForRestore(f.manager, f.scope.ServiceID); err == nil {
			t.Fatal("runtime render accepted missing ledger-bound disks")
		}
		declaration, err := RenderVMForNativeRestore(f.manager, f.scope.ServiceID)
		if err != nil {
			t.Fatalf("native configuration check requires the previous physical disks: %v", err)
		}
		vm := declaration.GetVirtualMachine()
		if vm == nil || declaration.ContainerCPU != 2000 || declaration.ContainerMemory != 4096 || declaration.ExtensionSet["vm_boot_mode"] != "uefi" {
			t.Fatal("native check lost the configured hardware")
		}
		if declaration.VMStorageView != nil || declaration.VMRestoredRuntime != nil || dbBefore != f.databaseImage(t) || writes != 0 || f.routes.Load() != 0 || f.clusterWrites.Load() != 0 {
			t.Fatal("native configuration render resolved physical storage or mutated platform state")
		}
	})
}

func (f renderLifecycleFixture) step(t *testing.T, name string, fn func(vmrestore.Current) (vmrestore.Mutation, error)) {
	t.Helper()
	c, err := f.store.Read(f.scope, f.op)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: f.op, ManifestHash: f.manifest, RequestHash: f.manifest, Step: name}, fn)
	if err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.render-success-integration
func TestRestoreRendererFullBRollbackSuccess(t *testing.T) {
	f := newRenderLifecycleFixture(t)
	f.step(t, "restore-rollback-writers-clear", func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Fence.Phase = "rollback-stopped"
		return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	c, _ := f.store.Read(f.scope, f.op)
	_, err := f.store.RollbackRestoreDB(f.scope, vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: f.op, ManifestHash: f.manifest, Step: "restore-rollback"})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := f.store.Read(f.scope, f.op)
	dbBefore := f.databaseImage(t)
	f.core.ClearActions()
	f.dynamic.ClearActions()
	app, err := RenderVMForRestore(f.manager, f.scope.ServiceID)
	if err != nil {
		t.Fatal(err)
	}
	if app.VMRestoredRuntime != nil {
		t.Fatal("B canonical origin was treated as selected A")
	}
	// This is the same renderer + sealed-selection projection used by the
	// production KubeRestoreRuntime rollback builder. No power API is called.
	startup := before.Receipt.Execution.BeforeStartup
	vm, err := vmrestore.ProjectRestoredRuntime(context.Background(), f.core, app.GetVirtualMachine(), vmrestore.RuntimeSelection{Scope: f.scope, BindingGeneration: before.State.BindingGeneration, Bindings: before.State.Bindings, Config: before.Receipt.Restore.Before, Startup: startup, BeforeStartup: startup})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderedConfig(t, vm, before.Receipt.Restore.Before, before.State.Bindings)
	if vm.Spec.Template.Spec.Domain.CPU.Cores != 4 || vm.Spec.Template.Spec.Domain.Memory.Guest.Value() != 8<<30 || vm.Spec.Template.Spec.Domain.Firmware.UUID != types.UID(startup.FirmwareUUID) || vm.Spec.Template.Spec.Domain.Firmware.Bootloader.BIOS == nil {
		t.Fatal("complete sealed B hardware/startup not restored")
	}
	claims := map[string]string{}
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if v.PersistentVolumeClaim != nil {
			claims[v.Name] = v.PersistentVolumeClaim.ClaimName
		}
	}
	for _, name := range []string{"manual7", "manual8", "manual9"} {
		if claims[name] != name {
			t.Fatal("B root/data disk omitted or mapped to A", claims)
		}
	}
	if len(vm.Spec.DataVolumeTemplates) != 0 {
		t.Fatal("plain B PVCs scheduled reimport")
	}
	advanced := false
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if v.ConfigMap != nil {
			for _, cm := range app.GetConfigMaps() {
				if cm.Name == v.ConfigMap.Name && cm.Data["config.ini"] == "USER_ADVANCED_SENTINEL" {
					advanced = true
				}
			}
		}
	}
	if !advanced {
		t.Fatal("B user configuration medium was lost")
	}
	after, _ := f.store.Read(f.scope, f.op)
	if !reflect.DeepEqual(before, after) || f.routes.Load() != 0 || f.clusterWrites.Load() != 0 || dbBefore != f.databaseImage(t) {
		t.Fatal("B rendering changed state, power seal or routes")
	}
	for _, action := range f.core.Actions() {
		if action.GetVerb() != "get" && action.GetVerb() != "list" {
			t.Fatal("B rendering wrote to cluster", action.GetVerb())
		}
	}
	if len(f.dynamic.Actions()) != 0 {
		t.Fatal("B render executed a VM power/resource operation")
	}
}
