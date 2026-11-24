package vmrestore

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	kv "kubevirt.io/api/core/v1"
)

// PreserveRuntimeVM applies only the known before/after differences to the raw
// live VM. Fields outside those differences never pass through the older API.
// A replaced subtree with fields that cannot be represented is refused.
func PreserveRuntimeVM(raw *unstructured.Unstructured, before, after *kv.VirtualMachine) (*unstructured.Unstructured, error) {
	if raw == nil || before == nil || after == nil || before.UID == "" || before.ResourceVersion == "" || raw.GetUID() != before.UID || raw.GetResourceVersion() != before.ResourceVersion || raw.GetName() != before.Name || raw.GetNamespace() != before.Namespace || after.UID != before.UID || after.ResourceVersion != before.ResourceVersion || after.Name != before.Name || after.Namespace != before.Namespace {
		return nil, ErrConflict
	}
	old, err := EncodeRuntimeVM(before)
	if err != nil {
		return nil, err
	}
	wanted, err := EncodeRuntimeVM(after)
	if err != nil {
		return nil, err
	}
	var observed kv.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(raw.Object, &observed) != nil {
		return nil, ErrCorrupt
	}
	projection, err := EncodeRuntimeVM(&observed)
	if err != nil || !reflect.DeepEqual(projection.Object["spec"], old.Object["spec"]) {
		return nil, ErrConflict
	}
	for _, label := range []string{"tenant_id", "service_id"} {
		if raw.GetLabels()[label] != before.Labels[label] {
			return nil, ErrConflict
		}
	}
	result := raw.DeepCopy()
	for _, field := range []string{"spec"} {
		value, err := mergeRuntimeValue(result.Object[field], old.Object[field], wanted.Object[field], []string{field})
		if err != nil {
			return nil, err
		}
		result.Object[field] = value
	}
	return result, nil
}

func runtimeReplacementKnown(value interface{}, path []string) bool {
	var target interface{}
	switch strings.Join(path, ".") {
	case "spec.template.spec.volumes":
		target = &[]kv.Volume{}
	case "spec.dataVolumeTemplates":
		target = &[]kv.DataVolumeTemplateSpec{}
	case "spec.template.spec.domain.devices.disks":
		target = &[]kv.Disk{}
	case "spec.template.spec.domain.devices.interfaces":
		target = &[]kv.Interface{}
	case "spec.template.spec.domain.devices.gpus":
		target = &[]kv.GPU{}
	case "spec.template.spec.domain.devices.hostDevices":
		target = &[]kv.HostDevice{}
	default:
		return false
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target) == nil
}

func runtimeContainsUnrepresented(raw, known interface{}) bool {
	if list, ok := raw.([]interface{}); ok {
		projection, ok := known.([]interface{})
		if !ok || len(list) != len(projection) {
			return len(list) != 0
		}
		for i := range list {
			if runtimeContainsUnrepresented(list[i], projection[i]) {
				return true
			}
		}
		return false
	}
	object, ok := raw.(map[string]interface{})
	if !ok {
		return false
	}
	projection, ok := known.(map[string]interface{})
	if !ok {
		return len(object) != 0
	}
	for name, value := range object {
		other, present := projection[name]
		if !present || runtimeContainsUnrepresented(value, other) {
			return true
		}
	}
	return false
}

func mergeRuntimeValue(raw, before, after interface{}, path []string) (interface{}, error) {
	if reflect.DeepEqual(before, after) {
		return runtime.DeepCopyJSONValue(raw), nil
	}
	old, oldMap := before.(map[string]interface{})
	next, nextMap := after.(map[string]interface{})
	current, rawMap := raw.(map[string]interface{})
	if oldMap && nextMap && rawMap {
		result := runtime.DeepCopyJSONValue(current).(map[string]interface{})
		for name, value := range old {
			if _, exists := next[name]; !exists {
				removedPath := append(append([]string(nil), path...), name)
				_, list := current[name].([]interface{})
				if list && !runtimeReplacementKnown(current[name], removedPath) || !list && runtimeContainsUnrepresented(current[name], value) {
					return nil, ErrRestoreCapability
				}
				delete(result, name)
			}
		}
		for name, value := range next {
			previous, exists := old[name]
			if exists {
				merged, err := mergeRuntimeValue(current[name], previous, value, append(append([]string(nil), path...), name))
				if err != nil {
					return nil, err
				}
				result[name] = merged
			} else {
				// A known addition must not overwrite an unrepresented live value.
				if existing, present := current[name]; present && runtimeContainsUnrepresented(existing, nil) {
					return nil, ErrRestoreCapability
				}
				result[name] = runtime.DeepCopyJSONValue(value)
			}
		}
		return result, nil
	}
	if _, list := raw.([]interface{}); list {
		if !runtimeReplacementKnown(raw, path) {
			return nil, ErrRestoreCapability
		}
	} else if runtimeContainsUnrepresented(raw, before) {
		return nil, ErrRestoreCapability
	}
	return runtime.DeepCopyJSONValue(after), nil
}

// UpdateObservedRuntimeVM refuses a typed-only write without the matching raw
// UID/resourceVersion. The caller retains its existing admission/fence checks.
func UpdateObservedRuntimeVM(ctx context.Context, client dynamic.Interface, before, after *kv.VirtualMachine, beforeWrite ...func() error) (*kv.VirtualMachine, error) {
	if client == nil || before == nil || after == nil {
		return nil, ErrReferenceUnavailable
	}
	raw, err := client.Resource(restoreVMGVR).Namespace(before.Namespace).Get(ctx, before.Name, metav1.GetOptions{})
	if err != nil {
		return nil, ErrReferenceUnavailable
	}
	object, err := PreserveRuntimeVM(raw, before, after)
	if err != nil {
		return nil, err
	}
	for _, guard := range beforeWrite {
		if guard == nil {
			return nil, ErrCorrupt
		}
		if err := guard(); err != nil {
			return nil, err
		}
	}
	updated, err := client.Resource(restoreVMGVR).Namespace(before.Namespace).Update(ctx, object, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	var result kv.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(updated.Object, &result) != nil {
		return nil, ErrCorrupt
	}
	return &result, nil
}
