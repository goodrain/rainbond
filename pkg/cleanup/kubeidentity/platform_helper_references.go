package kubeidentity

import (
	"path"
	"strings"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/util/constants"
	corev1 "k8s.io/api/core/v1"
)

func platformHelperEnvironmentReferences(component string, env []corev1.EnvVar, from []corev1.EnvFromSource) guard.RegionReferenceInventory {
	if component == "rbd-api" {
		return helperEnvironmentReferences(env, from)
	}
	keys := map[string]bool{"BUILD_IMAGE_REPOSTORY_DOMAIN": true}
	switch component {
	case "rbd-worker":
		keys["TCPMESH_DEFAULT_IMAGE_NAME"] = true
		keys["PROBE_MESH_IMAGE_NAME"] = true
	case "rbd-chaos":
		for _, key := range []string{"RUNNER_IMAGE_NAME", "BUILDER_IMAGE_NAME", "CNB_BUILDER_IMAGE", "CNB_RUN_IMAGE", "VM_QCOW2_CONVERTER_IMAGE", "VM_HTTP_ARTIFACT_IMAGE"} {
			keys[key] = true
		}
	default:
		return guard.RegionReferenceInventory{}
	}
	complete := true
	values := map[string]string{}
	for _, source := range from {
		for key := range keys {
			if strings.HasPrefix(key, source.Prefix) {
				complete = false
			}
		}
	}
	for _, entry := range env {
		if !keys[entry.Name] {
			continue
		}
		if entry.ValueFrom != nil || strings.Contains(entry.Value, "$(") {
			complete = false
			continue
		}
		values[entry.Name] = entry.Value
	}
	get := func(key, fallback string) string {
		if values[key] != "" {
			return values[key]
		}
		return fallback
	}
	domain := get("BUILD_IMAGE_REPOSTORY_DOMAIN", constants.DefImageRepository)
	images := []string{}
	if component == "rbd-worker" {
		images = append(images, get("TCPMESH_DEFAULT_IMAGE_NAME", domain+"/rbd-mesh-data-panel"), get("PROBE_MESH_IMAGE_NAME", domain+"/rbd-init-probe"))
	} else {
		// Retain both supported offline architectures, even before a build starts.
		for _, arch := range []string{"amd64", "arm64"} {
			images = append(images, path.Join(domain, get("RUNNER_IMAGE_NAME", "runner:latest-"+arch)), path.Join(domain, get("BUILDER_IMAGE_NAME", "builder:latest-"+arch)))
		}
		for _, name := range []string{constants.CNBBuilderImageName, constants.CNBRunImageName, constants.PHPCNBBuilderImageName, constants.PHPCNBRunImageName} {
			images = append(images, path.Join(domain, name))
		}
		for _, key := range []string{"CNB_BUILDER_IMAGE", "CNB_RUN_IMAGE"} {
			if values[key] != "" {
				images = append(images, values[key])
			}
		}
		images = append(images, get("VM_QCOW2_CONVERTER_IMAGE", constants.VMQCOW2ConverterImage), get("VM_HTTP_ARTIFACT_IMAGE", constants.VMHTTPArtifactImage))
	}
	result := guard.ConfiguredImageReferences(images)
	result.Complete = result.Complete && complete
	return result
}
