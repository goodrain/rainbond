package vmrestore

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

// nativeAdvancedSelection covers the fields currently rendered from Region
// configuration. Comparing only live VM A would miss an unapplied DB edit B.
type nativeAdvancedSelection struct {
	Labels           map[string]string    `json:"labels,omitempty"`
	Annotations      map[string]string    `json:"annotations,omitempty"`
	NodeSelector     map[string]string    `json:"node_selector,omitempty"`
	Affinity         *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations      []corev1.Toleration  `json:"tolerations,omitempty"`
	DNSPolicy        corev1.DNSPolicy     `json:"dns_policy,omitempty"`
	DNSConfig        *corev1.PodDNSConfig `json:"dns_config,omitempty"`
	SchedulerName    string               `json:"scheduler_name,omitempty"`
	Hostname         string               `json:"hostname,omitempty"`
	Readiness        *kv.Probe            `json:"readiness,omitempty"`
	Liveness         *kv.Probe            `json:"liveness,omitempty"`
	TerminationGrace *int64               `json:"termination_grace,omitempty"`
	GPUs             []kv.GPU             `json:"gpus,omitempty"`
	HostDevices      []kv.HostDevice      `json:"host_devices,omitempty"`
}

func nativeRestoreSourceVM(source *native.VirtualMachine) *kv.VirtualMachine {
	if source == nil {
		return nil
	}
	return &kv.VirtualMachine{ObjectMeta: *source.ObjectMeta.DeepCopy(), Spec: *source.Spec.DeepCopy()}
}

func nativeAdvanced(vm *kv.VirtualMachine) (nativeAdvancedSelection, error) {
	if vm == nil || vm.Spec.Template == nil {
		return nativeAdvancedSelection{}, ErrNativeRestoreConfigurationChanged
	}
	s := vm.Spec.Template.Spec
	result := nativeAdvancedSelection{Labels: vm.Spec.Template.ObjectMeta.Labels, Annotations: vm.Spec.Template.ObjectMeta.Annotations, NodeSelector: s.NodeSelector, Affinity: s.Affinity, Tolerations: s.Tolerations, DNSPolicy: s.DNSPolicy, DNSConfig: s.DNSConfig, SchedulerName: s.SchedulerName, Hostname: s.Hostname, Readiness: s.ReadinessProbe, Liveness: s.LivenessProbe, TerminationGrace: s.TerminationGracePeriodSeconds, GPUs: s.Domain.Devices.GPUs, HostDevices: s.Domain.Devices.HostDevices}
	// This label is regenerated on every pure Region render. Ignore only that
	// producer timestamp in the comparison, leaving both caller metadata maps
	// and every stable/user label intact.
	result.Labels = map[string]string{}
	for key, value := range vm.Spec.Template.ObjectMeta.Labels {
		if key != "creater_id" {
			result.Labels[key] = value
		}
	}
	result.Readiness = normalizedNativeProbe(s.ReadinessProbe)
	result.Liveness = normalizedNativeProbe(s.LivenessProbe)
	if result.DNSPolicy == "" {
		result.DNSPolicy = corev1.DNSClusterFirst
	}
	if result.Affinity != nil && reflect.DeepEqual(*result.Affinity, corev1.Affinity{}) {
		result.Affinity = nil
	}
	if result.DNSConfig != nil && reflect.DeepEqual(*result.DNSConfig, corev1.PodDNSConfig{}) {
		result.DNSConfig = nil
	}
	return result, nil
}

func normalizedNativeProbe(probe *kv.Probe) *kv.Probe {
	if probe == nil {
		return nil
	}
	copy := probe.DeepCopy()
	kv.SetDefaults_Probe(copy)
	return copy
}

func nativeNetworkHash(vm *kv.VirtualMachine) (string, error) {
	if vm == nil || vm.Spec.Template == nil {
		return "", ErrNativeRestoreConfigurationChanged
	}
	return snapshotDigest(struct {
		Networks   []kv.Network
		Interfaces []kv.Interface
	}{vm.Spec.Template.Spec.Networks, vm.Spec.Template.Spec.Domain.Devices.Interfaces})
}

func (m *ManagedNativeRestore) checkAdvanced(ctx context.Context, scope Scope, source, current *kv.VirtualMachine) error {
	desired, err := m.BuildPlatform(ctx, scope)
	if err != nil {
		return ErrReferenceUnavailable
	}
	want, err := nativeAdvanced(source)
	if err != nil {
		return err
	}
	a, err := nativeAdvanced(current)
	if err != nil {
		return err
	}
	b, err := nativeAdvanced(desired)
	if err != nil {
		return err
	}
	wantHash, _ := snapshotDigest(want)
	aHash, _ := snapshotDigest(a)
	bHash, _ := snapshotDigest(b)
	if wantHash != aHash || wantHash != bHash {
		return ErrNativeRestoreConfigurationChanged
	}
	// Ordinary platform rendering explicitly retains the live network identity;
	// no inferred/default MAC is treated as a DB network-edit request. A whole-VM
	// restore may not silently replace an actually changed current network.
	sourceNetwork, _ := nativeNetworkHash(source)
	currentNetwork, _ := nativeNetworkHash(current)
	if sourceNetwork != currentNetwork {
		return ErrNativeRestoreConfigurationChanged
	}
	raw, err := m.Dynamic.Resource(restoreVMGVR).Namespace(scope.Namespace).Get(ctx, current.Name, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	if raw.GetUID() != current.UID || raw.GetResourceVersion() != current.ResourceVersion {
		return ErrReferenceUnavailable
	}
	networks, _, _ := unstructured.NestedSlice(raw.Object, "spec", "template", "spec", "networks")
	interfaces, _, _ := unstructured.NestedSlice(raw.Object, "spec", "template", "spec", "domain", "devices", "interfaces")
	data, err := json.Marshal(struct {
		Networks   interface{} `json:"networks"`
		Interfaces interface{} `json:"interfaces"`
	}{networks, interfaces})
	if err != nil {
		return ErrCorrupt
	}
	var known struct {
		Networks   []kv.Network   `json:"networks"`
		Interfaces []kv.Interface `json:"interfaces"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&known) != nil {
		return ErrNativeRestoreConfigurationChanged
	}
	return nil
}
