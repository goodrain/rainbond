package sources

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

// capability_id: rainbond.builder.dockerfile-job-name
func TestDockerfileBuildJobNameFitsKubernetesContainerName(t *testing.T) {
	service := "497f2ec40d4e12ff43a29f243259cda2"
	capture := "delivery-8820775ce2fe477aa5fec9928c375623"
	for _, test := range []struct {
		name, service, version string
	}{
		{"normal build", service, "20261117120000"},
		{"63 character boundary", service, strings.Repeat("a", 19)},
		{"64 character name", service, strings.Repeat("a", 20)},
		{"live VM delivery capture", service, capture},
		{"long component identity", strings.Repeat("b", 64), capture},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := dockerfileBuildJobName(test.service, test.version)
			if problems := validation.IsDNS1123Label(name); len(problems) != 0 {
				t.Fatalf("build container rejected by Kubernetes: %q: %v", name, problems)
			}
			if name != dockerfileBuildJobName(test.service, test.version) {
				t.Fatal("a retried build changed its job identity")
			}
			raw := test.service + "-" + test.version + "-dockerfile"
			if len(raw) <= 63 && name != raw {
				t.Fatalf("existing valid build name changed: %q != %q", name, raw)
			}
		})
	}
}

// capability_id: rainbond.builder.dockerfile-job-name
func TestDockerfileBuildJobNameRetainsFullCaptureIdentity(t *testing.T) {
	service := strings.Repeat("a", 32)
	version := "delivery-" + strings.Repeat("b", 32)
	name := dockerfileBuildJobName(service, version)
	for _, changed := range []string{
		dockerfileBuildJobName(service, version[:len(version)-1]+"c"),
		dockerfileBuildJobName(service[:len(service)-1]+"c", version),
	} {
		if name == changed {
			t.Fatal("different source or capture was reduced to the same job identity")
		}
	}
}
