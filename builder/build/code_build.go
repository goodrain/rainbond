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

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/fsnotify/fsnotify"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"

	batchv1 "k8s.io/api/batch/v1"
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
	if err := s.runBuildJob(re); err != nil {
		re.Logger.Error(util.Translation("Compiling the source code failure"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build slug in container error,", err.Error())
		return nil, err
	}
	defer func() {
		if err := os.Remove(packageName); err != nil {
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
	re.Logger.Info(util.Translation("Compiling the source code SUCCESS"), map[string]string{"step": "build-code", "status": "success"})
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
	err := sources.ImageBuild(s.re.DockerClient, cacheDir, runbuildOptions, s.re.Logger, 30)
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
					case event := <-watcher.Events:
						if event.Op&fsnotify.Write == fsnotify.Write {
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

func (s *slugBuild) getSourceCodeTarFile(re *Request) (*os.File, error) {
	var cmd []string
	sourceTarFile := fmt.Sprintf("%s/%s-%s.tar", util.GetParentDirectory(re.SourceDir), re.ServiceID, re.DeployVersion)
	if re.ServerType == "svn" {
		cmd = append(cmd, "tar", "-cf", sourceTarFile, "--exclude=.svn", "./")
	}
	if re.ServerType == "git" {
		cmd = append(cmd, "tar", "-cf", sourceTarFile, "--exclude=.git", "./")
	}
	source := exec.Command(cmd[0], cmd[1:]...)
	source.Dir = re.SourceDir
	logrus.Debugf("tar source code to file %s", sourceTarFile)
	if err := source.Run(); err != nil {
		return nil, err
	}
	return os.OpenFile(sourceTarFile, os.O_RDONLY, 0755)
}

func (s *slugBuild) prepareSourceCodeFile(re *Request) error {
	var cmd []string
	if re.ServerType == "svn" {
		cmd = append(cmd, "rm", "-rf", path.Join(re.SourceDir, "./.svn"))
	}
	if re.ServerType == "git" {
		cmd = append(cmd, "rm", "-rf", path.Join(re.SourceDir, "./.git"))
	}
	source := exec.Command(cmd[0], cmd[1:]...)
	if err := source.Run(); err != nil {
		return err
	}
	logrus.Debug("delete .git and .svn folder success")
	return nil
}

func (s *slugBuild) runBuildJob(re *Request) error {
	fmt.Println("start build job")
	// delete .git and .svn folder
	if err := s.prepareSourceCodeFile(re); err != nil {
		logrus.Error("delete .git and .svn folder error")
		return err
	}
	name := re.ServiceID
	// namespace := "rbd-system"
	envs := []corev1.EnvVar{
		corev1.EnvVar{Name: "SLUG_VERSION", Value: re.DeployVersion},
		corev1.EnvVar{Name: "SERVICE_ID", Value: re.ServiceID},
		corev1.EnvVar{Name: "TENANT_ID", Value: re.TenantID},
		corev1.EnvVar{Name: "LANGUAGE", Value: re.Lang.String()},
	}
	for k, v := range re.BuildEnvs {
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
	job := batchv1.Job{}
	job.Name = name
	job.Namespace = "rbd-system"
	var ttl int32
	ttl = 20
	job.Spec.TTLSecondsAfterFinished = &ttl //  k8s version >= 1.12
	podTempSpec := corev1.PodTemplateSpec{}
	podTempSpec.Name = name
	podTempSpec.Namespace = "rbd-system"

	podSpec := corev1.PodSpec{RestartPolicy: corev1.RestartPolicyOnFailure} // only support never and onfailure
	// hostPathType := corev1.HostPathDirectoryOrCreate
	podSpec.Volumes = []corev1.Volume{
		// corev1.Volume{
		// 	Name: "cache",
		// 	VolumeSource: corev1.VolumeSource{
		// 		// PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
		// 		// 	ClaimName: "cache",
		// 		// },
		// 		HostPath: &corev1.HostPathVolumeSource{Path: re.CacheDir, Type: &hostPathType},
		// 	},
		// },
		corev1.Volume{
			Name: "slug",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "grdata",
				},
				// HostPath: &corev1.HostPathVolumeSource{Path: re.TGZDir, Type: &hostPathType},
			},
		},
		corev1.Volume{
			Name: "app",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "cache",
				},
				// HostPath: &corev1.HostPathVolumeSource{Path: re.SourceDir, Type: &hostPathType},
			},
		},
	}
	container := corev1.Container{Name: name, Image: builder.BUILDERIMAGENAME, Stdin: true, StdinOnce: true}
	container.Env = envs
	container.Args = []string{"local"}
	slugSubPath := strings.TrimPrefix(re.TGZDir, "/grdata/")
	logrus.Debugf("slug subpath is : %s", slugSubPath)
	appSubPath := strings.TrimPrefix(re.SourceDir, "/cache/")
	logrus.Debugf("app subpath is : %s", appSubPath)
	container.VolumeMounts = []corev1.VolumeMount{
		corev1.VolumeMount{
			Name:      "slug",
			MountPath: "/tmp/slug",
			SubPath:   slugSubPath,
		},
		corev1.VolumeMount{
			Name:      "app",
			MountPath: "/tmp/app",
			SubPath:   appSubPath,
		},
	}
	podSpec.Containers = append(podSpec.Containers, container)
	for _, ha := range re.HostAlias { // TODO fanyangyang wait k8s cluster
		podSpec.HostAliases = append(podSpec.HostAliases, corev1.HostAlias{IP: ha.IP, Hostnames: ha.Hostnames})
	}
	podTempSpec.Spec = podSpec
	job.Spec.Template = podTempSpec
	_, err := re.KubeClient.BatchV1().Jobs("rbd-system").Create(&job)
	if err != nil {
		return err
	}
	writer := re.Logger.GetWriter("builder", "info")
	for {
		logrus.Debug("waiting job finish")
		time.Sleep(5 * time.Second)
		job, err := re.KubeClient.BatchV1().Jobs("rbd-system").Get(re.ServiceID, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("get job error: %s", err.Error())
		}
		if job == nil {
			continue
		}
		if job.Status.Active > 0 {
			logrus.Debug("build job is active")
			var po corev1.Pod
			labelSelector := fmt.Sprintf("job-name=%s", re.ServiceID)
			for {
				pos, err := re.KubeClient.CoreV1().Pods("rbd-system").List(metav1.ListOptions{LabelSelector: labelSelector})
				if err != nil {
					logrus.Errorf(" get po error: %s", err.Error())
				}
				if len(pos.Items) == 0 {
					continue
				}
				if len(pos.Items[0].Spec.Containers) > 0 {
					logrus.Debug("pod container ready, start write log")
					po = pos.Items[0]
					break
				}
				time.Sleep(5 * time.Second)
			}
			podLogRequest := re.KubeClient.CoreV1().Pods("rbd-system").GetLogs(po.Name, &corev1.PodLogOptions{})
			reader, err := podLogRequest.Stream()
			if err != nil {
				logrus.Warnf("get build job pod log data error: %s, retry net loop", err.Error())
				continue
			}
			defer reader.Close()
			bufReader := bufio.NewReader(reader)
			for {
				line, err := bufReader.ReadBytes('\n')
				writer.Write(line)
				if err == io.EOF {
					break
				}
				if err != nil {
					logrus.Warningf("get job log error: %s, retry next loop", err.Error())
					break
				}

			}
		}
		if job.Status.Succeeded > 0 {
			writer.Write([]byte("build job have done successfully"))
			if err = re.KubeClient.BatchV1().Jobs("rbd-system").Delete(re.ServiceID, &metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("delete job failed: %s", err.Error())
			}
			break
		}
		if job.Status.Failed > 0 {
			writer.Write([]byte("build job have done failed"))
			if err = re.KubeClient.BatchV1().Jobs("rbd-system").Delete(re.ServiceID, &metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("delete job failed: %s", err.Error())
			}
			break
		}
	}

	return nil
}

func (s *slugBuild) runBuildContainer(re *Request) error {
	envs := []*sources.KeyValue{
		&sources.KeyValue{Key: "SLUG_VERSION", Value: re.DeployVersion},
		&sources.KeyValue{Key: "SERVICE_ID", Value: re.ServiceID},
		&sources.KeyValue{Key: "TENANT_ID", Value: re.TenantID},
		&sources.KeyValue{Key: "LANGUAGE", Value: re.Lang.String()},
	}
	for k, v := range re.BuildEnvs {
		envs = append(envs, &sources.KeyValue{Key: k, Value: v})
		if k == "PROC_ENV" {
			var mapdata = make(map[string]interface{})
			if err := json.Unmarshal([]byte(v), &mapdata); err == nil {
				if runtime, ok := mapdata["runtimes"]; ok {
					envs = append(envs, &sources.KeyValue{Key: "RUNTIME", Value: runtime.(string)})
				}
			}
		}
	}
	containerConfig := &sources.ContainerConfig{
		Metadata: &sources.ContainerMetadata{
			Name: re.ServiceID[:8] + "_" + re.DeployVersion,
		},
		Image: &sources.ImageSpec{
			Image: builder.BUILDERIMAGENAME,
		},
		Mounts: []*sources.Mount{
			&sources.Mount{
				ContainerPath: "/tmp/cache",
				HostPath:      re.CacheDir,
				Readonly:      false,
			},
			&sources.Mount{
				ContainerPath: "/tmp/slug",
				HostPath:      s.tgzDir,
				Readonly:      false,
			},
		},
		Envs:         envs,
		Stdin:        true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		NetworkConfig: &sources.NetworkConfig{
			NetworkMode: "host",
		},
		Args:       []string{"local"},
		ExtraHosts: re.ExtraHosts,
	}
	reader, err := s.getSourceCodeTarFile(re)
	if err != nil {
		return fmt.Errorf("create source code tar file error:%s", err.Error())
	}
	defer func() {
		reader.Close()
		os.Remove(reader.Name())
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	containerService := sources.CreateDockerService(ctx, re.DockerClient)
	containerID, err := containerService.CreateContainer(containerConfig)
	if err != nil {
		if client.IsErrNotFound(err) {
			// we don't want to write to stdout anything apart from container.ID
			if _, err = sources.ImagePull(re.DockerClient, containerConfig.Image.Image, builder.REGISTRYUSER, builder.REGISTRYPASS, re.Logger, 20); err != nil {
				return fmt.Errorf("pull builder container image error:%s", err.Error())
			}
			// Retry
			containerID, err = containerService.CreateContainer(containerConfig)
		}
		//The container already exists.
		if err != nil && strings.Contains(err.Error(), "is already in use by container") {
			//remove exist container
			containerService.RemoveContainer(containerID)
			// Retry
			containerID, err = containerService.CreateContainer(containerConfig)
		}
		if err != nil {
			return fmt.Errorf("create builder container failure %s", err.Error())
		}
	}
	errchan := make(chan error, 1)
	writer := re.Logger.GetWriter("builder", "info")
	close, err := containerService.AttachContainer(containerID, true, true, true, reader, writer, writer, &errchan)
	if err != nil {
		containerService.RemoveContainer(containerID)
		return fmt.Errorf("attach builder container error:%s", err.Error())
	}
	defer close()
	statuschan := containerService.WaitExitOrRemoved(containerID, true)
	//start the container
	if err := containerService.StartContainer(containerID); err != nil {
		containerService.RemoveContainer(containerID)
		return fmt.Errorf("start builder container error:%s", err.Error())
	}
	if err := <-errchan; err != nil {
		logrus.Debugf("Error hijack: %s", err)
	}
	status := <-statuschan
	if status != 0 {
		return &ErrorBuild{Code: status}
	}
	return nil
}

//ErrorBuild build error
type ErrorBuild struct {
	Code int
}

func (e *ErrorBuild) Error() string {
	return fmt.Sprintf("Run build return %d", e.Code)
}
