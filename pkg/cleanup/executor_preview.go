package cleanup

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
)

// Preserve authority-bearing fields across admission while allowing Kubernetes'
// ordinary image/termination-log defaults and resource quota defaults.
func sameExecutorPreview(original, observed corev1.PodSpec) bool {
	if original.NodeName != observed.NodeName || !reflect.DeepEqual(original.NodeSelector, observed.NodeSelector) || len(original.Containers) != 1 || len(observed.Containers) != 1 || !reflect.DeepEqual(original.Volumes, observed.Volumes) || !reflect.DeepEqual(original.ImagePullSecrets, observed.ImagePullSecrets) || !reflect.DeepEqual(original.HostAliases, observed.HostAliases) || !reflect.DeepEqual(original.DNSConfig, observed.DNSConfig) {
		return false
	}
	account := func(value string) string {
		if value == "" {
			return "default"
		}
		return value
	}
	if account(original.ServiceAccountName) != account(observed.ServiceAccountName) || (observed.DeprecatedServiceAccount != "" && observed.DeprecatedServiceAccount != account(observed.ServiceAccountName)) {
		return false
	}
	podSecurity := func(value *corev1.PodSecurityContext) *corev1.PodSecurityContext {
		if value != nil && reflect.DeepEqual(*value, corev1.PodSecurityContext{}) {
			return nil
		}
		return value
	}
	if !reflect.DeepEqual(podSecurity(original.SecurityContext), podSecurity(observed.SecurityContext)) {
		return false
	}
	before, after := original.Containers[0].DeepCopy(), observed.Containers[0].DeepCopy()
	if before.ImagePullPolicy == "" && after.ImagePullPolicy == corev1.PullIfNotPresent {
		after.ImagePullPolicy = ""
	}
	if before.TerminationMessagePath == "" && after.TerminationMessagePath == "/dev/termination-log" {
		after.TerminationMessagePath = ""
	}
	if before.TerminationMessagePolicy == "" && after.TerminationMessagePolicy == corev1.TerminationMessageReadFile {
		after.TerminationMessagePolicy = ""
	}
	if before.SecurityContext != nil && reflect.DeepEqual(*before.SecurityContext, corev1.SecurityContext{}) {
		before.SecurityContext = nil
	}
	if after.SecurityContext != nil && reflect.DeepEqual(*after.SecurityContext, corev1.SecurityContext{}) {
		after.SecurityContext = nil
	}
	after.Resources = before.Resources
	return reflect.DeepEqual(before, after)
}
