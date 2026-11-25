package vmrestore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-restore.native-pci-default-metadata
func TestManagedNativeRestorePreservesControllerPCITopologyDefault(t *testing.T) {
	const annotation = "kubevirt.io/pci-topology-version"
	for _, mode := range []string{"renderer omits controller default", "renderer preserves controller value", "desired user annotation changes", "live user annotation changes", "desired topology changes", "live topology changes"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			f := newManagedNativeFixture(t)
			source := f.snapshots.fixture.content.Spec.Source.VirtualMachine
			source.Spec.Template.ObjectMeta.Annotations = map[string]string{annotation: "v3", "user.example/stable": "keep"}
			live, _, err := f.backend.VirtualMachine(ctx, f.scope)
			if err != nil {
				t.Fatal(err)
			}
			live.Spec.Template.ObjectMeta.Annotations = map[string]string{annotation: "v3", "user.example/stable": "keep"}
			desired := nativeRestoreSourceVM(source)
			delete(desired.Spec.Template.ObjectMeta.Annotations, annotation)
			switch mode {
			case "renderer preserves controller value":
				desired.Spec.Template.ObjectMeta.Annotations[annotation] = "v3"
			case "desired user annotation changes":
				desired.Spec.Template.ObjectMeta.Annotations["user.example/stable"] = "changed"
			case "live user annotation changes":
				live.Spec.Template.ObjectMeta.Annotations["user.example/stable"] = "changed"
			case "desired topology changes":
				desired.Spec.Template.ObjectMeta.Annotations[annotation] = "v4"
			case "live topology changes":
				live.Spec.Template.ObjectMeta.Annotations[annotation] = "v4"
			}
			object, err := EncodeRuntimeVM(live)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, object, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			sourceBefore := source.DeepCopy()
			desiredBefore := desired.DeepCopy()
			f.managed.BuildPlatform = func(context.Context, Scope) (*kv.VirtualMachine, error) { return desired, nil }
			_, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-pci-default")
			allowed := mode == "renderer omits controller default" || mode == "renderer preserves controller value"
			if allowed {
				if err != nil || f.backend.creates != 1 {
					t.Fatalf("unchanged controller-defaulted VM could not restore: %v (created %d)", err, f.backend.creates)
				}
			} else if !errors.Is(err, ErrNativeRestoreConfigurationChanged) || f.backend.creates != 0 {
				t.Fatalf("real metadata change reached restore: %v (created %d)", err, f.backend.creates)
			}
			if !reflect.DeepEqual(source, sourceBefore) || !reflect.DeepEqual(desired, desiredBefore) {
				t.Fatal("comparison mutated source or platform-rendered metadata")
			}
		})
	}
}
