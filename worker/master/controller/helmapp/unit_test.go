package helmapp

import (
	"context"
	"testing"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestHelmApp() *v1alpha1.HelmApp {
	return &v1alpha1.HelmApp{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: v1alpha1.HelmAppSpec{
			EID:          "eid-1",
			TemplateName: "mysql",
			Version:      "1.0.0",
			AppStore: &v1alpha1.HelmAppStore{
				Name: "bitnami",
				URL:  "https://charts.bitnami.com/bitnami",
			},
		},
		Status: v1alpha1.HelmAppStatus{},
	}
}

// capability_id: rainbond.worker.helmapp.chart-ref
func TestAppChart(t *testing.T) {
	app := &App{
		repoName:        "bitnami",
		templateName:    "mysql",
		helmApp:         newTestHelmApp(),
		originalHelmApp: newTestHelmApp(),
	}

	if got := app.Chart(); got != "bitnami/mysql" {
		t.Fatalf("expected chart ref bitnami/mysql, got %q", got)
	}
}

// capability_id: rainbond.worker.helmapp.setup-required
func TestAppNeedSetup(t *testing.T) {
	app := &App{helmApp: newTestHelmApp()}
	if !app.NeedSetup() {
		t.Fatal("expected setup to be required for empty spec/status")
	}

	helmApp := newTestHelmApp()
	helmApp.Spec.PreStatus = v1alpha1.HelmAppPreStatusNotConfigured
	helmApp.Status.Phase = v1alpha1.HelmAppStatusPhaseDetecting
	for _, typ := range defaultConditionTypes {
		helmApp.Status.UpdateConditionStatus(typ, corev1.ConditionFalse)
	}
	app = &App{helmApp: helmApp}
	if app.NeedSetup() {
		t.Fatal("expected setup to be skipped once defaults are present")
	}
}

// capability_id: rainbond.worker.helmapp.detect-required
func TestAppNeedDetect(t *testing.T) {
	helmApp := newTestHelmApp()
	app := &App{helmApp: helmApp}
	if !app.NeedDetect() {
		t.Fatal("expected detect when conditions are missing")
	}

	for _, typ := range []v1alpha1.HelmAppConditionType{
		v1alpha1.HelmAppChartReady,
		v1alpha1.HelmAppPreInstalled,
	} {
		helmApp.Status.UpdateConditionStatus(typ, corev1.ConditionTrue)
	}
	if app.NeedDetect() {
		t.Fatal("expected detect to be skipped when prerequisites are true")
	}
}

// capability_id: rainbond.worker.helmapp.update-required
func TestAppNeedUpdate(t *testing.T) {
	helmApp := newTestHelmApp()
	helmApp.Spec.PreStatus = v1alpha1.HelmAppPreStatusNotConfigured
	app := &App{helmApp: helmApp}
	if app.NeedUpdate() {
		t.Fatal("did not expect update before helm app is configured")
	}

	helmApp.Spec.PreStatus = v1alpha1.HelmAppPreStatusConfigured
	helmApp.Status.CurrentVersion = helmApp.Spec.Version
	helmApp.Status.Overrides = []string{"a=1"}
	helmApp.Spec.Overrides = []string{"a=1"}
	if app.NeedUpdate() {
		t.Fatal("did not expect update when version and overrides match")
	}

	helmApp.Spec.Overrides = []string{"a=2"}
	if !app.NeedUpdate() {
		t.Fatal("expected update when overrides change")
	}
}

// capability_id: rainbond.worker.helmapp.phase-derive
func TestStatusGetPhase(t *testing.T) {
	helmApp := newTestHelmApp()
	status := NewStatus(context.Background(), helmApp, nil)

	if phase := status.getPhase(); phase != v1alpha1.HelmAppStatusPhaseDetecting {
		t.Fatalf("expected detecting phase, got %q", phase)
	}

	for _, typ := range []v1alpha1.HelmAppConditionType{
		v1alpha1.HelmAppChartReady,
		v1alpha1.HelmAppPreInstalled,
	} {
		helmApp.Status.UpdateConditionStatus(typ, corev1.ConditionTrue)
	}
	if phase := status.getPhase(); phase != v1alpha1.HelmAppStatusPhaseConfiguring {
		t.Fatalf("expected configuring phase, got %q", phase)
	}

	helmApp.Spec.PreStatus = v1alpha1.HelmAppPreStatusConfigured
	if phase := status.getPhase(); phase != v1alpha1.HelmAppStatusPhaseInstalling {
		t.Fatalf("expected installing phase, got %q", phase)
	}

	helmApp.Status.UpdateConditionStatus(v1alpha1.HelmAppInstalled, corev1.ConditionTrue)
	if phase := status.getPhase(); phase != v1alpha1.HelmAppStatusPhaseInstalled {
		t.Fatalf("expected installed phase, got %q", phase)
	}
}

// capability_id: rainbond.worker.helmapp.detected-prerequisites
func TestStatusIsDetected(t *testing.T) {
	helmApp := newTestHelmApp()
	status := NewStatus(context.Background(), helmApp, nil)
	if status.isDetected() {
		t.Fatal("expected isDetected to be false before conditions are ready")
	}

	helmApp.Status.UpdateConditionStatus(v1alpha1.HelmAppChartReady, corev1.ConditionTrue)
	if status.isDetected() {
		t.Fatal("expected isDetected to stay false until all prerequisites are ready")
	}

	helmApp.Status.UpdateConditionStatus(v1alpha1.HelmAppPreInstalled, corev1.ConditionTrue)
	if !status.isDetected() {
		t.Fatal("expected isDetected to become true")
	}
}

// capability_id: rainbond.worker.helmapp.queue-key-parse
func TestNameNamespace(t *testing.T) {
	name, ns := nameNamespace("demo/default")
	if name != "demo" || ns != "default" {
		t.Fatalf("unexpected split result: name=%q ns=%q", name, ns)
	}
}
