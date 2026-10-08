package cleanup

import (
	"io"
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/db/model"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

var customImageTemplate = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}`)

// Inspect only the API-defined pod template fields. Unknown custom resources
// cannot provide a negative-reference proof. Raw documents are never returned.
func inspectSavedWorkload(content string, image func(string)) bool {
	if len(content) == 0 || len(content) > 1<<20 {
		return false
	}
	decoder := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(content), 1024)
	for documents := 0; documents < 1000; documents++ {
		var object map[string]interface{}
		err := decoder.Decode(&object)
		if err == io.EOF {
			return documents > 0
		}
		if err != nil || len(object) == 0 || !inspectWorkloadObject(object, image, 0) {
			return false
		}
	}
	return false
}
func inspectWorkloadObject(object map[string]interface{}, image func(string), depth int) bool {
	if depth > 8 {
		return false
	}
	kind, _ := object["kind"].(string)
	version, _ := object["apiVersion"].(string)
	group := ""
	if slash := strings.IndexByte(version, '/'); slash >= 0 {
		group = version[:slash]
	}
	if strings.HasSuffix(group, ".kubeblocks.io") {
		visited := 0
		return inspectCustomImageReferences(object, image, 0, &visited)
	}
	if version == "aquasecurity.github.io/v1alpha1" && kind == "ClusterComplianceReport" {
		return true
	}
	validVersion := map[string]bool{"v1": true, "apps/v1": true, "batch/v1": true, "networking.k8s.io/v1": true, "rbac.authorization.k8s.io/v1": true, "policy/v1": true, "autoscaling/v1": true, "autoscaling/v2": true, "apiextensions.k8s.io/v1": true, "rainbond.io/v1alpha1": true}
	if !validVersion[version] {
		return false
	}

	expectedGroup := map[string]string{"Deployment": "apps", "StatefulSet": "apps", "DaemonSet": "apps", "ReplicaSet": "apps", "Job": "batch", "CronJob": "batch", "Ingress": "networking.k8s.io", "NetworkPolicy": "networking.k8s.io", "Role": "rbac.authorization.k8s.io", "RoleBinding": "rbac.authorization.k8s.io", "ClusterRole": "rbac.authorization.k8s.io", "ClusterRoleBinding": "rbac.authorization.k8s.io", "PodDisruptionBudget": "policy", "HorizontalPodAutoscaler": "autoscaling", "CustomResourceDefinition": "apiextensions.k8s.io", "RBDPlugin": "rainbond.io", "ServiceMesh": "rainbond.io"}
	if group != expectedGroup[kind] {
		return false
	}
	if kind == "List" {
		items, found, err := unstructured.NestedSlice(object, "items")
		if err != nil || !found || len(items) > 1000 {
			return false
		}
		for _, item := range items {
			child, ok := item.(map[string]interface{})
			if !ok || !inspectWorkloadObject(child, image, depth+1) {
				return false
			}
		}
		return true
	}
	var fields []string
	switch kind {
	case "ServiceMesh":
		// Native governance uses the platform mesh/probe images collected by
		// ReadPlatformHelperReferences, not an image field on this CR. Custom
		// providers can inject other images and remain incomplete evidence.
		provider, ok := object["provisioner"].(string)
		if !ok || (provider != model.GovernanceModeBuildInServiceMesh && provider != model.GovernanceModeKubernetesNativeService) {
			return false
		}
		for key := range object {
			switch key {
			case "apiVersion", "kind", "metadata", "provisioner", "selector", "status":
			default:
				return false
			}
		}
		selector, found, err := unstructured.NestedStringMap(object, "selector")
		return err == nil && found && len(selector) > 0
	case "Pod":
		fields = []string{"spec"}
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "ReplicationController", "Job":
		fields = []string{"spec", "template", "spec"}
	case "CronJob":
		fields = []string{"spec", "jobTemplate", "spec", "template", "spec"}
	case "Namespace", "Service", "Endpoints", "Secret", "ConfigMap", "Ingress", "PersistentVolumeClaim", "PersistentVolume", "ServiceAccount", "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding", "NetworkPolicy", "ResourceQuota", "LimitRange", "PodDisruptionBudget", "HorizontalPodAutoscaler", "CustomResourceDefinition", "RBDPlugin":
		return true
	default:
		return false
	}
	spec, found, err := unstructured.NestedMap(object, fields...)
	if err != nil || !found {
		return false
	}
	for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
		containers, present, err := unstructured.NestedSlice(spec, field)
		if err != nil || len(containers) > 1000 || (field == "containers" && (!present || len(containers) == 0)) {
			return false
		}
		for _, container := range containers {
			values, ok := container.(map[string]interface{})
			if !ok {
				return false
			}
			name, ok := values["image"].(string)
			if !ok || name == "" {
				return false
			}
			image(name)
		}
	}
	// Unsupported image-backed volume definitions must not be overlooked.
	volumes, _, err := unstructured.NestedSlice(spec, "volumes")
	if err != nil {
		return false
	}
	for _, volume := range volumes {
		if value, ok := volume.(map[string]interface{}); !ok {
			return false
		} else if _, found := value["image"]; found {
			return false
		}
	}
	return true
}

func inspectCustomImageReferences(value interface{}, image func(string), depth int, visited *int) bool {
	if depth > 16 || visited == nil {
		return false
	}
	(*visited)++
	if *visited > 20000 {
		return false
	}
	switch current := value.(type) {
	case map[string]interface{}:
		for key, child := range current {
			switch key {
			case "image":
				name, ok := child.(string)
				if !ok || name == "" {
					return false
				}
				if !customImageTemplate.MatchString(name) {
					image(name)
				}
			case "images":
				if !inspectImageReferenceValues(child, image, depth+1, visited) {
					return false
				}
			default:
				if !inspectCustomImageReferences(child, image, depth+1, visited) {
					return false
				}
			}
		}
	case []interface{}:
		for _, child := range current {
			if !inspectCustomImageReferences(child, image, depth+1, visited) {
				return false
			}
		}
	}
	return true
}

func inspectImageReferenceValues(value interface{}, image func(string), depth int, visited *int) bool {
	if depth > 16 || visited == nil {
		return false
	}
	(*visited)++
	if *visited > 20000 {
		return false
	}
	switch current := value.(type) {
	case string:
		if current == "" {
			return false
		}
		if !customImageTemplate.MatchString(current) {
			image(current)
		}
		return true
	case map[string]interface{}:
		for _, child := range current {
			if !inspectImageReferenceValues(child, image, depth+1, visited) {
				return false
			}
		}
		return true
	case []interface{}:
		for _, child := range current {
			if !inspectImageReferenceValues(child, image, depth+1, visited) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
