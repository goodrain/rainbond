package handler

import (
	"context"
	"time"

	"github.com/goodrain/rainbond/api/model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// addRainbondComponentReplicaStatus preserves the legacy Pod inventory and only
// supplements rollout history whose entire ownership chain can be verified.
func (c *clusterAction) addRainbondComponentReplicaStatus(ctx context.Context, namespace string, components []*model.RainbondComponent) {
	// Share a short optional budget so slow owner lookups cannot consume the
	// main request's five seconds or exceed Console's shorter read timeout.
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	// Cache successful and failed lookups for this request, never across requests.
	replicaSets := make(map[string]*appsv1.ReplicaSet)
	deployments := make(map[string]*appsv1.Deployment)
	for _, component := range components {
		candidate := false
		for _, pod := range component.Pods {
			if terminalComponentPod(pod) && componentControllerOwner(pod.OwnerReferences, "ReplicaSet") != nil {
				candidate = true
				break
			}
		}
		if !candidate {
			continue
		}
		component.ReplicaStatus = c.verifiedComponentReplicas(ctx, namespace, component.Pods, replicaSets, deployments)
	}
}

func (c *clusterAction) verifiedComponentReplicas(ctx context.Context, namespace string, pods []corev1.Pod, replicaSets map[string]*appsv1.ReplicaSet, deployments map[string]*appsv1.Deployment) *model.RainbondComponentReplicas {
	// Reject mixed or unknown workloads before making optional API requests.
	for _, pod := range pods {
		if pod.Namespace != namespace || componentControllerOwner(pod.OwnerReferences, "ReplicaSet") == nil {
			return nil
		}
	}
	var deploymentOwner *metav1.OwnerReference
	status := &model.RainbondComponentReplicas{}
	for _, pod := range pods {
		if ctx.Err() != nil {
			return nil
		}
		owner := componentControllerOwner(pod.OwnerReferences, "ReplicaSet")
		rs, found := replicaSets[owner.Name]
		if !found {
			var err error
			rs, err = c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err != nil {
				rs = nil
			}
			replicaSets[owner.Name] = rs
		}
		if rs == nil || rs.Namespace != namespace || rs.Name != owner.Name || rs.UID != owner.UID {
			return nil
		}
		controller := componentControllerOwner(rs.OwnerReferences, "Deployment")
		if controller == nil {
			return nil
		}
		if deploymentOwner == nil {
			deploymentOwner = controller
		} else if deploymentOwner.Name != controller.Name || deploymentOwner.UID != controller.UID {
			return nil
		}
		if !terminalComponentPod(pod) {
			status.Total++
			if podReadyForOverview(pod) {
				status.Ready++
			}
		}
	}
	if deploymentOwner == nil || ctx.Err() != nil {
		return nil
	}
	deployment, found := deployments[deploymentOwner.Name]
	if !found {
		var err error
		deployment, err = c.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentOwner.Name, metav1.GetOptions{})
		if err != nil {
			deployment = nil
		}
		deployments[deploymentOwner.Name] = deployment
	}
	if ctx.Err() != nil || deployment == nil || deployment.Namespace != namespace || deployment.Name != deploymentOwner.Name ||
		deployment.UID != deploymentOwner.UID || deployment.DeletionTimestamp != nil ||
		deployment.Status.ObservedGeneration < deployment.Generation || deployment.Spec.Replicas == nil || *deployment.Spec.Replicas <= 0 {
		return nil
	}
	if desired := int(*deployment.Spec.Replicas); desired > status.Total {
		status.Total = desired
	}
	return status
}

func terminalComponentPod(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}

func componentControllerOwner(owners []metav1.OwnerReference, kind string) *metav1.OwnerReference {
	var controller *metav1.OwnerReference
	for i := range owners {
		owner := &owners[i]
		if owner.Controller == nil || !*owner.Controller {
			continue
		}
		if controller != nil || owner.APIVersion != "apps/v1" || owner.Kind != kind || owner.Name == "" || owner.UID == "" {
			return nil
		}
		controller = owner
	}
	return controller
}
