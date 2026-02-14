package cnb

import (
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

// buildPlatformAnnotations creates annotations for platform env values.
// Language-specific annotations are delegated to LanguageConfig.
// Generic BP_* passthrough is handled here.
func (b *Builder) buildPlatformAnnotations(re *build.Request) map[string]string {
	annotations := make(map[string]string)

	// Language-specific annotations
	lang := getLanguageConfig(re)
	lang.BuildAnnotations(re, annotations)

	// Pass through any additional BP_* variables from BuildEnvs
	for key, value := range re.BuildEnvs {
		if strings.HasPrefix(key, "BP_") && value != "" {
			annotationKey := bpEnvToAnnotationKey(key)
			if _, exists := annotations[annotationKey]; !exists {
				annotations[annotationKey] = value
			}
		}
	}

	return annotations
}

// bpEnvToAnnotationKey converts a BP_* env var name to a cnb annotation key.
// e.g. BP_NODE_VERSION -> cnb-bp-node-version
func bpEnvToAnnotationKey(envName string) string {
	lower := strings.ToLower(envName)
	dashed := strings.ReplaceAll(lower, "_", "-")
	return "cnb-" + dashed
}

// annotationKeyToBPEnv converts a cnb annotation key back to an env var name.
// e.g. cnb-bp-node-version -> BP_NODE_VERSION, cnb-node-env -> NODE_ENV
func annotationKeyToBPEnv(annotationKey string) string {
	withoutPrefix := strings.TrimPrefix(annotationKey, "cnb-")
	upper := strings.ToUpper(withoutPrefix)
	return strings.ReplaceAll(upper, "-", "_")
}

// createPlatformVolume creates a DownwardAPI volume for platform/env.
func (b *Builder) createPlatformVolume(annotations map[string]string) (*corev1.Volume, *corev1.VolumeMount) {
	if len(annotations) == 0 {
		return nil, nil
	}

	var items []corev1.DownwardAPIVolumeFile
	for key := range annotations {
		if !strings.HasPrefix(key, "cnb-") {
			continue
		}
		envName := annotationKeyToBPEnv(key)
		items = append(items, corev1.DownwardAPIVolumeFile{
			Path: "env/" + envName,
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.annotations['" + key + "']",
			},
		})
	}

	if len(items) == 0 {
		return nil, nil
	}

	volume := &corev1.Volume{
		Name: "platform",
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: items,
			},
		},
	}

	mount := &corev1.VolumeMount{
		Name:      "platform",
		MountPath: "/platform",
	}

	return volume, mount
}
