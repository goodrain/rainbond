package cnb

import (
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	corev1 "k8s.io/api/core/v1"
)

// buildPlatformAnnotations creates annotations for platform env values.
// Language-specific annotations are delegated to LanguageConfig.
// Generic BP_* passthrough is handled here.
func (b *Builder) buildPlatformAnnotations(re *build.Request) map[string]string {
	annotations := make(map[string]string)
	addDebugAnnotations(re, annotations)

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

func addDebugAnnotations(re *build.Request, annotations map[string]string) {
	if lang := cnbDebugLanguage(re); lang != "" {
		annotations["rainbond.io/cnb-language"] = lang
	}
	if procfile := strings.TrimSpace(firstNonEmptyEnv(re.BuildEnvs, "BUILD_PROCFILE", "BUILD_AUTO_PROCFILE")); procfile != "" {
		source := strings.TrimSpace(re.BuildEnvs["START_COMMAND_SOURCE"])
		if source == "" {
			source = "procfile"
		}
		annotations["rainbond.io/cnb-start-command-source"] = source
		annotations["rainbond.io/cnb-start-command-hint"] = procfile
		return
	}
	if startScript := strings.TrimSpace(re.BuildEnvs["CNB_START_SCRIPT"]); startScript != "" {
		annotations["rainbond.io/cnb-start-command-source"] = "script"
		annotations["rainbond.io/cnb-start-command-hint"] = startScript
	}
}

func cnbDebugLanguage(re *build.Request) string {
	switch re.Lang {
	case code.JavaMaven, code.JaveWar, code.JavaJar, code.Gradle:
		return "java"
	case code.Python:
		return "python"
	case code.Golang:
		return "golang"
	case code.PHP:
		return "php"
	case code.NetCore:
		return "dotnet"
	case code.Static:
		return "static"
	default:
		if strings.Contains(strings.ToLower(string(re.Lang)), "node") {
			return "nodejs"
		}
		return ""
	}
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

// createPlatformVolume creates the projected platform volume for env and bindings.
func (b *Builder) createPlatformVolume(annotations map[string]string, bindings []platformBinding) (*corev1.Volume, *corev1.VolumeMount) {
	if len(annotations) == 0 && len(bindings) == 0 {
		return nil, nil
	}

	var downwardItems []corev1.DownwardAPIVolumeFile
	for key := range annotations {
		if !strings.HasPrefix(key, "cnb-") {
			continue
		}
		if strings.HasPrefix(key, "cnb-binding-") {
			continue
		}
		envName := annotationKeyToBPEnv(key)
		downwardItems = append(downwardItems, corev1.DownwardAPIVolumeFile{
			Path: "env/" + envName,
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.annotations['" + key + "']",
			},
		})
	}
	for _, binding := range bindings {
		key := bindingTypeAnnotationKey(binding.Name)
		downwardItems = append(downwardItems, corev1.DownwardAPIVolumeFile{
			Path: "bindings/" + binding.Name + "/type",
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.annotations['" + key + "']",
			},
		})
	}

	var projections []corev1.VolumeProjection
	if len(downwardItems) > 0 {
		projections = append(projections, corev1.VolumeProjection{
			DownwardAPI: &corev1.DownwardAPIProjection{
				Items: downwardItems,
			},
		})
	}
	for _, binding := range bindings {
		projections = append(projections, corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: binding.ConfigMapName},
				Items: []corev1.KeyToPath{{
					Key:  binding.ConfigMapKey,
					Path: "bindings/" + binding.Name + "/" + binding.TargetFile,
				}},
			},
		})
	}

	if len(projections) == 0 {
		return nil, nil
	}

	volume := &corev1.Volume{
		Name: "platform",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: projections,
			},
		},
	}

	mount := &corev1.VolumeMount{
		Name:      "platform",
		MountPath: "/platform",
	}

	return volume, mount
}
