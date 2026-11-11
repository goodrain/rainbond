package conversion

import (
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// applyDeploymentUpgradeStrategy prevents overlapping writers to an exclusive claim.
func applyDeploymentUpgradeStrategy(app *typesv1.AppService) {
	deployment := app.GetDeployment()
	if deployment == nil {
		return
	}
	if deployment.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType {
		deployment.Spec.Strategy.RollingUpdate = nil
		return
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
		return
	}
	claims := make(map[string]*corev1.PersistentVolumeClaim)
	for _, list := range [][]*corev1.PersistentVolumeClaim{app.GetClaims(), app.GetClaimsManually()} {
		for _, claim := range list {
			if claim != nil {
				claims[claim.Name] = claim
			}
		}
	}
	writable := make(map[string]bool)
	for _, containers := range [][]corev1.Container{
		deployment.Spec.Template.Spec.Containers, deployment.Spec.Template.Spec.InitContainers,
	} {
		for _, container := range containers {
			for _, mount := range container.VolumeMounts {
				if !mount.ReadOnly {
					writable[mount.Name] = true
				}
			}
		}
	}
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		source := volume.PersistentVolumeClaim
		if source == nil || source.ReadOnly || !writable[volume.Name] {
			continue
		}
		claim := claims[source.ClaimName]
		if claim == nil || len(claim.Spec.AccessModes) != 1 {
			continue
		}
		mode := claim.Spec.AccessModes[0]
		if mode == corev1.ReadWriteOnce || mode == corev1.ReadWriteOncePod {
			deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
			return
		}
	}
}
