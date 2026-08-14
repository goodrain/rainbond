// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const serviceIDLabel = "service_id"

// CreateKubeService creates missing Services and reconciles existing Rainbond-owned Services.
func CreateKubeService(client kubernetes.Interface, namespace string, services ...*corev1.Service) error {
	for _, desired := range services {
		if desired == nil || desired.Name == "" {
			return fmt.Errorf("service name is required")
		}
		if desired.Labels[serviceIDLabel] == "" {
			return fmt.Errorf("service %s/%s has no %q ownership label", namespace, desired.Name, serviceIDLabel)
		}
		if err := ensureKubeService(client, namespace, desired); err != nil {
			return fmt.Errorf("ensure service %s/%s: %w", namespace, desired.Name, err)
		}
	}
	return nil
}

func ensureKubeService(client kubernetes.Interface, namespace string, desired *corev1.Service) error {
	ctx := context.Background()
	return retry.OnError(retry.DefaultRetry, isRetryableServiceEnsureError, func() error {
		existing, err := client.CoreV1().Services(namespace).Get(ctx, desired.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.CoreV1().Services(namespace).Create(ctx, desired.DeepCopy(), metav1.CreateOptions{})
			return err
		}
		if err != nil {
			return err
		}
		if err := validateServiceOwnership(existing, desired); err != nil {
			return err
		}
		if err := validateClusterIPMode(existing, desired); err != nil {
			return err
		}

		updated := reconciledService(existing, desired)
		_, err = client.CoreV1().Services(namespace).Update(ctx, updated, metav1.UpdateOptions{})
		return err
	})
}

func isRetryableServiceEnsureError(err error) bool {
	return errors.IsAlreadyExists(err) || errors.IsConflict(err) || errors.IsNotFound(err)
}

func validateServiceOwnership(existing, desired *corev1.Service) error {
	desiredServiceID := desired.Labels[serviceIDLabel]
	existingServiceID := existing.Labels[serviceIDLabel]
	if desiredServiceID == "" {
		return fmt.Errorf("desired service has no %q ownership label", serviceIDLabel)
	}
	if existingServiceID != desiredServiceID {
		return fmt.Errorf("existing service belongs to service %q, expected %q", existingServiceID, desiredServiceID)
	}
	return nil
}

func validateClusterIPMode(existing, desired *corev1.Service) error {
	existingHeadless := existing.Spec.ClusterIP == corev1.ClusterIPNone
	desiredHeadless := desired.Spec.ClusterIP == corev1.ClusterIPNone
	if existingHeadless != desiredHeadless {
		return fmt.Errorf("cluster IP mode is immutable: existing=%q desired=%q", existing.Spec.ClusterIP, desired.Spec.ClusterIP)
	}
	return nil
}

func reconciledService(existing, desired *corev1.Service) *corev1.Service {
	updated := existing.DeepCopy()
	desiredCopy := desired.DeepCopy()

	updated.Labels = mergeStringMap(existing.Labels, desired.Labels)
	updated.Annotations = mergeStringMap(existing.Annotations, desired.Annotations)
	updated.Spec = desiredCopy.Spec
	updated.Spec.ClusterIP = existing.Spec.ClusterIP
	updated.Spec.ClusterIPs = append([]string(nil), existing.Spec.ClusterIPs...)
	updated.Spec.IPFamilies = append([]corev1.IPFamily(nil), existing.Spec.IPFamilies...)
	if existing.Spec.IPFamilyPolicy != nil {
		policy := *existing.Spec.IPFamilyPolicy
		updated.Spec.IPFamilyPolicy = &policy
	}
	if updated.Spec.Type == corev1.ServiceTypeLoadBalancer && updated.Spec.HealthCheckNodePort == 0 {
		updated.Spec.HealthCheckNodePort = existing.Spec.HealthCheckNodePort
	}
	if updated.Spec.Type == corev1.ServiceTypeNodePort || updated.Spec.Type == corev1.ServiceTypeLoadBalancer {
		preserveAllocatedNodePorts(updated.Spec.Ports, existing.Spec.Ports)
	}
	return updated
}

func mergeStringMap(existing, desired map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(desired))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range desired {
		merged[key] = value
	}
	return merged
}

func preserveAllocatedNodePorts(desired, existing []corev1.ServicePort) {
	for desiredIndex := range desired {
		if desired[desiredIndex].NodePort != 0 {
			continue
		}
		for existingIndex := range existing {
			if servicePortsShareIdentity(desired[desiredIndex], existing[existingIndex]) {
				desired[desiredIndex].NodePort = existing[existingIndex].NodePort
				break
			}
		}
	}
}

func servicePortsShareIdentity(desired, existing corev1.ServicePort) bool {
	if desired.Protocol != existing.Protocol {
		return false
	}
	if desired.Name != "" || existing.Name != "" {
		return desired.Name == existing.Name
	}
	return desired.Port == existing.Port
}
