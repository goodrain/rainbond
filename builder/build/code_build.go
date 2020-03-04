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
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/fsnotify/fsnotify"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
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
	ctx, cancel := context.WithCancel(re.Ctx)
	defer cancel()
	logrus.Info("start build job")
	// delete .git and .svn folder
	if err := s.prepareSourceCodeFile(re); err != nil {
		logrus.Error("delete .git and .svn folder error")
		return err
	}
	name := fmt.Sprintf("%s-%s", re.ServiceID, re.Commit.Hash[0:7])
	namespace := "rbd-system"
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
	job.Namespace = namespace
	podTempSpec := corev1.PodTemplateSpec{}
	podTempSpec.Name = name
	podTempSpec.Namespace = namespace

	podSpec := corev1.PodSpec{RestartPolicy: corev1.RestartPolicyOnFailure} // only support never and onfailure
	podSpec.Volumes = []corev1.Volume{
		corev1.Volume{
			Name: "slug",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: s.re.GRDataPVCName,
				},
			},
		},
		corev1.Volume{
			Name: "app",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: s.re.CachePVCName,
				},
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
	cacheSubPath := strings.TrimPrefix(re.CacheDir, "/cache/")
	container.VolumeMounts = []corev1.VolumeMount{
		corev1.VolumeMount{
			Name:      "app",
			MountPath: "/tmp/cache",
			SubPath:   cacheSubPath,
		},
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
	for _, ha := range re.HostAlias {
		podSpec.HostAliases = append(podSpec.HostAliases, corev1.HostAlias{IP: ha.IP, Hostnames: ha.Hostnames})
	}
	podTempSpec.Spec = podSpec
	job.Spec.Template = podTempSpec

	_, err := re.KubeClient.BatchV1().Jobs(namespace).Create(&job)
	if err != nil {
		if !k8sErrors.IsAlreadyExists(err) {
			logrus.Errorf("create new job:%s failed: %s", name, err.Error())
			return err
		}
		_, err := re.KubeClient.BatchV1().Jobs(namespace).Get(job.Name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("get old job:%s failed : %s", name, err.Error())
			return err
		}

		waitChan := make(chan struct{})
		// if get old job, must clean it before re create a new one
		go waitOldJobDeleted(ctx, waitChan, re.KubeClient, namespace, name)

		var gracePeriod int64 = 0
		if err := re.KubeClient.BatchV1().Jobs(namespace).Delete(job.Name, &metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		}); err != nil {
			logrus.Errorf("get old job:%s failed: %s", name, err.Error())
			return err
		}

		<-waitChan
		logrus.Infof("old job has beed cleaned, create new job: %s", job.Name)

		if _, err := re.KubeClient.BatchV1().Jobs(namespace).Create(&job); err != nil {
			logrus.Errorf("create new job:%s failed: %s", name, err.Error())
			return err
		}
	}

	defer delete(re.KubeClient, namespace, job.Name)

	// get job builder log and delete job util it is finished
	writer := re.Logger.GetWriter("builder", "info")

	podChan := make(chan struct{})

	go getJobPodLogs(ctx, podChan, re.KubeClient, writer, namespace, job.Name)
	getJob(ctx, podChan, re.KubeClient, namespace, job.Name)

	return nil
}

func waitOldJobDeleted(ctx context.Context, waitChan chan struct{}, clientset kubernetes.Interface, namespace, name string) {
	labelSelector := fmt.Sprintf("job-name=%s", name)
	jobWatch, err := clientset.BatchV1().Jobs(namespace).Watch(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		logrus.Errorf("watch job: %s failed: %s", name, err.Error())
		return
	}

	for {
		select {
		case <-time.After(30 * time.Second):
			logrus.Warnf("wait old job[%s] cleaned time out", name)
			waitChan <- struct{}{}
			return
		case <-ctx.Done():
			return
		case evt, ok := <-jobWatch.ResultChan():
			if !ok {
				logrus.Error("old job watch chan be closed")
				return
			}
			switch evt.Type {
			case watch.Deleted:
				logrus.Infof("old job deleted : %s", name)
				waitChan <- struct{}{}
				return
			}
		}
	}
}

func getJob(ctx context.Context, podChan chan struct{}, clientset kubernetes.Interface, namespace, name string) {
	var job *batchv1.Job
	labelSelector := fmt.Sprintf("job-name=%s", name)
	jobWatch, err := clientset.BatchV1().Jobs(namespace).Watch(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		logrus.Errorf("watch job: %s failed: %s", name, err.Error())
		return
	}

	once := sync.Once{}
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-jobWatch.ResultChan():
			if !ok {
				logrus.Error("job watch chan be closed")
				return
			}
			switch evt.Type {
			case watch.Modified, watch.Added:
				job, _ = evt.Object.(*batchv1.Job)
				if job.Name == name {
					logrus.Debugf("job: %s status is: %+v ", name, job.Status)
					// active means this job has bound a pod, can't ensure this pod's status is running or creating or initing or some status else
					if job.Status.Active > 0 {
						once.Do(func() {
							logrus.Debug("job is ready")
							waitPod(ctx, podChan, clientset, namespace, name)
							podChan <- struct{}{}
						})
					}
					if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
						logrus.Debug("job is finished")
						return
					}
				}
			case watch.Error:
				logrus.Errorf("job: %s error", name)
				return
			case watch.Deleted:
				logrus.Infof("job deleted : %s", name)
				return
			}

		}
	}
}

func waitPod(ctx context.Context, podChan chan struct{}, clientset kubernetes.Interface, namespace, name string) {
	logrus.Debug("waiting pod")
	var pod *corev1.Pod
	labelSelector := fmt.Sprintf("job-name=%s", name)
	podWatch, err := clientset.CoreV1().Pods(namespace).Watch(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-podWatch.ResultChan():
			if !ok {
				logrus.Error("pod watch chan be closed")
				return
			}
			switch evt.Type {
			case watch.Added, watch.Modified:
				pod, _ = evt.Object.(*corev1.Pod)
				logrus.Debugf("pod status is : %s", pod.Status.Phase)
				if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].Ready {
					logrus.Debug("pod is running")
					return
				}
			case watch.Deleted:
				logrus.Infof("pod : %s deleted", name)
				return
			case watch.Error:
				logrus.Errorf("pod : %s error", name)
				return
			}
		}
	}
}

func getJobPodLogs(ctx context.Context, podChan chan struct{}, clientset kubernetes.Interface, writer event.LoggerWriter, namespace, job string) {
	once := sync.Once{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-podChan:
			once.Do(func() {
				logrus.Debug("pod ready")
				labelSelector := fmt.Sprintf("job-name=%s", job)
				pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
				if err != nil {
					logrus.Errorf("do not found job's pod, %s", err.Error())
					return
				}
				logrus.Debug("pod name is : ", pods.Items[0].Name)
				podLogRequest := clientset.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{Follow: true})
				reader, err := podLogRequest.Stream()
				if err != nil {
					logrus.Warnf("get build job pod log data error: %s, retry net loop", err.Error())
					return
				}
				defer reader.Close()
				bufReader := bufio.NewReader(reader)
				for {
					line, err := bufReader.ReadBytes('\n')
					writer.Write(line)
					if err == io.EOF {
						logrus.Info("get job log finished(io.EOF)")
						return
					}
					if err != nil {
						logrus.Warningf("get job log error: %s", err.Error())
						return
					}
				}
			})
		}
	}
}

func delete(clientset kubernetes.Interface, namespace, job string) {
	logrus.Debugf("start delete job: %s", job)
	listOptions := metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", job)}

	if err := clientset.CoreV1().Pods(namespace).DeleteCollection(&metav1.DeleteOptions{}, listOptions); err != nil {
		logrus.Errorf("delete job pod failed: %s", err.Error())
	}

	// delete job
	if err := clientset.BatchV1().Jobs(namespace).Delete(job, &metav1.DeleteOptions{}); err != nil {
		logrus.Errorf("delete job failed: %s", err.Error())
	}

	logrus.Debug("delete job finish")

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
