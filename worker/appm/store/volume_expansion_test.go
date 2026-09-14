package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/fake"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	k8stesting "k8s.io/client-go/testing"
)

type volumeExpansionTestManager struct {
	db.Manager
	serviceDao *volumeExpansionServiceDAO
	volumeDao  *volumeExpansionVolumeDAO
	tenantDao  *volumeExpansionTenantDAO
}

func (m volumeExpansionTestManager) TenantServiceDao() dao.TenantServiceDao { return m.serviceDao }
func (m volumeExpansionTestManager) TenantServiceVolumeDao() dao.TenantServiceVolumeDao {
	return m.volumeDao
}
func (m volumeExpansionTestManager) TenantDao() dao.TenantDao { return m.tenantDao }

type volumeExpansionServiceDAO struct {
	dao.TenantServiceDao
	service *dbmodel.TenantServices
}

func (d *volumeExpansionServiceDAO) GetServiceByID(string) (*dbmodel.TenantServices, error) {
	return d.service, nil
}

type volumeExpansionVolumeDAO struct {
	dao.TenantServiceVolumeDao
	volume *dbmodel.TenantServiceVolume
	err    error
}

func (d *volumeExpansionVolumeDAO) GetVolumeByServiceIDAndName(string, string) (*dbmodel.TenantServiceVolume, error) {
	return d.volume, d.err
}

type volumeExpansionTenantDAO struct {
	dao.TenantDao
	tenant *dbmodel.Tenants
}

func (d *volumeExpansionTenantDAO) GetTenantByUUID(string) (*dbmodel.Tenants, error) {
	return d.tenant, nil
}

type volumeExpansionFixture struct {
	store        *appRuntimeStore
	client       *fake.Clientset
	manager      volumeExpansionTestManager
	claim        *corev1.PersistentVolumeClaim
	statefulSet  *appsv1.StatefulSet
	storageClass *storagev1.StorageClass
}

type contextObservingVolumeClient struct {
	*fake.Clientset
	observe func(context.Context)
}

func (c contextObservingVolumeClient) CoreV1() coreclient.CoreV1Interface {
	return contextObservingCoreClient{CoreV1Interface: c.Clientset.CoreV1(), observe: c.observe}
}

type contextObservingCoreClient struct {
	coreclient.CoreV1Interface
	observe func(context.Context)
}

func (c contextObservingCoreClient) PersistentVolumeClaims(namespace string) coreclient.PersistentVolumeClaimInterface {
	return contextObservingClaimClient{PersistentVolumeClaimInterface: c.CoreV1Interface.PersistentVolumeClaims(namespace), observe: c.observe}
}

type contextObservingClaimClient struct {
	coreclient.PersistentVolumeClaimInterface
	observe func(context.Context)
}

func (c contextObservingClaimClient) Get(ctx context.Context, name string, options metav1.GetOptions) (*corev1.PersistentVolumeClaim, error) {
	c.observe(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.PersistentVolumeClaimInterface.Get(ctx, name, options)
}

func newVolumeExpansionFixture(t *testing.T) *volumeExpansionFixture {
	t.Helper()
	t.Setenv("ENABLE_SUBPATH", "false")
	className := "expandable"
	allowed := true
	labels := map[string]string{
		"creator": "Rainbond", "creater_id": "deployment-creator", "service_id": "service-1",
		"tenant_id": "tenant-1", "volume_name": "data",
	}
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "manual7-app-database-1", Namespace: "tenant-ns", UID: "claim-uid", ResourceVersion: "1",
			Labels: labels, Annotations: map[string]string{"volume_name": "data", "preserve": "value"},
			Finalizers: []string{"kubernetes.io/pvc-protection"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &className, VolumeName: "pv-database-1", AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")}},
		},
		Status: corev1.PersistentVolumeClaimStatus{Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")}},
	}
	template := claim.DeepCopy()
	template.Name = "manual7"
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "app-database", Namespace: "tenant-ns", Labels: claim.DeepCopy().Labels},
		Spec:       appsv1.StatefulSetSpec{VolumeClaimTemplates: []corev1.PersistentVolumeClaim{*template}},
	}
	storageClass := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: className}, AllowVolumeExpansion: &allowed}
	manager := volumeExpansionTestManager{
		serviceDao: &volumeExpansionServiceDAO{service: &dbmodel.TenantServices{
			ServiceID: "service-1", TenantID: "tenant-1", Namespace: "not-the-tenant-namespace",
			ExtendMethod: dbmodel.ServiceTypeStateMultiple.String(),
		}},
		tenantDao: &volumeExpansionTenantDAO{tenant: &dbmodel.Tenants{UUID: "tenant-1", Namespace: "tenant-ns"}},
		volumeDao: &volumeExpansionVolumeDAO{volume: &dbmodel.TenantServiceVolume{
			Model: dbmodel.Model{ID: 7}, ServiceID: "service-1", VolumeName: "data", VolumeType: className, VolumeCapacity: 20,
		}},
	}
	client := fake.NewSimpleClientset(claim.DeepCopy(), statefulSet.DeepCopy(), storageClass.DeepCopy())
	store := &appRuntimeStore{ctx: context.Background(), dbmanager: manager, volumeExpansionClient: client}
	app := &v1.AppService{AppServiceBase: v1.AppServiceBase{ServiceID: "service-1", CreaterID: "deployment-creator"}}
	app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant-ns"}})
	store.appServices.Store(v1.GetCacheKeyOnlyServiceID("service-1"), app)
	return &volumeExpansionFixture{store: store, client: client, manager: manager, claim: claim, statefulSet: statefulSet, storageClass: storageClass}
}

func (f *volumeExpansionFixture) assertCapacity(t *testing.T, capacity string, updateCount int) {
	t.Helper()
	current, err := f.client.CoreV1().PersistentVolumeClaims(f.claim.Namespace).Get(context.Background(), f.claim.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := current.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse(capacity)) != 0 {
		t.Fatalf("PVC request = %s, want %s", got.String(), capacity)
	}
	updates := 0
	for _, action := range f.client.Actions() {
		if action.GetVerb() == "update" && action.GetResource().Resource == "persistentvolumeclaims" {
			updates++
			continue
		}
		if action.GetVerb() != "get" && action.GetVerb() != "list" {
			t.Fatalf("unexpected mutation %s %s", action.GetVerb(), action.GetResource().Resource)
		}
	}
	if updates != updateCount {
		t.Fatalf("PVC update attempts = %d, want %d", updates, updateCount)
	}
}

// capability_id: rainbond.worker.appm.store.stateful-volume-capacity-reconciliation
func TestStatefulVolumeExpansionOnClaimEvents(t *testing.T) {
	for _, event := range []string{"add", "update", "same resource version resync", "add after workload upgrade"} {
		t.Run(event, func(t *testing.T) {
			f := newVolumeExpansionFixture(t)
			switch event {
			case "add":
				f.store.OnAdd(f.claim.DeepCopy(), false)
			case "update":
				current := f.claim.DeepCopy()
				current.ResourceVersion = "2"
				f.store.OnUpdate(f.claim.DeepCopy(), current)
			case "add after workload upgrade":
				f.applyStatefulSetUpgrade(t)
				f.store.OnAdd(f.claim.DeepCopy(), false)
			default:
				f.store.OnUpdate(f.claim.DeepCopy(), f.claim.DeepCopy())
			}
			f.assertCapacity(t, "20Gi", 1)
			current, _ := f.client.CoreV1().PersistentVolumeClaims(f.claim.Namespace).Get(context.Background(), f.claim.Name, metav1.GetOptions{})
			want := f.claim.DeepCopy()
			want.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("20Gi")
			if !reflect.DeepEqual(current, want) {
				t.Fatal("capacity update changed unrelated PVC fields")
			}
			sts, _ := f.client.AppsV1().StatefulSets(f.claim.Namespace).Get(context.Background(), f.statefulSet.Name, metav1.GetOptions{})
			if !reflect.DeepEqual(sts, f.statefulSet) {
				t.Fatal("StatefulSet template must not be mutated or recreated")
			}
			if got := f.claim.Spec.Resources.Requests[corev1.ResourceStorage]; got.String() != "10Gi" {
				t.Fatal("informer object was mutated")
			}
		})
	}
}

func (f *volumeExpansionFixture) applyStatefulSetUpgrade(t *testing.T) {
	t.Helper()
	replicas := int32(2)
	f.statefulSet.Spec.Replicas = &replicas
	f.statefulSet.Spec.Template.Labels = f.claim.DeepCopy().Labels
	desired := f.statefulSet.DeepCopy()
	desired.Labels["creater_id"] = "upgrade-creator"
	desired.Spec.Template.Labels["creater_id"] = "upgrade-creator"
	desired.Spec.VolumeClaimTemplates[0].Labels["creater_id"] = "upgrade-creator"
	desired.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("20Gi")
	oldApp := &v1.AppService{}
	oldApp.SetStatefulSet(f.statefulSet.DeepCopy())
	newApp := &v1.AppService{UpgradePatch: map[string][]byte{}}
	newApp.SetStatefulSet(desired)
	if err := oldApp.SetUpgradePatch(newApp); err != nil {
		t.Fatalf("build StatefulSet upgrade patch: %v", err)
	}
	original, err := json.Marshal(f.statefulSet)
	if err != nil {
		t.Fatal(err)
	}
	patched, err := strategicpatch.StrategicMergePatch(original, newApp.UpgradePatch["statefulset"], appsv1.StatefulSet{})
	if err != nil {
		t.Fatalf("apply StatefulSet upgrade patch: %v", err)
	}
	upgraded := &appsv1.StatefulSet{}
	if err := json.Unmarshal(patched, upgraded); err != nil {
		t.Fatal(err)
	}
	if upgraded.Labels["creater_id"] != "upgrade-creator" {
		t.Fatal("upgrade must change the StatefulSet metadata generation")
	}
	if !reflect.DeepEqual(upgraded.Spec.VolumeClaimTemplates, f.statefulSet.Spec.VolumeClaimTemplates) {
		t.Fatal("upgrade must preserve the immutable claim template generation and capacity")
	}
	f.statefulSet = upgraded
	if err := f.client.Tracker().Update(appsv1.SchemeGroupVersion.WithResource("statefulsets"), upgraded, upgraded.Namespace); err != nil {
		t.Fatal(err)
	}
}

func TestStatefulVolumeExpansionRetriesAfterDatabaseCommit(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	f.manager.volumeDao.volume.VolumeCapacity = 10
	f.store.OnAdd(f.claim.DeepCopy(), false)
	f.assertCapacity(t, "10Gi", 0)
	f.manager.volumeDao.volume.VolumeCapacity = 20
	f.store.OnUpdate(f.claim.DeepCopy(), f.claim.DeepCopy())
	f.assertCapacity(t, "20Gi", 1)
}

func TestStatefulVolumeExpansionRetriesFailuresAndConflicts(t *testing.T) {
	for _, reason := range []string{"database", "temporary API failure", "conflict"} {
		t.Run(reason, func(t *testing.T) {
			f := newVolumeExpansionFixture(t)
			attempts := 0
			if reason == "database" {
				f.manager.volumeDao.err = fmt.Errorf("database unavailable")
			} else {
				f.client.PrependReactor("update", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
					attempts++
					if attempts == 1 {
						if reason == "conflict" {
							current := f.claim.DeepCopy()
							current.Annotations["concurrent-change"] = "preserved"
							if err := f.client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), current, current.Namespace); err != nil {
								t.Fatal(err)
							}
							return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "persistentvolumeclaims"}, f.claim.Name, fmt.Errorf("changed"))
						}
						return true, nil, apierrors.NewServiceUnavailable("temporary failure")
					}
					return false, nil, nil
				})
			}
			f.store.OnAdd(f.claim.DeepCopy(), false)
			if reason != "conflict" {
				count := 1
				if reason == "database" {
					count = 0
				}
				f.assertCapacity(t, "10Gi", count)
				f.manager.volumeDao.err = nil
				f.store.OnUpdate(f.claim.DeepCopy(), f.claim.DeepCopy())
			}
			count := 2
			if reason == "database" {
				count = 1
			}
			f.assertCapacity(t, "20Gi", count)
			if reason == "conflict" {
				current, _ := f.client.CoreV1().PersistentVolumeClaims(f.claim.Namespace).Get(context.Background(), f.claim.Name, metav1.GetOptions{})
				if current.Annotations["concurrent-change"] != "preserved" {
					t.Fatal("conflict retry overwrote concurrent PVC metadata")
				}
			}
		})
	}
}

func TestStatefulVolumeExpansionSkipsUnsupportedOrForeignClaims(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*volumeExpansionFixture)
	}{
		{"already at target", func(f *volumeExpansionFixture) {
			f.claim.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("20Gi")
		}},
		{"larger request", func(f *volumeExpansionFixture) {
			f.claim.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("30Gi")
		}},
		{"larger actual capacity", func(f *volumeExpansionFixture) {
			f.claim.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("30Gi")
		}},
		{"storage class expansion disabled", func(f *volumeExpansionFixture) { allowed := false; f.storageClass.AllowVolumeExpansion = &allowed }},
		{"storage class expansion unset", func(f *volumeExpansionFixture) { f.storageClass.AllowVolumeExpansion = nil }},
		{"storage class absent", func(f *volumeExpansionFixture) { f.claim.Spec.StorageClassName = nil }},
		{"NFS subdir provisioner", func(f *volumeExpansionFixture) {
			f.storageClass.Provisioner = "example/nfs-subdir-external-provisioner"
		}},
		{"NFS client provisioner", func(f *volumeExpansionFixture) { f.storageClass.Provisioner = "example/nfs-client-provisioner" }},
		{"stateless service", func(f *volumeExpansionFixture) {
			f.manager.serviceDao.service.ExtendMethod = dbmodel.ServiceTypeStatelessMultiple.String()
		}},
		{"VM", func(f *volumeExpansionFixture) { f.manager.serviceDao.service.ExtendMethod = "vm" }},
		{"memory filesystem", func(f *volumeExpansionFixture) {
			f.manager.volumeDao.volume.VolumeType = dbmodel.MemoryFSVolumeType.String()
		}},
		{"configuration file", func(f *volumeExpansionFixture) {
			f.manager.volumeDao.volume.VolumeType = dbmodel.ConfigFileVolumeType.String()
		}},
		{"shared subpath", func(f *volumeExpansionFixture) {
			t.Setenv("ENABLE_SUBPATH", "true")
			f.manager.volumeDao.volume.VolumeType = dbmodel.ShareFileVolumeType.String()
		}},
		{"no desired capacity", func(f *volumeExpansionFixture) { f.manager.volumeDao.volume.VolumeCapacity = 0 }},
		{"foreign creator", func(f *volumeExpansionFixture) { f.claim.Labels["creator"] = "other-platform" }},
		{"foreign tenant label", func(f *volumeExpansionFixture) { f.claim.Labels["tenant_id"] = "other-tenant" }},
		{"foreign namespace", func(f *volumeExpansionFixture) { f.claim.Namespace = "other-namespace" }},
		{"foreign volume label", func(f *volumeExpansionFixture) { f.claim.Labels["volume_name"] = "other-data" }},
		{"missing volume annotation", func(f *volumeExpansionFixture) { delete(f.claim.Annotations, "volume_name") }},
		{"wrong volume ID", func(f *volumeExpansionFixture) { f.manager.volumeDao.volume.ID = 8 }},
		{"wrong volume service", func(f *volumeExpansionFixture) { f.manager.volumeDao.volume.ServiceID = "other-service" }},
		{"wrong volume name", func(f *volumeExpansionFixture) { f.manager.volumeDao.volume.VolumeName = "other-data" }},
		{"wrong StatefulSet service", func(f *volumeExpansionFixture) { f.statefulSet.Labels["service_id"] = "other-service" }},
		{"wrong template creator", func(f *volumeExpansionFixture) {
			f.statefulSet.Spec.VolumeClaimTemplates[0].Labels["creater_id"] = "other-generation"
		}},
		{"unrelated template", func(f *volumeExpansionFixture) { f.statefulSet.Spec.VolumeClaimTemplates[0].Name = "manual8" }},
		{"non-ordinal claim", func(f *volumeExpansionFixture) { f.claim.Name = "manual7-app-database-backup" }},
		{"manual standalone claim", func(f *volumeExpansionFixture) { f.claim.Name = "manual7" }},
		{"deleting claim", func(f *volumeExpansionFixture) { now := metav1.Now(); f.claim.DeletionTimestamp = &now }},
		{"deleting StatefulSet", func(f *volumeExpansionFixture) { now := metav1.Now(); f.statefulSet.DeletionTimestamp = &now }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newVolumeExpansionFixture(t)
			test.change(f)
			f.client = fake.NewSimpleClientset(f.claim.DeepCopy(), f.statefulSet.DeepCopy(), f.storageClass.DeepCopy())
			f.store.volumeExpansionClient = f.client
			f.store.OnAdd(f.claim.DeepCopy(), false)
			capacity := f.claim.Spec.Resources.Requests[corev1.ResourceStorage]
			f.assertCapacity(t, capacity.String(), 0)
		})
	}
}

func TestStatefulVolumeExpansionAcceptsManualVolumeLabel(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	f.claim.Labels["volume_name"] = "manual7"
	f.statefulSet.Spec.VolumeClaimTemplates[0].Labels["volume_name"] = "manual7"
	f.client = fake.NewSimpleClientset(f.claim.DeepCopy(), f.statefulSet.DeepCopy(), f.storageClass.DeepCopy())
	f.store.volumeExpansionClient = f.client
	f.store.OnAdd(f.claim.DeepCopy(), false)
	f.assertCapacity(t, "20Gi", 1)
}

func TestStatefulVolumeExpansionBoundsKubernetesCalls(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("store cancelled %t", cancelled), func(t *testing.T) {
			f := newVolumeExpansionFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				cancel()
			}
			f.store.ctx = ctx
			observed := false
			f.store.volumeExpansionClient = contextObservingVolumeClient{Clientset: f.client, observe: func(ctx context.Context) {
				observed = true
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 30*time.Second {
					t.Error("PVC reconciliation must use a bounded context")
				}
				if cancelled && ctx.Err() != context.Canceled {
					t.Error("PVC reconciliation must honor store cancellation")
				}
			}}
			f.store.OnAdd(f.claim.DeepCopy(), false)
			if !observed {
				t.Fatal("PVC read was not observed")
			}
			if cancelled {
				f.assertCapacity(t, "10Gi", 0)
			} else {
				f.assertCapacity(t, "20Gi", 1)
			}
		})
	}
}
