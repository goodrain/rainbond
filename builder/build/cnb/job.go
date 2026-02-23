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

	cnbBuilderImage := GetCNBBuilderImage()
	cnbRunImage := GetCNBRunImage()

	// Compute platform annotations once
	annotations := b.buildPlatformAnnotations(re)

	job := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels: map[string]string{
				"service":    re.ServiceID,
				"job":        "codebuild",
				"build-type": "cnb",
			},
			Annotations: annotations,
		},
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

	volumes, mounts := b.createVolumeAndMount(re, secret.Name)

	platformVolume, platformMount := b.createPlatformVolume(annotations)
	if platformVolume != nil {
		volumes = append(volumes, *platformVolume)
		mounts = append(mounts, *platformMount)
	}

	podSpec.Volumes = volumes

	creatorArgs := b.buildCreatorArgs(re, buildImageName, cnbRunImage)

	// Chown workspace to cnb user inside the builder image (where cnb user exists with correct UID),
	// then exec the lifecycle creator. This ensures buildpacks can chmod generated files.
	container := corev1.Container{
		Name:         name,
		Image:        cnbBuilderImage,
		Stdin:        true,
		StdinOnce:    true,
		Command:      []string{"sh", "-c", `chown -R cnb:cnb /workspace && exec "$@"`, "--"},
		Args:         append([]string{CNBLifecycleCreatorPath}, creatorArgs...),
		Env:          b.buildEnvVars(re),
		VolumeMounts: mounts,
	}

	podSpec.Containers = append(podSpec.Containers, container)

	for _, ha := range re.HostAlias {
		podSpec.HostAliases = append(podSpec.HostAliases, corev1.HostAlias{IP: ha.IP, Hostnames: ha.Hostnames})
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

	args := []string{
		"-app=/workspace",
		"-layers=/layers",
		"-platform=/platform",
		"-run-image=" + runImage,
		"-cache-image=" + stableImageTag(buildImageName, "cnb-cache"),
		"-previous-image=" + latestImage,
		"-tag=" + latestImage,
		"-insecure-registry=" + registryHost,
		"-parallel",
		"-log-level=" + logLevel,
	}

	// Skip cache restore when NO_CACHE is set
	if re.BuildEnvs["NO_CACHE"] == "True" || re.BuildEnvs["NO_CACHE"] == "true" {
		args = append(args, "-skip-restore")
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

	return envs
}

// createVolumeAndMount creates volumes and volume mounts for the CNB build pod
func (b *Builder) createVolumeAndMount(re *build.Request, secretName string) ([]corev1.Volume, []corev1.VolumeMount) {
	hostPathType := corev1.HostPathDirectoryOrCreate
	hostPathDirectoryType := corev1.HostPathDirectory
	hostsFilePathType := corev1.HostPathFile

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
		{
			Name: "hosts",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/hosts",
					Type: &hostsFilePathType,
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "workspace", MountPath: "/workspace"},
		{Name: "layers", MountPath: "/layers"},
		{Name: "grdata", MountPath: "/grdata"},
		{Name: "docker-config", MountPath: "/home/cnb/.docker"},
		{Name: "hosts", MountPath: "/etc/hosts"},
	}

	return volumes, volumeMounts
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