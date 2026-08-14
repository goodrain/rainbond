package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	appmtypes "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func configMapForTest(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Data:       map[string]string{"k": "v"},
	}
}

func serviceForTest(name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 80}},
		},
	}
}

func newAppWithConfigMaps(namespace *corev1.Namespace, cms ...*corev1.ConfigMap) *appmtypes.AppService {
	app := &appmtypes.AppService{AppServiceBase: appmtypes.AppServiceBase{ServiceID: "service-1", ServiceAlias: "demo"}}
	app.SetTenant(namespace)
	for _, cm := range cms {
		app.SetConfigMap(cm)
	}
	return app
}

func newAppWithServices(namespace *corev1.Namespace, svcs ...*corev1.Service) *appmtypes.AppService {
	app := &appmtypes.AppService{AppServiceBase: appmtypes.AppServiceBase{ServiceID: "service-1", ServiceAlias: "demo"}}
	app.SetTenant(namespace)
	for _, svc := range svcs {
		app.SetService(svc)
	}
	return app
}

// capability_id: rainbond.upgrade-configmap-aggregates-create-update-errors
func TestUpgradeConfigMapErrorAggregation(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	tests := []struct {
		name        string
		setup       func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService)
		wantErr     bool
		wantContain []string
	}{
		{
			name: "create failure returns error",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				newApp.SetConfigMap(configMapForTest("cm-new"))
				client.PrependReactor("create", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("create boom")
				})
			},
			wantErr:     true,
			wantContain: []string{"create boom"},
		},
		{
			name: "update failure returns error",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				existing := configMapForTest("cm-shared")
				nowApp.SetConfigMap(existing)
				newApp.SetConfigMap(configMapForTest("cm-shared"))
				// pre-create the configmap in the fake tracker so the failed
				// update leaves a real object behind; the best-effort delete
				// path then finds it and stays quiet (no not-found log noise).
				if _, err := client.CoreV1().ConfigMaps("default").Create(context.Background(), configMapForTest("cm-shared"), metav1.CreateOptions{}); err != nil {
					panic(err)
				}
				client.PrependReactor("update", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("update boom")
				})
			},
			wantErr:     true,
			wantContain: []string{"update boom"},
		},
		{
			name: "delete failure does not return error",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				// nowApp has a stale configmap not present in newApp -> delete path
				nowApp.SetConfigMap(configMapForTest("cm-stale"))
				client.PrependReactor("delete", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("delete boom")
				})
			},
			wantErr: false,
		},
		{
			name: "multiple create failures aggregate",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				newApp.SetConfigMap(configMapForTest("cm-a"))
				newApp.SetConfigMap(configMapForTest("cm-b"))
				client.PrependReactor("create", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
					cm := action.(k8stesting.CreateAction).GetObject().(*corev1.ConfigMap)
					return true, nil, errors.New("create failed for " + cm.Name)
				})
			},
			wantErr:     true,
			wantContain: []string{"cm-a", "cm-b"},
		},
		{
			name: "success returns nil",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				newApp.SetConfigMap(configMapForTest("cm-ok"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := k8sfake.NewSimpleClientset(namespace)
			nowApp := newAppWithConfigMaps(namespace)
			newApp := newAppWithConfigMaps(namespace)
			tt.setup(client, nowApp, newApp)

			controller := &upgradeController{
				manager: &Manager{client: client},
				ctx:     context.Background(),
			}
			err := controller.upgradeConfigMap(nowApp, *newApp)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if err != nil {
				for _, c := range tt.wantContain {
					if !strings.Contains(err.Error(), c) {
						t.Fatalf("expected error to contain %q, got %q", c, err.Error())
					}
				}
			}
		})
	}
}

// capability_id: rainbond.config-file.dependent-configmap-ownership
func TestUpgradeConfigMapLeavesExistingDependencyConfigMapToProvider(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	existing := configMapForTest("vm-cfg-provider")
	existing.Labels = map[string]string{"creator": "Rainbond", "service_id": "provider-service"}
	existing.Data = map[string]string{"application.yaml": "owner=provider\n"}
	desired := existing.DeepCopy()
	desired.Annotations = map[string]string{appmtypes.ConfigFileScopeAnnotation: appmtypes.ConfigFileScopeDependent}
	desired.Data = map[string]string{"application.yaml": "owner=consumer-rendered\n"}

	client := k8sfake.NewSimpleClientset(namespace, existing.DeepCopy())
	nowApp := newAppWithConfigMaps(namespace)
	nowApp.ServiceID = "consumer-service"
	newApp := newAppWithConfigMaps(namespace, desired)
	newApp.ServiceID = "consumer-service"
	controller := &upgradeController{manager: &Manager{client: client}, ctx: context.Background()}
	if err := controller.upgradeConfigMap(nowApp, *newApp); err != nil {
		t.Fatalf("reference existing dependency configmap: %v", err)
	}
	unchanged, err := client.CoreV1().ConfigMaps("default").Get(context.Background(), existing.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get provider configmap: %v", err)
	}
	if unchanged.Data["application.yaml"] != "owner=provider\n" {
		t.Fatalf("expected consumer upgrade not to update provider configmap, got %#v", unchanged.Data)
	}
}

// capability_id: rainbond.config-file.dependent-configmap-ownership
func TestUpgradeConfigMapCreatesMissingDependencyConfigMapForProvider(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	desired := configMapForTest("vm-cfg-provider")
	desired.Labels = map[string]string{"creator": "Rainbond", "service_id": "provider-service"}
	desired.Annotations = map[string]string{appmtypes.ConfigFileScopeAnnotation: appmtypes.ConfigFileScopeDependent}
	client := k8sfake.NewSimpleClientset(namespace)
	nowApp := newAppWithConfigMaps(namespace)
	nowApp.ServiceID = "consumer-service"
	newApp := newAppWithConfigMaps(namespace, desired)
	newApp.ServiceID = "consumer-service"

	controller := &upgradeController{manager: &Manager{client: client}, ctx: context.Background()}
	if err := controller.upgradeConfigMap(nowApp, *newApp); err != nil {
		t.Fatalf("create missing dependency configmap: %v", err)
	}
	created, err := client.CoreV1().ConfigMaps("default").Get(context.Background(), desired.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get dependency configmap created for provider: %v", err)
	}
	if created.Labels["service_id"] != "provider-service" {
		t.Fatalf("expected dependency configmap to belong to provider, got %#v", created.Labels)
	}
}

// capability_id: rainbond.config-file.dependent-configmap-ownership
func TestUpgradeConfigMapDoesNotDeleteProviderDependency(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	providerConfigMap := configMapForTest("vm-cfg-provider")
	providerConfigMap.Labels = map[string]string{"creator": "Rainbond", "service_id": "provider-service"}
	client := k8sfake.NewSimpleClientset(namespace, providerConfigMap.DeepCopy())
	nowApp := newAppWithConfigMaps(namespace, providerConfigMap.DeepCopy())
	nowApp.ServiceID = "consumer-service"
	newApp := newAppWithConfigMaps(namespace)
	newApp.ServiceID = "consumer-service"

	controller := &upgradeController{manager: &Manager{client: client}, ctx: context.Background()}
	if err := controller.upgradeConfigMap(nowApp, *newApp); err != nil {
		t.Fatalf("upgrade consumer without dependency: %v", err)
	}
	if _, err := client.CoreV1().ConfigMaps("default").Get(context.Background(), providerConfigMap.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("expected consumer not to delete provider configmap, got %v", err)
	}
}

// capability_id: rainbond.config-file.dependent-configmap-ownership
func TestUpgradeConfigMapReclaimsLegacyProviderConfigMap(t *testing.T) {
	tests := []struct {
		name              string
		existingServiceID string
	}{
		{name: "legacy consumer owned provider name", existingServiceID: "consumer-service"},
		{name: "same service cache miss", existingServiceID: "provider-service"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
			legacy := configMapForTest("vm-cfg-provider")
			legacy.Labels = map[string]string{"creator": "Rainbond", "service_id": tt.existingServiceID}
			legacy.Data = map[string]string{"application.yaml": "owner=old\n"}
			desired := configMapForTest(legacy.Name)
			desired.Labels = map[string]string{"creator": "Rainbond", "service_id": "provider-service"}
			desired.Annotations = map[string]string{appmtypes.ConfigFileScopeAnnotation: appmtypes.ConfigFileScopeOwned}
			desired.Data = map[string]string{"application.yaml": "owner=provider\n"}

			client := k8sfake.NewSimpleClientset(namespace, legacy.DeepCopy())
			nowApp := newAppWithConfigMaps(namespace)
			newApp := newAppWithConfigMaps(namespace, desired)
			controller := &upgradeController{manager: &Manager{client: client}, ctx: context.Background()}
			if err := controller.upgradeConfigMap(nowApp, *newApp); err != nil {
				t.Fatalf("reconcile provider configmap after cache miss: %v", err)
			}
			updated, err := client.CoreV1().ConfigMaps("default").Get(context.Background(), legacy.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("get reconciled provider configmap: %v", err)
			}
			if updated.Labels["service_id"] != "provider-service" || updated.Data["application.yaml"] != "owner=provider\n" {
				t.Fatalf("expected provider to own updated configmap, got labels=%#v data=%#v", updated.Labels, updated.Data)
			}
		})
	}
}

// capability_id: rainbond.config-file.dependent-configmap-ownership
func TestUpgradeConfigMapDoesNotAdoptForeignCollision(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	foreign := configMapForTest("vm-cfg-provider")
	foreign.Labels = map[string]string{"creator": "external", "service_id": "other-service"}
	desired := configMapForTest(foreign.Name)
	desired.Labels = map[string]string{"creator": "Rainbond", "service_id": "provider-service"}
	desired.Annotations = map[string]string{appmtypes.ConfigFileScopeAnnotation: appmtypes.ConfigFileScopeOwned}

	client := k8sfake.NewSimpleClientset(namespace, foreign.DeepCopy())
	controller := &upgradeController{manager: &Manager{client: client}, ctx: context.Background()}
	err := controller.upgradeConfigMap(newAppWithConfigMaps(namespace), *newAppWithConfigMaps(namespace, desired))
	if err == nil {
		t.Fatal("expected foreign configmap collision to fail")
	}
	unchanged, getErr := client.CoreV1().ConfigMaps("default").Get(context.Background(), foreign.Name, metav1.GetOptions{})
	if getErr != nil {
		t.Fatalf("get foreign configmap after failed upgrade: %v", getErr)
	}
	if unchanged.Labels["service_id"] != "other-service" || unchanged.Data["k"] != "v" {
		t.Fatalf("expected foreign configmap to remain unchanged, got labels=%#v data=%#v", unchanged.Labels, unchanged.Data)
	}
}

// capability_id: rainbond.upgrade-service-aggregates-create-update-errors
func TestUpgradeServiceErrorAggregation(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	tests := []struct {
		name        string
		setup       func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService)
		wantErr     bool
		wantContain []string
	}{
		{
			name: "create failure returns error",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				newApp.SetService(serviceForTest("svc-new"))
				client.PrependReactor("create", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("svc create boom")
				})
			},
			wantErr:     true,
			wantContain: []string{"svc create boom"},
		},
		{
			name: "update failure returns error",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				nowApp.SetService(serviceForTest("svc-shared"))
				newApp.SetService(serviceForTest("svc-shared"))
				client.PrependReactor("update", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("svc update boom")
				})
			},
			wantErr:     true,
			wantContain: []string{"svc update boom"},
		},
		{
			name: "delete failure does not return error",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				nowApp.SetService(serviceForTest("svc-stale"))
				client.PrependReactor("delete", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("svc delete boom")
				})
			},
			wantErr: false,
		},
		{
			name: "multiple create failures aggregate",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				newApp.SetService(serviceForTest("svc-a"))
				newApp.SetService(serviceForTest("svc-b"))
				client.PrependReactor("create", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
					svc := action.(k8stesting.CreateAction).GetObject().(*corev1.Service)
					return true, nil, errors.New("create failed for " + svc.Name)
				})
			},
			wantErr:     true,
			wantContain: []string{"svc-a", "svc-b"},
		},
		{
			name: "success returns nil",
			setup: func(client *k8sfake.Clientset, nowApp, newApp *appmtypes.AppService) {
				newApp.SetService(serviceForTest("svc-ok"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := k8sfake.NewSimpleClientset(namespace)
			nowApp := newAppWithServices(namespace)
			newApp := newAppWithServices(namespace)
			tt.setup(client, nowApp, newApp)

			controller := &upgradeController{
				manager: &Manager{client: client},
				ctx:     context.Background(),
			}
			err := controller.upgradeService(nowApp, *newApp)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if err != nil {
				for _, c := range tt.wantContain {
					if !strings.Contains(err.Error(), c) {
						t.Fatalf("expected error to contain %q, got %q", c, err.Error())
					}
				}
			}
		})
	}
}

func TestTruncateErr(t *testing.T) {
	if got := truncateErr(nil, 10); got != "" {
		t.Fatalf("expected empty string for nil err, got %q", got)
	}
	if got := truncateErr(errors.New("short"), 1024); got != "short" {
		t.Fatalf("expected %q, got %q", "short", got)
	}
	long := errors.New(strings.Repeat("x", 2000))
	if got := truncateErr(long, 1024); len(got) != 1024 {
		t.Fatalf("expected truncated length 1024, got %d", len(got))
	}
	// A multi-byte rune straddling the byte boundary must be trimmed so the
	// result stays valid UTF-8 and never exceeds max bytes.
	multibyte := errors.New(strings.Repeat("世", 100)) // each rune is 3 bytes
	if got := truncateErr(multibyte, 10); !utf8.ValidString(got) || len(got) > 10 {
		t.Fatalf("expected valid UTF-8 within 10 bytes, got %q (len=%d, valid=%v)", got, len(got), utf8.ValidString(got))
	}
}
