package cnb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// runCNBBuildJob creates and runs the CNB build pod
func (b *Builder) runCNBBuildJob(re *build.Request, buildImageName string) error {
	name := fmt.Sprintf("%s-%s", re.ServiceID, re.DeployVersion)
	namespace := re.RbdNamespace

	cnbBuilderImage := GetCNBBuilderImageForLanguage(re.Lang)
	cnbRunImage := GetCNBRunImageForLanguage(re.Lang)

	bindings, err := b.resolvePlatformBindings(re)
	if err != nil {
		return err
	}

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
	}

	// Set node affinity for architecture and hostname
	nodeSelectors := []corev1.NodeSelectorRequirement{
		{
			Key:      "kubernetes.io/arch",
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{re.Arch},
		},
	}
	if hostIP := os.Getenv("HOST_IP"); hostIP != "" {
		nodeSelectors = append(nodeSelectors, corev1.NodeSelectorRequirement{
			Key:      "kubernetes.io/hostname",
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{hostIP},
		})
	}
	podSpec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: nodeSelectors,
				}},
			},
		},
	}

	podSpec.Tolerations = []corev1.Toleration{{Operator: "Exists"}}

	rootUser := int64(0)
	podSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsUser:  &rootUser,
		RunAsGroup: &rootUser,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := b.createAuthSecret(re)
	if err != nil {
		return err
	}
	defer b.deleteAuthSecret(re, secret.Name)

	imageDomain, buildKitTomlCMName := sources.GetImageFirstPart(builder.REGISTRYDOMAIN)
	err = b.prepareBuildKit(ctx, re.KubeClient, re.RbdNamespace, buildKitTomlCMName, imageDomain)
	if err != nil {
		return err
	}

	procfileBinding, cleanupProcfileBinding, err := b.createProcfileBinding(ctx, re, name)
	if err != nil {
		return err
	}
	if cleanupProcfileBinding != nil {
		defer cleanupProcfileBinding()
	}
	if procfileBinding != nil {
		bindings = append(bindings, *procfileBinding)
	}

	// Compute platform annotations after bindings so derived BP_* values are included.
	annotations := b.buildPlatformAnnotations(re)
	for _, binding := range bindings {
		annotations[bindingTypeAnnotationKey(binding.Name)] = binding.Type
	}

	job := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"service":    re.ServiceID,
				"job":        "codebuild",
				"build-type": "cnb",
			},
			Annotations: annotations,
		},
	}

	volumes, mounts := b.createVolumeAndMount(re, secret.Name)

	platformVolume, platformMount := b.createPlatformVolume(annotations, bindings)
	if platformVolume != nil {
		volumes = append(volumes, *platformVolume)
		mounts = append(mounts, *platformMount)
	}

	podSpec.Volumes = volumes

	creatorArgs := b.buildCreatorArgs(re, buildImageName, cnbRunImage)

	// Chown workspace to cnb user inside the builder image (where cnb user exists with correct UID),
	// then exec the lifecycle creator. This ensures buildpacks can chmod generated files.
	container := corev1.Container{
		Name:            name,
		Image:           cnbBuilderImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Stdin:           true,
		StdinOnce:       true,
		Command:         []string{"sh", "-c", `chown -R cnb:cnb /workspace && exec "$@"`, "--"},
		Args:            append([]string{CNBLifecycleCreatorPath}, creatorArgs...),
		Env:             b.buildEnvVars(re),
		VolumeMounts:    mounts,
	}

	podSpec.Containers = append(podSpec.Containers, container)

	// Merge hostAliases with the same IP into a single entry to avoid K8s warnings.
	mergedAliases := make(map[string][]string)
	for _, ha := range re.HostAlias {
		mergedAliases[ha.IP] = append(mergedAliases[ha.IP], ha.Hostnames...)
	}
	for ip, hostnames := range mergedAliases {
		podSpec.HostAliases = append(podSpec.HostAliases, corev1.HostAlias{IP: ip, Hostnames: hostnames})
	}

	job.Spec = podSpec

	writer := re.Logger.GetWriter("builder", "info")
	reChan := channels.NewRingChannel(10)

	logrus.Debugf("create CNB job[name: %s; namespace: %s]", job.Name, job.Namespace)
	err = b.jobCtrl.ExecJob(ctx, &job, writer, reChan)
	if err != nil {
		logrus.Errorf("create CNB job %s failed: %s", name, err.Error())
		re.Logger.Error(util.Translation("Create CNB build job failed"), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}

	re.Logger.Info(util.Translation("create CNB build job success"), map[string]string{"step": "build-exector"})

	defer b.jobCtrl.DeleteJob(job.Name)

	return b.waitingComplete(re, reChan)
}

// buildCreatorArgs builds the lifecycle creator arguments
func (b *Builder) buildCreatorArgs(re *build.Request, buildImageName, runImage string) []string {
	registryHost, _ := sources.GetImageFirstPart(builder.REGISTRYDOMAIN)
	latestImage := stableImageTag(buildImageName, "latest")

	logLevel := "info"
	if v := re.BuildEnvs["CNB_LOG_LEVEL"]; v != "" {
		logLevel = v
	}

	noCache := truthyBuildEnv(re.BuildEnvs["NO_CACHE"]) || truthyBuildEnv(re.BuildEnvs["BUILD_NO_CACHE"])

	args := []string{
		"-app=/workspace",
		"-layers=/layers",
		"-platform=/platform",
		"-run-image=" + runImage,
		"-tag=" + latestImage,
		"-insecure-registry=" + registryHost,
		"-log-level=" + logLevel,
	}

	if noCache {
		// Skip both cache restore and image layer reuse
		args = append(args, "-skip-restore")
	} else {
		// Enable registry cache, previous image layer reuse, and parallel export
		args = append(args, "-cache-image="+stableImageTag(buildImageName, "cnb-cache"))
		args = append(args, "-previous-image="+latestImage)
		args = append(args, "-parallel")
	}

	// Custom order from language config (e.g., pure static projects need nginx-only order)
	lang := getLanguageConfig(re)
	if bps := lang.CustomOrder(re); len(bps) > 0 {
		if flag := b.writeCustomOrder(re, bps, "custom language order"); flag != "" {
			args = append(args, flag)
		}
	}

	args = append(args, buildImageName)
	return args
}

// buildEnvVars builds environment variables for the CNB build container.
func (b *Builder) buildEnvVars(re *build.Request) []corev1.EnvVar {
	registryHost, _ := sources.GetImageFirstPart(builder.REGISTRYDOMAIN)
	envs := []corev1.EnvVar{
		{Name: "CNB_PLATFORM_API", Value: "0.13"},
		{Name: "CNB_INSECURE_REGISTRIES", Value: registryHost},
		{Name: "DOCKER_CONFIG", Value: "/home/cnb/.docker"},
	}

	if httpProxy := os.Getenv("HTTP_PROXY"); httpProxy != "" {
		envs = append(envs, corev1.EnvVar{Name: "HTTP_PROXY", Value: httpProxy})
	}
	if httpsProxy := os.Getenv("HTTPS_PROXY"); httpsProxy != "" {
		envs = append(envs, corev1.EnvVar{Name: "HTTPS_PROXY", Value: httpsProxy})
	}
	if noProxy := os.Getenv("NO_PROXY"); noProxy != "" {
		envs = append(envs, corev1.EnvVar{Name: "NO_PROXY", Value: noProxy})
	}

	lang := getLanguageConfig(re)
	envs = append(envs, lang.BuildEnvVars(re)...)

	return envs
}

func (b *Builder) resolvePlatformBindings(re *build.Request) ([]platformBinding, error) {
	ctx := re.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	switch re.Lang {
	case code.JavaMaven:
		explicitName := strings.TrimSpace(firstNonEmptyEnv(re.BuildEnvs, "BUILD_MAVEN_SETTING_NAME", "MAVEN_SETTING_NAME"))
		configMapName := ""
		if explicitName != "" {
			configMapName = b.jobCtrl.GetLanguageBuildSetting(ctx, code.JavaMaven, explicitName)
			if configMapName == "" {
				return nil, fmt.Errorf("maven setting config %s not found", explicitName)
			}
		} else {
			configMapName = b.jobCtrl.GetDefaultLanguageBuildSetting(ctx, code.JavaMaven)
		}
		if configMapName == "" {
			return nil, nil
		}
		re.BuildEnvs["BP_MAVEN_SETTINGS_PATH"] = fmt.Sprintf("/platform/bindings/%s/settings.xml", configMapName)

		return []platformBinding{{
			Name:          configMapName,
			Type:          "maven",
			ConfigMapName: configMapName,
			ConfigMapKey:  "mavensetting",
			TargetFile:    "settings.xml",
		}}, nil
	case code.NetCore:
		configMapName := strings.TrimSpace(firstNonEmptyEnv(re.BuildEnvs, "BUILD_NUGET_CONFIG_NAME"))
		if configMapName == "" {
			return nil, nil
		}
		if resolved := b.jobCtrl.GetLanguageBuildSetting(ctx, code.NetCore, configMapName); resolved == "" {
			return nil, fmt.Errorf("nuget config %s not found", configMapName)
		}
		return []platformBinding{{
			Name:          configMapName,
			Type:          "nugetconfig",
			ConfigMapName: configMapName,
			ConfigMapKey:  "nuget.config",
			TargetFile:    "nuget.config",
		}}, nil
	default:
		return nil, nil
	}
}

func (b *Builder) createProcfileBinding(ctx context.Context, re *build.Request, jobName string) (*platformBinding, func(), error) {
	procfile, _ := resolveProcfileBindingContent(re)
	if strings.TrimSpace(procfile) == "" {
		return nil, nil, nil
	}

	if b.createConfigMap == nil || b.deleteConfigMap == nil {
		return nil, nil, fmt.Errorf("procfile binding handlers are not configured")
	}
	if re.KubeClient == nil && b.createConfigMap == nil {
		return nil, nil, fmt.Errorf("kube client is required for procfile binding")
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-procfile-", strings.ToLower(jobName)),
			Namespace:    re.RbdNamespace,
			Labels: map[string]string{
				"service":    re.ServiceID,
				"job":        "codebuild",
				"build-type": "cnb",
			},
		},
		Data: map[string]string{
			"Procfile": normalizeProcfileBindingContent(procfile),
		},
	}

	createdConfigMap, err := b.createConfigMap(ctx, re.KubeClient, re.RbdNamespace, configMap)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		if err := b.deleteConfigMap(ctx, re.KubeClient, re.RbdNamespace, createdConfigMap.Name); err != nil {
			logrus.Warnf("delete procfile binding configmap %s failed: %v", createdConfigMap.Name, err)
		}
	}

	return &platformBinding{
		Name:          createdConfigMap.Name,
		Type:          "Procfile",
		ConfigMapName: createdConfigMap.Name,
		ConfigMapKey:  "Procfile",
		TargetFile:    "Procfile",
	}, cleanup, nil
}

// createVolumeAndMount creates volumes and volume mounts for the CNB build pod
func (b *Builder) createVolumeAndMount(re *build.Request, secretName string) ([]corev1.Volume, []corev1.VolumeMount) {
	hostPathType := corev1.HostPathDirectoryOrCreate
	hostPathDirectoryType := corev1.HostPathDirectory

	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: filepath.Join("/opt/rainbond/", re.SourceDir),
					Type: &hostPathDirectoryType,
				},
			},
		},
		{
			Name: "layers",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "grdata",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/rainbond/grdata",
					Type: &hostPathType,
				},
			},
		},
		{
			Name: "docker-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
					Items: []corev1.KeyToPath{
						{Key: ".dockerconfigjson", Path: "config.json"},
					},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "workspace", MountPath: "/workspace"},
		{Name: "layers", MountPath: "/layers"},
		{Name: "grdata", MountPath: "/grdata"},
		{Name: "docker-config", MountPath: "/home/cnb/.docker"},
	}

	return volumes, volumeMounts
}

func resolveProcfileBindingContent(re *build.Request) (string, string) {
	if procfile := re.BuildEnvs["BUILD_PROCFILE"]; strings.TrimSpace(procfile) != "" {
		source := strings.TrimSpace(re.BuildEnvs["START_COMMAND_SOURCE"])
		if source == "" {
			source = "procfile"
		}
		return procfile, source
	}
	if procfile := re.BuildEnvs["BUILD_AUTO_PROCFILE"]; strings.TrimSpace(procfile) != "" {
		source := strings.TrimSpace(re.BuildEnvs["START_COMMAND_SOURCE"])
		if source == "" {
			source = "auto-detected"
		}
		return procfile, source
	}
	if re.SourceDir == "" {
		return "", ""
	}
	body, err := os.ReadFile(filepath.Join(re.SourceDir, "Procfile"))
	if err != nil || strings.TrimSpace(string(body)) == "" {
		return "", ""
	}
	return string(body), "procfile"
}

func normalizeProcfileBindingContent(procfile string) string {
	procfile = strings.TrimRight(procfile, "\r\n")
	return procfile + "\n"
}

// waitingComplete waits for the build job to complete
func (b *Builder) waitingComplete(re *build.Request, reChan *channels.RingChannel) error {
	logComplete := false
	jobComplete := false
	var err error

	timeout := time.NewTimer(time.Minute * 60)
	defer timeout.Stop()
	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("cnb build timed out (exceeded 60 minutes)")
		case jobStatus := <-reChan.Out():
			status, ok := jobStatus.(string)
			if !ok {
				logrus.Warnf("unexpected job status type: %T", jobStatus)
				continue
			}
			switch status {
			case "complete":
				jobComplete = true
				if logComplete {
					return nil
				}
				re.Logger.Info(util.Translation("CNB build job completed"), map[string]string{"step": "build-exector"})
			case "failed":
				jobComplete = true
				err = fmt.Errorf("cnb build job exec failure")
				if logComplete {
					return err
				}
				re.Logger.Info(util.Translation("CNB build job failed"), map[string]string{"step": "build-exector"})
			case "cancel":
				jobComplete = true
				err = fmt.Errorf("cnb build job is canceled")
				if logComplete {
					return err
				}
			case "logcomplete":
				logComplete = true
				if jobComplete {
					return err
				}
			}
		}
	}
}

// stableImageTag replaces the tag portion of an image reference.
// e.g. "goodrain.me/workload:20260220215515" + "latest" → "goodrain.me/workload:latest"
func stableImageTag(imageName, tag string) string {
	if i := strings.LastIndex(imageName, ":"); i != -1 {
		return imageName[:i] + ":" + tag
	}
	return imageName + ":" + tag
}
