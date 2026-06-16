package conversion

import (
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// capability_id: rainbond.worker.conversion.daemonset-workload
func TestInitBaseDaemonSetCreatesDaemonSetWorkload(t *testing.T) {
	app := &typesv1.AppService{
		AppServiceBase: typesv1.AppServiceBase{
			AppID:            "app-1",
			ServiceID:        "service-1",
			ServiceAlias:     "node-agent",
			K8sApp:           "demo",
			K8sComponentName: "node-agent",
			DeployVersion:    "v1",
			CreaterID:        "creator-1",
		},
	}
	app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}})

	service := &dbmodel.TenantServices{
		TenantID:         "tenant-1",
		ServiceID:        "service-1",
		ServiceAlias:     "node-agent",
		K8sComponentName: "node-agent",
		DeployVersion:    "v1",
	}

	initBaseDaemonSet(app, service)

	if app.ServiceType != typesv1.TypeDaemonSet {
		t.Fatalf("expected app service type %q, got %q", typesv1.TypeDaemonSet, app.ServiceType)
	}
	daemonSet := app.GetDaemonSet()
	if daemonSet == nil {
		t.Fatalf("expected daemonset workload to be initialized")
	}
	if daemonSet.Name != "demo-node-agent" {
		t.Fatalf("expected daemonset name %q, got %q", "demo-node-agent", daemonSet.Name)
	}
	if daemonSet.Namespace != "team-a" {
		t.Fatalf("expected daemonset namespace %q, got %q", "team-a", daemonSet.Namespace)
	}
	if daemonSet.Spec.Selector == nil || daemonSet.Spec.Selector.MatchLabels["service_id"] != "service-1" {
		t.Fatalf("expected daemonset selector to include service_id")
	}
	if daemonSet.Labels["version"] != "v1" {
		t.Fatalf("expected daemonset labels to include version")
	}
}
