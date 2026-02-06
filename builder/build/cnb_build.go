// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package build

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/builder"
	jobc "github.com/goodrain/rainbond/builder/job"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultCNBBuilder is the default CNB builder image
	DefaultCNBBuilder = "registry.cn-hangzhou.aliyuncs.com/goodrain/ubuntu-noble-builder:latest"
	// DefaultCNBRunImage is the default CNB run image
	DefaultCNBRunImage = "registry.cn-hangzhou.aliyuncs.com/goodrain/ubuntu-noble-run:0.0.50"
	// CNBLifecycleCreatorPath is the path to the lifecycle creator binary in builder image
	CNBLifecycleCreatorPath = "/lifecycle/creator"
)

// China mirror configuration constants
const (
	// DefaultNpmRegistry is the default npm registry for China
	DefaultNpmRegistry = "https://registry.npmmirror.com"
	// DefaultNodeDistURL is the default Node.js binary download URL for China
	DefaultNodeDistURL = "https://cdn.npmmirror.com/binaries/node"
	// DefaultDependencyMirror is the default dependency mirror for Paketo buildpacks
	DefaultDependencyMirror = "https://cdn.npmmirror.com/binaries"
)

// DefaultNpmrcContent is the default .npmrc content for China mirror
const DefaultNpmrcContent = `registry=https://registry.npmmirror.com
`

// DefaultYarnrcContent is the default .yarnrc content for China mirror (Yarn Classic)
const DefaultYarnrcContent = `registry "https://registry.npmmirror.com"
`

// DefaultPnpmrcContent is the default .pnpmrc content for China mirror
const DefaultPnpmrcContent = `registry=https://registry.npmmirror.com
`

// cnbBuilder creates a new CNB builder
func cnbBuilder() (Build, error) {
	return &cnbBuild{}, nil
}

// cnbBuild implements the Build interface for Cloud Native Buildpacks
type cnbBuild struct{}

// Build executes the CNB build process
func (c *cnbBuild) Build(re *Request) (*Response, error) {
	re.Logger.Info("Starting CNB build", map[string]string{"step": "builder-exector"})

	// Stop any previous build jobs for this service
	if err := c.stopPreBuildJob(re); err != nil {
		logrus.Errorf("stop pre build job for service %s failure %s", re.ServiceID, err.Error())
	}

	// Validate project files and warn user about common issues
	c.validateProjectFiles(re)

	// Inject .npmrc for China mirror support
	if err := c.injectNpmrc(re); err != nil {
		logrus.Warnf("inject .npmrc failed: %v", err)
	}

	// Prepare platform env directory with BP_* environment variables
	if err := c.preparePlatformEnv(re); err != nil {
		logrus.Warnf("prepare platform env failed: %v", err)
	}

	// Set source directory permissions for CNB user
	if err := c.setSourceDirPermissions(re); err != nil {
		logrus.Warnf("set source dir permissions failed: %v", err)
	}

	// Create the output image name
	buildImageName := CreateImageName(re.ServiceID, re.DeployVersion)

	// Run the CNB build job
	if err := c.runCNBBuildJob(re, buildImageName); err != nil {
		re.Logger.Error(util.Translation("CNB build failed"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("CNB build job error:", err.Error())
		return nil, err
	}

	re.Logger.Info("CNB build success", map[string]string{"step": "build-exector"})
	return &Response{
		MediumPath: buildImageName,
		MediumType: ImageMediumType,
	}, nil
}

// injectMirrorConfig injects .npmrc, .yarnrc, .pnpmrc files for npm/yarn/pnpm registry configuration
// This replaces the old injectNpmrc method to support all three package managers
func (c *cnbBuild) injectMirrorConfig(re *Request) error {
	// Determine mirror source: project (use existing files) or global (use platform config)
	mirrorSource := re.BuildEnvs["CNB_MIRROR_SOURCE"]

	// If mirror source is "project", check if project has config files
	if mirrorSource == "project" {
		// Check if any config file exists in the project
		hasConfig := false
		configFiles := []string{".npmrc", ".yarnrc", ".pnpmrc"}
		for _, file := range configFiles {
			filePath := re.SourceDir + "/" + file
			if _, err := os.Stat(filePath); err == nil {
				hasConfig = true
				logrus.Infof("Using project config file: %s", file)
			}
		}

		if hasConfig {
			logrus.Info("Using project mirror configuration files")
			return nil
		}

		// No project config found, fall through to use platform default
		logrus.Info("No project config files found, using platform default configuration")
	}

	// Use global/platform configuration
	// Priority: user-provided CNB_MIRROR_* > environment default > built-in default

	// Handle .npmrc
	if err := c.injectConfigFile(re, ".npmrc", "CNB_MIRROR_NPMRC"); err != nil {
		logrus.Warnf("inject .npmrc failed: %v", err)
	}

	// Handle .yarnrc
	if err := c.injectConfigFile(re, ".yarnrc", "CNB_MIRROR_YARNRC"); err != nil {
		logrus.Warnf("inject .yarnrc failed: %v", err)
	}

	// Handle .pnpmrc
	if err := c.injectConfigFile(re, ".pnpmrc", "CNB_MIRROR_PNPMRC"); err != nil {
		logrus.Warnf("inject .pnpmrc failed: %v", err)
	}

	return nil
}

// injectConfigFile injects a single config file (.npmrc, .yarnrc, or .pnpmrc)
func (c *cnbBuild) injectConfigFile(re *Request, fileName, envKey string) error {
	filePath := re.SourceDir + "/" + fileName

	// Check if file already exists in project
	if _, err := os.Stat(filePath); err == nil {
		// File exists, don't overwrite
		logrus.Infof("Config file %s already exists in project, skipping", fileName)
		return nil
	}

	// Check if user provided custom content via environment variable
	if customContent, ok := re.BuildEnvs[envKey]; ok && customContent != "" {
		logrus.Infof("Using user-provided %s configuration from %s", fileName, envKey)
		return os.WriteFile(filePath, []byte(customContent), 0644)
	}

	// Check if China mirror is enabled (default: true)
	enableChinaMirror := util.GetenvDefault("ENABLE_CHINA_MIRROR", "true")
	if enableChinaMirror != "true" {
		return nil
	}

	// Use default configuration based on file type
	var defaultContent string
	switch fileName {
	case ".npmrc":
		defaultContent = util.GetenvDefault("DEFAULT_NPMRC", DefaultNpmrcContent)
	case ".yarnrc":
		// Yarn classic (.yarnrc) format
		defaultContent = util.GetenvDefault("DEFAULT_YARNRC", DefaultYarnrcContent)
	case ".pnpmrc":
		// pnpm uses same format as npm
		defaultContent = util.GetenvDefault("DEFAULT_PNPMRC", DefaultPnpmrcContent)
	default:
		return nil
	}

	if defaultContent != "" {
		logrus.Infof("Creating default %s with China mirror configuration", fileName)
		return os.WriteFile(filePath, []byte(defaultContent), 0644)
	}

	return nil
}

// injectNpmrc injects .npmrc file for npm/yarn registry configuration
// Deprecated: Use injectMirrorConfig instead for full package manager support
func (c *cnbBuild) injectNpmrc(re *Request) error {
	// Delegate to the new method for backward compatibility
	return c.injectMirrorConfig(re)
}


// stopPreBuildJob stops any previous build jobs for the service
func (c *cnbBuild) stopPreBuildJob(re *Request) error {
	jobList, err := jobc.GetJobController().GetServiceJobs(re.ServiceID)
	if err != nil {
		logrus.Errorf("get pre build job for service %s failure: %s", re.ServiceID, err.Error())
	}
	if len(jobList) > 0 {
		for _, job := range jobList {
			jobc.GetJobController().DeleteJob(job.Name)
		}
	}
	return nil
}

// preparePlatformEnv is now a no-op as platform/env is handled by DownwardAPI volume
// Keeping for backward compatibility but the actual work is done by createPlatformVolume
func (c *cnbBuild) preparePlatformEnv(_ *Request) error {
	// Platform env files are now created via DownwardAPI volume mount
	// which exposes Pod annotations as files in /platform/env/
	return nil
}

// validateProjectFiles checks for common project configuration issues and warns user
func (c *cnbBuild) validateProjectFiles(re *Request) {
	// Check if lock files exist but are empty
	lockFiles := []string{
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
	}

	for _, lockFile := range lockFiles {
		lockfilePath := re.SourceDir + "/" + lockFile
		if info, err := os.Stat(lockfilePath); err == nil {
			if info.Size() == 0 {
				re.Logger.Error(fmt.Sprintf("ERROR: %s is empty. Please run the appropriate install command (npm install / yarn install / pnpm install) to generate a valid lock file, then commit and push.", lockFile), map[string]string{"step": "build-code"})
			}
		}
	}
}

// setSourceDirPermissions sets the source directory permissions for CNB user
func (c *cnbBuild) setSourceDirPermissions(re *Request) error {
	// Remove .git directory to avoid permission issues during CNB export phase
	// The .git directory contains pack files with restricted permissions that
	// lifecycle creator cannot read when exporting layers to the final image.
	// This is safe because re.SourceDir is a temporary copy of the source code.
	gitDir := re.SourceDir + "/.git"
	if _, err := os.Stat(gitDir); err == nil {
		if err := os.RemoveAll(gitDir); err != nil {
			logrus.Warnf("failed to remove .git directory: %v", err)
		}
	}

	// CNB user typically has uid=1000, gid=1000
	// Use chmod to make directory writable by all users as a simple solution
	return os.Chmod(re.SourceDir, 0777)
}

// runCNBBuildJob creates and runs the CNB build pod
func (c *cnbBuild) runCNBBuildJob(re *Request, buildImageName string) error {
	name := fmt.Sprintf("%s-%s", re.ServiceID, re.DeployVersion)
	namespace := re.RbdNamespace

	// Get CNB builder image from environment or use default
	// The builder image contains both buildpacks and lifecycle binaries
	cnbBuilderImage := util.GetenvDefault("CNB_BUILDER_IMAGE", DefaultCNBBuilder)
	cnbRunImage := util.GetenvDefault("CNB_RUN_IMAGE", DefaultCNBRunImage)

	// Create the pod definition
	job := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"service":    re.ServiceID,
				"job":        "codebuild",
				"build-type": "cnb",
			},
			Annotations: c.buildPlatformAnnotations(re),
		},
	}

	// Configure pod spec
	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyOnFailure,
	}

	// Set node affinity for architecture and hostname (ensure job runs on same node as chaos)
	podSpec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "kubernetes.io/arch",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{re.Arch},
						},
						{
							Key:      "kubernetes.io/hostname",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{os.Getenv("HOST_IP")},
						},
					},
				}},
			},
		},
	}

	// Set tolerations to allow scheduling on any node
	podSpec.Tolerations = []corev1.Toleration{
		{
			Operator: "Exists",
		},
	}

	// Set security context to run as root
	rootUser := int64(0)
	podSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsUser:  &rootUser,
		RunAsGroup: &rootUser,
	}

	// Create auth secret for registry
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := c.createAuthSecret(re)
	if err != nil {
		return err
	}

	// Prepare BuildKit config for registry
	imageDomain, buildKitTomlCMName := sources.GetImageFirstPart(builder.REGISTRYDOMAIN)
	err = sources.PrepareBuildKitTomlCM(ctx, re.KubeClient, re.RbdNamespace, buildKitTomlCMName, imageDomain)
	if err != nil {
		return err
	}

	// Create volumes and mounts
	volumes, mounts := c.createVolumeAndMount(re, secret.Name, buildKitTomlCMName)

	// Add platform volume using DownwardAPI to expose annotations as env files
	platformVolume, platformMount := c.createPlatformVolume(re)
	if platformVolume != nil {
		volumes = append(volumes, *platformVolume)
		mounts = append(mounts, *platformMount)
	}

	podSpec.Volumes = volumes

	// Build lifecycle creator arguments (instead of pack CLI)
	creatorArgs := c.buildCreatorArgs(re, buildImageName, cnbRunImage)

	// Create the container using builder image (which contains lifecycle binaries)
	container := corev1.Container{
		Name:         name,
		Image:        cnbBuilderImage, // Use builder image instead of pack CLI image
		Stdin:        true,
		StdinOnce:    true,
		Command:      []string{CNBLifecycleCreatorPath},
		Args:         creatorArgs,
		Env:          c.buildEnvVars(re),
		VolumeMounts: mounts,
	}

	podSpec.Containers = append(podSpec.Containers, container)

	// Add host aliases
	for _, ha := range re.HostAlias {
		podSpec.HostAliases = append(podSpec.HostAliases, corev1.HostAlias{IP: ha.IP, Hostnames: ha.Hostnames})
	}

	job.Spec = podSpec

	// Execute the job
	writer := re.Logger.GetWriter("builder", "info")
	reChan := channels.NewRingChannel(10)

	logrus.Debugf("create CNB job[name: %s; namespace: %s]", job.Name, job.Namespace)
	err = jobc.GetJobController().ExecJob(ctx, &job, writer, reChan)
	if err != nil {
		logrus.Errorf("create CNB job:%s failed: %s", name, err.Error())
		re.Logger.Error(util.Translation("Create CNB build job failed"), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}

	re.Logger.Info(util.Translation("create CNB build job success"), map[string]string{"step": "build-exector"})

	// Cleanup after completion
	defer c.deleteAuthSecret(re, secret.Name)
	defer jobc.GetJobController().DeleteJob(job.Name)

	return c.waitingComplete(re, reChan)
}

// buildCreatorArgs builds the lifecycle creator arguments
func (c *cnbBuild) buildCreatorArgs(_ *Request, buildImageName, runImage string) []string {
	args := []string{
		"-app=/workspace",
		"-layers=/layers",
		"-platform=/platform",
		"-run-image=" + runImage,
		"-cache-dir=/cache",
		"-log-level=info",
	}

	// Note: -cache-image is disabled because CNB_INSECURE_REGISTRIES does not work
	// for cache image access in lifecycle 0.20. Using local -cache-dir only.
	// TODO: Re-enable when lifecycle properly supports insecure registries for cache

	// Add the output image name (positional argument at the end)
	args = append(args, buildImageName)

	return args
}

// buildEnvVars builds environment variables for the container
func (c *cnbBuild) buildEnvVars(re *Request) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name:  "CNB_PLATFORM_API",
			Value: "0.12",
		},
		{
			// CNB_INSECURE_REGISTRIES for self-signed certificates
			Name:  "CNB_INSECURE_REGISTRIES",
			Value: builder.REGISTRYDOMAIN,
		},
		{
			// DOCKER_CONFIG points to the docker config directory
			Name:  "DOCKER_CONFIG",
			Value: "/home/cnb/.docker",
		},
	}

	// Add China mirror configuration for Paketo dependency downloads
	enableChinaMirror := util.GetenvDefault("ENABLE_CHINA_MIRROR", "true")
	if enableChinaMirror == "true" {
		// BP_DEPENDENCY_MIRROR configures Paketo buildpacks to use mirror for downloading dependencies
		dependencyMirror := util.GetenvDefault("BP_DEPENDENCY_MIRROR", DefaultDependencyMirror)
		if dependencyMirror != "" {
			envs = append(envs, corev1.EnvVar{Name: "BP_DEPENDENCY_MIRROR", Value: dependencyMirror})
		}
	}

	// Add Node.js version if specified (CNB_NODE_VERSION from frontend, or RUNTIMES for backward compatibility)
	if nodeVersion, ok := re.BuildEnvs["CNB_NODE_VERSION"]; ok && nodeVersion != "" {
		envs = append(envs, corev1.EnvVar{Name: "BP_NODE_VERSION", Value: nodeVersion})
	} else if nodeVersion, ok := re.BuildEnvs["RUNTIMES"]; ok && nodeVersion != "" {
		envs = append(envs, corev1.EnvVar{Name: "BP_NODE_VERSION", Value: nodeVersion})
	}

	// For static builds (indicated by CNB_OUTPUT_DIR), configure nginx web server
	// This sets BP_WEB_SERVER as container env var which nginx buildpack reads via os.Getenv()
	if outputDir, ok := re.BuildEnvs["CNB_OUTPUT_DIR"]; ok && outputDir != "" {
		envs = append(envs, corev1.EnvVar{Name: "BP_WEB_SERVER", Value: "nginx"})
		envs = append(envs, corev1.EnvVar{Name: "BP_WEB_SERVER_ROOT", Value: outputDir})
		// Enable SPA routing support for single-page applications
		envs = append(envs, corev1.EnvVar{Name: "BP_WEB_SERVER_ENABLE_PUSH_STATE", Value: "true"})
	}

	// Add custom build scripts if specified (CNB_BUILD_SCRIPT from frontend)
	if buildScript, ok := re.BuildEnvs["CNB_BUILD_SCRIPT"]; ok && buildScript != "" {
		envs = append(envs, corev1.EnvVar{Name: "BP_NODE_RUN_SCRIPTS", Value: buildScript})
	}

	// Add any additional environment variables prefixed with BP_
	for key, value := range re.BuildEnvs {
		if strings.HasPrefix(key, "BP_") {
			envs = append(envs, corev1.EnvVar{Name: key, Value: value})
		}
	}

	// Add proxy settings if configured
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
func (c *cnbBuild) createVolumeAndMount(re *Request, secretName, _ string) ([]corev1.Volume, []corev1.VolumeMount) {
	hostPathType := corev1.HostPathDirectoryOrCreate
	hostPathDirectoryType := corev1.HostPathDirectory
	hostsFilePathType := corev1.HostPathFile

	volumes := []corev1.Volume{
		{
			// Source code directory mounted as workspace
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: path.Join("/opt/rainbond/", re.SourceDir),
					Type: &hostPathDirectoryType,
				},
			},
		},
		{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: path.Join("/opt/rainbond/", re.CacheDir),
					Type: &hostPathType,
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
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
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
		{
			Name:      "workspace",
			MountPath: "/workspace",
		},
		{
			Name:      "cache",
			MountPath: "/cache",
		},
		{
			Name:      "layers",
			MountPath: "/layers",
		},
		{
			Name:      "grdata",
			MountPath: "/grdata",
		},
		{
			// CNB lifecycle uses /home/cnb/.docker for registry auth
			Name:      "docker-config",
			MountPath: "/home/cnb/.docker",
		},
		{
			Name:      "hosts",
			MountPath: "/etc/hosts",
		},
	}

	return volumes, volumeMounts
}

// buildPlatformAnnotations creates annotations for platform env values
// These annotations will be exposed as files via DownwardAPI
func (c *cnbBuild) buildPlatformAnnotations(re *Request) map[string]string {
	annotations := make(map[string]string)

	// For static builds (indicated by CNB_OUTPUT_DIR), set nginx web server
	if outputDir, ok := re.BuildEnvs["CNB_OUTPUT_DIR"]; ok && outputDir != "" {
		annotations["cnb-bp-web-server"] = "nginx"
		annotations["cnb-bp-web-server-root"] = outputDir
		annotations["cnb-bp-web-server-enable-push-state"] = "true"
	}

	// Add build script if specified
	if buildScript, ok := re.BuildEnvs["CNB_BUILD_SCRIPT"]; ok && buildScript != "" {
		annotations["cnb-bp-node-run-scripts"] = buildScript
	}

	return annotations
}

// createPlatformVolume creates a DownwardAPI volume for platform/env
func (c *cnbBuild) createPlatformVolume(re *Request) (*corev1.Volume, *corev1.VolumeMount) {
	annotations := c.buildPlatformAnnotations(re)
	if len(annotations) == 0 {
		return nil, nil
	}

	// Map annotation keys to BP_* env file names
	annotationToEnvName := map[string]string{
		"cnb-bp-web-server":                  "BP_WEB_SERVER",
		"cnb-bp-web-server-root":             "BP_WEB_SERVER_ROOT",
		"cnb-bp-web-server-enable-push-state": "BP_WEB_SERVER_ENABLE_PUSH_STATE",
		"cnb-bp-node-run-scripts":            "BP_NODE_RUN_SCRIPTS",
	}

	// Build DownwardAPI items from annotations
	var items []corev1.DownwardAPIVolumeFile
	for key := range annotations {
		envName, ok := annotationToEnvName[key]
		if !ok {
			continue
		}
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

// createAuthSecret creates the registry authentication secret
func (c *cnbBuild) createAuthSecret(re *Request) (corev1.Secret, error) {
	// Reuse the dockerfile build's auth secret creation logic
	d := &dockerfileBuild{}
	return d.createAuthSecret(re)
}

// deleteAuthSecret deletes the registry authentication secret
func (c *cnbBuild) deleteAuthSecret(re *Request, secretName string) {
	err := re.KubeClient.CoreV1().Secrets(re.RbdNamespace).Delete(context.Background(), secretName, metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("delete auth secret error: %v", err.Error())
	}
}

// waitingComplete waits for the build job to complete
func (c *cnbBuild) waitingComplete(re *Request, reChan *channels.RingChannel) error {
	var logComplete = false
	var jobComplete = false
	var err error

	timeout := time.NewTimer(time.Minute * 60)
	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("CNB build time out (more than 60 minutes)")
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
				err = fmt.Errorf("CNB build job exec failure")
				if logComplete {
					return err
				}
				re.Logger.Info(util.Translation("CNB build job failed"), map[string]string{"step": "build-exector"})
			case "cancel":
				jobComplete = true
				err = fmt.Errorf("CNB build job is canceled")
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
