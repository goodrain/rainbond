package build

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestNewSlugBuildPodSpecToleratesDedicatedNode(t *testing.T) {
	for _, cacheMode := range []string{"", "sharefile", "hostpath"} {
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(cacheMode+"/"+arch, func(t *testing.T) {
				spec := newSlugBuildPodSpec(arch, "chaos-node", cacheMode)
				for _, taint := range []corev1.Taint{
					{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoSchedule},
					{Key: "dedicated", Value: "rbd-chaos", Effect: corev1.TaintEffectNoSchedule},
					{Key: "dedicated", Value: "rbd-chaos", Effect: corev1.TaintEffectNoExecute},
				} {
					tolerated := false
					for _, toleration := range spec.Tolerations {
						if toleration.ToleratesTaint(&taint) {
							tolerated = true
							break
						}
					}
					if !tolerated {
						t.Errorf("build pod does not tolerate dedicated node taint %v", taint)
					}
				}

				if spec.Affinity == nil || spec.Affinity.NodeAffinity == nil {
					t.Fatal("build pod must remain restricted to the current chaos node and architecture")
				}
				want := &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "kubernetes.io/arch", Operator: corev1.NodeSelectorOpIn, Values: []string{arch}},
							{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"chaos-node"}},
						},
					}},
				}
				if got := spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution; !reflect.DeepEqual(got, want) {
					t.Errorf("required node affinity = %#v, want %#v", got, want)
				}
				if cacheMode == "hostpath" && spec.NodeSelector["kubernetes.io/hostname"] != "chaos-node" {
					t.Errorf("hostpath node selector = %v, want chaos-node", spec.NodeSelector)
				}
				if spec.RestartPolicy != corev1.RestartPolicyOnFailure {
					t.Errorf("restart policy = %q, want OnFailure", spec.RestartPolicy)
				}
			})
		}
	}
}

func TestNewSlugBuildPodSpecWithoutHostDoesNotTolerateTaints(t *testing.T) {
	for _, cacheMode := range []string{"", "sharefile", "hostpath"} {
		t.Run(cacheMode, func(t *testing.T) {
			spec := newSlugBuildPodSpec("amd64", "", cacheMode)
			if len(spec.Tolerations) != 0 {
				t.Errorf("expected no taint tolerations without a target host, got %v", spec.Tolerations)
			}
		})
	}
}
