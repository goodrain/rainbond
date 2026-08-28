package volume

import (
	"encoding/json"
	"testing"

	api_model "github.com/goodrain/rainbond/api/model"
	appmtypes "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPluginStorageVolumeUsesConfiguredPVCSettings(t *testing.T) {
	app := newPluginStorageTestApp()
	pluginVolume := &PluginStorageVolume{
		Plugin: mustPluginStorageFromJSON(t, `{
			"volume_name":"data",
			"volume_path":"/data",
			"attr_type":"storage",
			"volume_type":"fast-sc",
			"volume_capacity":25,
			"access_mode":"RWX"
		}`),
		PluginID: "plugin-id",
		AS:       app,
	}

	if err := pluginVolume.CreateVolume(&Define{}); err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	claim := requireSinglePluginStorageClaim(t, app)
	if claim.Spec.StorageClassName == nil {
		t.Fatal("StorageClassName = nil, want fast-sc")
	}
	if *claim.Spec.StorageClassName != "fast-sc" {
		t.Fatalf("StorageClassName = %q, want fast-sc", *claim.Spec.StorageClassName)
	}
	if got := claim.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("25Gi")) != 0 {
		t.Errorf("storage request = %s, want 25Gi", got.String())
	}
	if len(claim.Spec.AccessModes) != 1 || claim.Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Errorf("access modes = %v, want [%s]", claim.Spec.AccessModes, corev1.ReadWriteMany)
	}
}

func TestPluginStorageVolumeKeepsLegacyPVCDefaults(t *testing.T) {
	app := newPluginStorageTestApp()
	pluginVolume := &PluginStorageVolume{
		Plugin: mustPluginStorageFromJSON(t, `{
			"volume_name":"data",
			"volume_path":"/data",
			"attr_type":"storage"
		}`),
		PluginID: "plugin-id",
		AS:       app,
	}

	if err := pluginVolume.CreateVolume(&Define{}); err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	claim := requireSinglePluginStorageClaim(t, app)
	if claim.Spec.StorageClassName == nil {
		t.Fatal("StorageClassName = nil, want local-path")
	}
	if *claim.Spec.StorageClassName != "local-path" {
		t.Fatalf("StorageClassName = %q, want local-path", *claim.Spec.StorageClassName)
	}
	if got := claim.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("10Gi")) != 0 {
		t.Errorf("storage request = %s, want 10Gi", got.String())
	}
	if len(claim.Spec.AccessModes) != 1 || claim.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("access modes = %v, want [%s]", claim.Spec.AccessModes, corev1.ReadWriteOnce)
	}
}

func mustPluginStorageFromJSON(t *testing.T, raw string) api_model.PluginStorage {
	t.Helper()
	var storage api_model.PluginStorage
	if err := json.Unmarshal([]byte(raw), &storage); err != nil {
		t.Fatalf("unmarshal plugin storage: %v", err)
	}
	return storage
}

func newPluginStorageTestApp() *appmtypes.AppService {
	app := &appmtypes.AppService{
		AppServiceBase: appmtypes.AppServiceBase{
			ServiceID:    "service-id",
			ServiceAlias: "demo",
			TenantID:     "tenant-id",
			TenantName:   "team-a",
			AppID:        "app-id",
			K8sApp:       "app",
		},
	}
	app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}})
	return app
}

func requireSinglePluginStorageClaim(t *testing.T, app *appmtypes.AppService) *corev1.PersistentVolumeClaim {
	t.Helper()
	claims := app.GetClaims()
	if len(claims) != 1 {
		t.Fatalf("claim count = %d, want 1", len(claims))
	}
	return claims[0]
}
