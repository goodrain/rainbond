package conversion

import (
	"testing"

	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// capability_id: rainbond.worker.deployment.single-writer-upgrade
func TestDeploymentUpgradeStrategyProtectsSingleWriterClaims(t *testing.T) {
	for _, tc := range []struct {
		name          string
		mode          corev1.PersistentVolumeAccessMode
		replicas      int32
		claimReadOnly bool
		mountReadOnly bool
		missingClaim  bool
		missingMount  bool
		want          appsv1.DeploymentStrategyType
	}{
		{name: "single RWO writer", mode: corev1.ReadWriteOnce, replicas: 1, want: appsv1.RecreateDeploymentStrategyType},
		{name: "single RWOP writer", mode: corev1.ReadWriteOncePod, replicas: 1, want: appsv1.RecreateDeploymentStrategyType},
		{name: "shared writable claim", mode: corev1.ReadWriteMany, replicas: 1, want: appsv1.RollingUpdateDeploymentStrategyType},
		{name: "read only access mode", mode: corev1.ReadOnlyMany, replicas: 1, want: appsv1.RollingUpdateDeploymentStrategyType},
		{name: "read only claim", mode: corev1.ReadWriteOnce, replicas: 1, claimReadOnly: true, want: appsv1.RollingUpdateDeploymentStrategyType},
		{name: "read only mount", mode: corev1.ReadWriteOnce, replicas: 1, mountReadOnly: true, want: appsv1.RollingUpdateDeploymentStrategyType},
		{name: "multiple replicas", mode: corev1.ReadWriteOnce, replicas: 2, want: appsv1.RollingUpdateDeploymentStrategyType},
		{name: "unknown claim", mode: corev1.ReadWriteOnce, replicas: 1, missingClaim: true, want: appsv1.RollingUpdateDeploymentStrategyType},
		{name: "unmounted claim", mode: corev1.ReadWriteOnce, replicas: 1, missingMount: true, want: appsv1.RollingUpdateDeploymentStrategyType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			maxSurge := intstr.FromInt(1)
			deployment := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{
				Replicas: &tc.replicas,
				Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: &maxSurge}},
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data", ReadOnly: tc.claimReadOnly}}}},
					Containers: []corev1.Container{{Name: "app", VolumeMounts: []corev1.VolumeMount{
						{Name: "data", MountPath: "/data", ReadOnly: tc.mountReadOnly}}}},
				}},
			}}
			if tc.missingMount {
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = nil
			}
			app := &typesv1.AppService{}
			app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team"}})
			app.SetDeployment(deployment)
			if !tc.missingClaim {
				app.SetClaimManually(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{tc.mode}}})
			}
			applyDeploymentUpgradeStrategy(app)
			if deployment.Spec.Strategy.Type != tc.want {
				t.Fatalf("strategy = %s, want %s", deployment.Spec.Strategy.Type, tc.want)
			}
			if tc.want == appsv1.RecreateDeploymentStrategyType && deployment.Spec.Strategy.RollingUpdate != nil {
				t.Fatal("Recreate must not retain rollingUpdate settings")
			}
		})
	}
}

// capability_id: rainbond.worker.deployment.single-writer-upgrade
func TestDeploymentUpgradeStrategyKeepsExplicitRecreate(t *testing.T) {
	app := &typesv1.AppService{}
	replicas := int32(1)
	app.SetDeployment(&appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: &replicas, Strategy: appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType, RollingUpdate: &appsv1.RollingUpdateDeployment{},
	}}})
	applyDeploymentUpgradeStrategy(app)
	if app.GetDeployment().Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType ||
		app.GetDeployment().Spec.Strategy.RollingUpdate != nil {
		t.Fatal("explicit Recreate must be retained without rollingUpdate")
	}
	applyDeploymentUpgradeStrategy(&typesv1.AppService{})
}
