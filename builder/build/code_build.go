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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/eapache/channels"
	"github.com/fsnotify/fsnotify"
	"github.com/goodrain/rainbond/builder"
	jobc "github.com/goodrain/rainbond/builder/job"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func slugBuilder() (Build, error) {
	return &slugBuild{}, nil
}

type slugBuild struct {
	tgzDir        string
	buildCacheDir string
	sourceDir     string
	re            *Request
}

func (s *slugBuild) Build(re *Request) (*Response, error) {
	re.Logger.Info(util.Translation("Start compiling the source code"), map[string]string{"step": "build-exector"})
	s.tgzDir = re.TGZDir
	s.re = re
	s.buildCacheDir = re.CacheDir
	packageName := fmt.Sprintf("%s/%s.tgz", s.tgzDir, re.DeployVersion)
	//Stops previous build tasks for the same component
	//If an error occurs, it does not affect the current build task
	if err := s.stopPreBuildJob(re); err != nil {
		logrus.Errorf("stop pre build job for service %s failure %s", re.ServiceID, err.Error())
	}
	if err := s.runBuildJob(re); err != nil {
		re.Logger.Error(util.Translation("Compiling the source code failure"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build slug in container error,", err.Error())
		return nil, err
	}
	re.Logger.Info("code build success", map[string]string{"step": "build-exector"})
	defer func() {
		if err := os.Remove(packageName); err != nil && !strings.Contains(err.Error(), "no such file or directory") {
			logrus.Warningf("pkg name: %s; remove slug pkg: %v", packageName, err)
		}
	}()
	fileInfo, err := os.Stat(packageName)
	if err != nil {
		re.Logger.Error(util.Translation("Check that the build result failure"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build package check error", err.Error())
		return nil, fmt.Errorf("build package failure")
	}
	if fileInfo.Size() == 0 {
		re.Logger.Error(util.Translation("Source build failure and result slug size is 0"),
			map[string]string{"step": "build-code", "status": "failure"})
		return nil, fmt.Errorf("build package failure")
	}
	//After 5.1.5 version, wrap slug pacage in the runner image
	imageName, err := s.buildRunnerImage(packageName)
	if err != nil {
		re.Logger.Error(util.Translation("Build runner image failure"),
			map[string]string{"step": "build-code", "status": "failure"})
		return nil, fmt.Errorf("build runner image failure")
	}
	re.Logger.Info(util.Translation("build runtime image success"), map[string]string{"step": "build-code", "status": "success"})
	res := &Response{
		MediumType: ImageMediumType,
		MediumPath: imageName,
	}
	return res, nil
}
func (s *slugBuild) writeRunDockerfile(sourceDir, packageName string, envs map[string]string) error {
	runDockerfile := `
	 FROM %s
	 COPY %s /tmp/slug/slug.tgz
	 RUN chown rain:rain /tmp/slug/slug.tgz
	 ENV VERSION=%s
	`
	result := util.ParseVariable(fmt.Sprintf(runDockerfile, builder.RUNNERIMAGENAME, packageName, s.re.DeployVersion), envs)
	return ioutil.WriteFile(path.Join(sourceDir, "Dockerfile"), []byte(result), 0755)
}

//buildRunnerImage Wrap slug in the runner image
func (s *slugBuild) buildRunnerImage(slugPackage string) (string, error) {
	imageName := fmt.Sprintf("%s/%s:%s", builder.REGISTRYDOMAIN, s.re.ServiceID, s.re.DeployVersion)
	cacheDir := path.Join(path.Dir(slugPackage), "."+s.re.DeployVersion)
	if err := util.CheckAndCreateDir(cacheDir); err != nil {
		return "", fmt.Errorf("create cache package dir failure %s", err.Error())
	}
	defer func() {
		if err := os.RemoveAll(cacheDir); err != nil {
			logrus.Errorf("remove cache dir %s failure %s", cacheDir, err.Error())
		}
	}()

	packageName := path.Base(slugPackage)
	if err := util.Rename(slugPackage, path.Join(cacheDir, packageName)); err != nil {
		return "", fmt.Errorf("move code package failure %s", err.Error())
	}
	//write default runtime dockerfile
	if err := s.writeRunDockerfile(cacheDir, packageName, s.re.BuildEnvs); err != nil {
		return "", fmt.Errorf("write default runtime dockerfile error:%s", err.Error())
	}
	//build runtime image
	runbuildOptions := types.ImageBuildOptions{
		Tags:   []string{imageName},
		Remove: true,
	}
	if _, ok := s.re.BuildEnvs["NO_CACHE"]; ok {
		runbuildOptions.NoCache = true
	} else {
		runbuildOptions.NoCache = false
	}
	// pull image runner
	if _, err := sources.ImagePull(s.re.DockerClient, builder.RUNNERIMAGENAME, builder.REGISTRYUSER, builder.REGISTRYPASS, s.re.Logger, 30); err != nil {
		return "", fmt.Errorf("pull image %s: %v", builder.RUNNERIMAGENAME, err)
	}
	logrus.Infof("pull image %s successfully.", builder.RUNNERIMAGENAME)
	_, err := sources.ImageBuild(s.re.DockerClient, cacheDir, runbuildOptions, s.re.Logger, 30)
	if err != nil {
		s.re.Logger.Error(fmt.Sprintf("build image %s of new version failure", imageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return "", err
	}
	// check build image exist
	_, err = sources.ImageInspectWithRaw(s.re.DockerClient, imageName)
	if err != nil {
		s.re.Logger.Error(fmt.Sprintf("build image %s of service version failure", imageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("get image inspect error: %s", err.Error())
		return "", err
	}
	s.re.Logger.Info("build image of new version success, will push to local registry", map[string]string{"step": "builder-exector"})
	err = sources.ImagePush(s.re.DockerClient, imageName, builder.REGISTRYUSER, builder.REGISTRYPASS, s.re.Logger, 10)
	if err != nil {
		s.re.Logger.Error("push image failure", map[string]string{"step": "builder-exector"})
		logrus.Errorf("push image error: %s", err.Error())
		return "", err
	}
	s.re.Logger.Info("push image of new version success", map[string]string{"step": "builder-exector"})
	if err := sources.ImageRemove(s.re.DockerClient, imageName); err != nil {
		logrus.Errorf("remove image %s failure %s", imageName, err.Error())
	}
	return imageName, nil
}

func (s *slugBuild) readLogFile(logfile string, logger event.Logger, closed chan struct{}) {
	file, _ := os.Open(logfile)
	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()
	_ = watcher.Add(logfile)
	readerr := bufio.NewReader(file)
	for {
		line, _, err := readerr.ReadLine()
		if err != nil {
			if err != io.EOF {
				logrus.Errorf("Read build container log error:%s", err.Error())
				return
			}
			wait := func() error {
				for {
					select {
					case <-closed:
						return nil
					case evt := <-watcher.Events:
						if evt.Op&fsnotify.Write == fsnotify.Write {
							return nil
						}
					case err := <-watcher.Errors:
						return err
					}
				}
			}
			if err := wait(); err != nil {
				logrus.Errorf("Read build container log error:%s", err.Error())
				return
			}
		}
		if logger != nil {
			var message = make(map[string]string)
			if err := ffjson.Unmarshal(line, &message); err == nil {
				if m, ok := message["log"]; ok {
					logger.Info(m, map[string]string{"step": "build-exector"})
				}
			} else {
				fmt.Println(err.Error())
			}
		}
		select {
		case <-closed:
			return
		default:
		}
	}
}

func (s *slugBuild) getSourceCodeTarFile(re *Request) (string, error) {
	var cmd []string
	sourceTarFile := fmt.Sprintf("%s/%s-%s.tar", util.GetParentDirectory(re.SourceDir), re.ServiceID, re.DeployVersion)
	if re.ServerType == "svn" {
		cmd = append(cmd, "tar", "-cf", sourceTarFile, "./")
	}
	if re.ServerType == "git" {
		cmd = append(cmd, "tar", "-cf", sourceTarFile, "./")
	}
	source := exec.Command(cmd[0], cmd[1:]...)
	source.Dir = re.SourceDir
	logrus.Debugf("tar source code to file %s", sourceTarFile)
	if err := source.Run(); err != nil && err.Error() != "exit status 1" {
		return "", fmt.Errorf("command %s: %v", source.String(), err)
	}
	return sourceTarFile, nil
}

//stopPreBuildJob Stops previous build tasks for the same component
//The same component retains only one build task to perform
func (s *slugBuild) stopPreBuildJob(re *Request) error {
	jobList, err := jobc.GetJobController().GetServiceJobs(re.ServiceID)
	if err != nil {
		logrus.Errorf("get pre build job for service %s failure ,%s", re.ServiceID, err.Error())
	}
	if jobList != nil && len(jobList) > 0 {
		for _, job := range jobList {
			jobc.GetJobController().DeleteJob(job.Name)
		}
	}
	return nil
}

func (s *slugBuild) createVolumeAndMount(re *Request, sourceTarFileName string) (volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) {
	slugSubPath := strings.TrimPrefix(re.TGZDir, "/grdata/")
	sourceTarPath := strings.TrimPrefix(sourceTarFileName, "/cache/")
	cacheSubPath := strings.TrimPrefix(re.CacheDir, "/cache/")

	hostPathType := corev1.HostPathDirectoryOrCreate
	unset := corev1.HostPathUnset
	if re.CacheMode == "hostpath" {
		volumeMounts = []corev1.VolumeMount{
			{
				Name:      "cache",
				MountPath: "/tmp/cache",
			},
			{
				Name:      "slug",
				MountPath: "/tmp/slug",
				SubPath:   slugSubPath,
			},
			{
				Name:      "source-file",
				MountPath: "/tmp/app-source.tar",
			},
		}
		volumes = []corev1.Volume{
			{
				Name: "slug",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: s.re.GRDataPVCName,
					},
				},
			},
			{
				Name: "cache",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: path.Join(re.CachePath, cacheSubPath),
						Type: &hostPathType,
					},
				},
			},
			{
				Name: "source-file",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: path.Join(re.CachePath, sourceTarPath),
						// host file type can not auto create parent dir, so can not use.
						Type: &unset,
					},
				},
			},
		}
	} else {
		volumes = []corev1.Volume{
			{
				Name: "slug",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: s.re.GRDataPVCName,
					},
				},
			},
			{
				Name: "app",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: re.CachePVCName,
					},
				},
			},
		}
		volumeMounts = []corev1.VolumeMount{
			{
				Name:      "app",
				MountPath: "/tmp/cache",
				SubPath:   cacheSubPath,
			},
			{
				Name:      "slug",
				MountPath: "/tmp/slug",
				SubPath:   slugSubPath,
			},
			{
				Name:      "app",
				MountPath: "/tmp/app-source.tar",
				SubPath:   sourceTarPath,
			},
		}
	}
	return volumes, volumeMounts
}

func (s *slugBuild) runBuildJob(re *Request) error {
	//prepare build code dir
	re.Logger.Info(util.Translation("Start make code package"), map[string]string{"step": "build-exector"})
	start := time.Now()
	sourceTarFileName, err := s.getSourceCodeTarFile(re)
	if err != nil {
		return fmt.Errorf("create source code tar file error:%s", err.Error())
	}
	re.Logger.Info(util.Translation("make code package success"), map[string]string{"step": "build-exector"})
	logrus.Infof("package code for building service %s version %s successful, take time %s", re.ServiceID, re.DeployVersion, time.Now().Sub(start))
	// remove source cache tar file
	defer func() {
		os.Remove(sourceTarFileName)
	}()
	name := fmt.Sprintf("%s-%s", re.ServiceID, re.DeployVersion)
	namespace := re.RbdNamespace
	job := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"service": re.ServiceID,
				"job":     "codebuild",
			},
		},
	}
	envs := []corev1.EnvVar{
		{Name: "SLUG_VERSION", Value: re.DeployVersion},
		{Name: "SERVICE_ID", Value: re.ServiceID},
		{Name: "TENANT_ID", Value: re.TenantID},
		{Name: "LANGUAGE", Value: re.Lang.String()},
	}
	var mavenSettingName string
	for k, v := range re.BuildEnvs {
		if k == "MAVEN_SETTING_NAME" {
			mavenSettingName = v
			continue
		}
		if k == "PROCFILE" {
			if !strings.HasPrefix(v, "web:") {
				v = "web: " + v
			} else if v[4] != ' ' {
				v = "web: " + v[4:]
			}
		}
		envs = append(envs, corev1.EnvVar{Name: k, Value: v})
		if k == "PROC_ENV" {
			var mapdata = make(map[string]interface{})
			if err := json.Unmarshal([]byte(v), &mapdata); err == nil {
				if runtime, ok := mapdata["runtimes"]; ok {
					envs = append(envs, corev1.EnvVar{Name: "RUNTIME", Value: runtime.(string)})
				}
			}
		}
	}
	podSpec := corev1.PodSpec{RestartPolicy: corev1.RestartPolicyOnFailure} // only support never and onfailure
	// schedule builder
	if re.CacheMode == "hostpath" {
		logrus.Debugf("builder cache mode using hostpath, schedule job into current node")
		hostIP := os.Getenv("HOST_IP")
		if hostIP != "" {
			podSpec.NodeSelector = map[string]string{
				"kubernetes.io/hostname": hostIP,
			}
			podSpec.Tolerations = []corev1.Toleration{
				{
					Operator: "Exists",
				},
			}
		}
	}
	logrus.Debugf("request is: %+v", re)

	volumes, mounts := s.createVolumeAndMount(re, sourceTarFileName)
	podSpec.Volumes = volumes
	container := corev1.Container{
		Name:      name,
		Image:     builder.BUILDERIMAGENAME,
		Stdin:     true,
		StdinOnce: true,
		Env:       envs,
		Args:      []string{"local"},
	}
	container.VolumeMounts = mounts
	//set maven setting
	var mavenSettingConfigName string
	if mavenSettingName != "" && re.Lang.String() == code.JavaMaven.String() {
		if setting := jobc.GetJobController().GetLanguageBuildSetting(re.Ctx, code.JavaMaven, mavenSettingName); setting != "" {
			mavenSettingConfigName = setting
		}
	}
	if mavenSettingConfigName == "" {
		if settingName := jobc.GetJobController().GetDefaultLanguageBuildSetting(re.Ctx, code.JavaMaven); settingName != "" {
			mavenSettingConfigName = settingName
		} else {
			logrus.Warnf("maven setting config %s not found", mavenSettingName)
		}
	}
	if mavenSettingConfigName != "" {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "mavensetting",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mavenSettingConfigName,
					},
				},
			},
		})
		mountPath := "/etc/maven/setting.xml"
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			MountPath: mountPath,
			SubPath:   "mavensetting",
			Name:      "mavensetting",
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "MAVEN_SETTINGS_PATH",
			Value: mountPath,
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "MAVEN_MIRROR_DISABLE",
			Value: "true",
		})
		logrus.Infof("set maven setting config %s success", mavenSettingName)
	}
	podSpec.Containers = append(podSpec.Containers, container)
	for _, ha := range re.HostAlias {
		podSpec.HostAliases = append(podSpec.HostAliases, corev1.HostAlias{IP: ha.IP, Hostnames: ha.Hostnames})
	}
	job.Spec = podSpec
	s.setImagePullSecretsForPod(&job)
	writer := re.Logger.GetWriter("builder", "info")
	reChan := channels.NewRingChannel(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logrus.Debugf("create job[name: %s; namespace: %s]", job.Name, job.Namespace)
	err = jobc.GetJobController().ExecJob(ctx, &job, writer, reChan)
	if err != nil {
		logrus.Errorf("create new job:%s failed: %s", name, err.Error())
		return err
	}
	re.Logger.Info(util.Translation("create build code job success"), map[string]string{"step": "build-exector"})
	logrus.Infof("create build job %s for service %s build version %s", job.Name, re.ServiceID, re.DeployVersion)
	// delete job after complete
	defer jobc.GetJobController().DeleteJob(job.Name)
	return s.waitingComplete(re, reChan)
}

func (s *slugBuild) waitingComplete(re *Request, reChan *channels.RingChannel) (err error) {
	var logComplete = false
	var jobComplete = false
	timeout := time.NewTimer(time.Minute * 60)
	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("build time out (more than 60 minute)")
		case jobStatus := <-reChan.Out():
			status := jobStatus.(string)
			switch status {
			case "complete":
				jobComplete = true
				if logComplete {
					return nil
				}
				re.Logger.Info(util.Translation("build code job exec completed"), map[string]string{"step": "build-exector"})
			case "failed":
				jobComplete = true
				err = fmt.Errorf("build code job exec failure")
				if logComplete {
					return err
				}
				re.Logger.Info(util.Translation("build code job exec failed"), map[string]string{"step": "build-exector"})
			case "cancel":
				jobComplete = true
				err = fmt.Errorf("build code job is canceled")
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

func (s *slugBuild) setImagePullSecretsForPod(pod *corev1.Pod) {
	imagePullSecretName := os.Getenv("IMAGE_PULL_SECRET")
	if imagePullSecretName == "" {
		return
	}

	pod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
		{Name: imagePullSecretName},
	}
}

//ErrorBuild build error
type ErrorBuild struct {
	Code int
}

func (e *ErrorBuild) Error() string {
	return fmt.Sprintf("Run build return %d", e.Code)
}
