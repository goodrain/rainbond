// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package sources

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/builder"
	jobc "github.com/goodrain/rainbond/builder/job"
	"github.com/goodrain/rainbond/builder/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

//ErrorNoAuth error no auth
var ErrorNoAuth = fmt.Errorf("pull image require docker login")

//ErrorNoImage error no image
var ErrorNoImage = fmt.Errorf("image not exist")

//ImagePull pull docker image
//timeout minutes of the unit
func ImagePull(containerdClient *containerd.Client, image string, username, password string, logger event.Logger, timeout int) (*images.Image, error) {
	printLog(logger, "info", fmt.Sprintf("start get image:%s", image), map[string]string{"step": "pullimage"})
	rf, err := reference.ParseAnyReference(image)
	if err != nil {
		logrus.Errorf("reference image error: %s", err.Error())
		return nil, err
	}
	//最少一分钟
	if timeout < 1 {
		timeout = 1
	}
	ctx := namespaces.WithNamespace(context.Background(), "rbd-ctr")
	defaultTLS := &tls.Config{
		InsecureSkipVerify: true,
	}
	hostOpt := config.HostOptions{}
	hostOpt.DefaultTLS = defaultTLS
	if username != "" && password != "" {
		hostOpt.Credentials = func(host string) (string, string, error) {
			return username, password, nil
		}
	}
	options := docker.ResolverOptions{
		Tracker: docker.NewInMemoryTracker(),
		Hosts:   config.ConfigureHosts(ctx, hostOpt),
	}
	pullOpts := []containerd.RemoteOpt{
		containerd.WithPullUnpack,
		containerd.WithResolver(docker.NewResolver(options)),
	}

	//TODO: 使用1.12版本api的bug “repository name must be canonical”，使用rf.String()完整的镜像地址
	_, err = containerdClient.Pull(ctx, rf.String(), pullOpts...)
	if err != nil {
		logrus.Debugf("image name: %s readcloser error: %v", image, err.Error())
		if strings.HasSuffix(err.Error(), "does not exist or no pull access") {
			printLog(logger, "error", fmt.Sprintf("image: %s does not exist or is not available", image), map[string]string{"step": "pullimage", "status": "failure"})
			return nil, fmt.Errorf("Image(%s) does not exist or no pull access", image)
		}
		return nil, err
	}
	imageService := containerdClient.ImageService()
	imageObj, err := imageService.Get(ctx, rf.String())
	if err != nil {
		return nil, fmt.Errorf("image(%v) pull error", image)
	}
	logrus.Infof("pull image taget:", imageObj.Target)
	//defer readcloser.Close()
	printLog(logger, "info", fmt.Sprintf("Success Pull Image：%s", image), map[string]string{"step": "pullimage"})
	return &imageObj, nil
}

//ImageTag change docker image tag
func ImageTag(containerdClient *containerd.Client, source, target string, logger event.Logger, timeout int) error {
	logrus.Debugf(fmt.Sprintf("change image tag：%s -> %s", source, target))
	printLog(logger, "info", fmt.Sprintf("change image tag：%s -> %s", source, target), map[string]string{"step": "changetag"})
	ctx := namespaces.WithNamespace(context.Background(), "rbd-ctr")
	imageService := containerdClient.ImageService()
	image, err := imageService.Get(ctx, source)
	if err != nil {
		logrus.Errorf("imagetag imageService Get error: %s", err.Error())
		return err
	}
	image.Name = target
	if _, err = imageService.Create(ctx, image); err != nil {
		if errdefs.IsAlreadyExists(err) {
			if err = imageService.Delete(ctx, image.Name); err != nil {
				logrus.Errorf("imagetag imageService Delete error: %s", err.Error())
				return err
			}
			if _, err = imageService.Create(ctx, image); err != nil {
				logrus.Errorf("imageService Create error: %s", err.Error())
				return err
			}
		} else {
			logrus.Errorf("imagetag imageService Create error: %s", err.Error())
			return err
		}
	}
	logrus.Info("change image tag success")
	printLog(logger, "info", "change image tag success", map[string]string{"step": "changetag"})
	return nil
}

//ImageNameHandle 解析imagename
func ImageNameHandle(imageName string) *model.ImageName {
	var i model.ImageName
	if strings.Contains(imageName, "/") {
		mm := strings.Split(imageName, "/")
		i.Host = mm[0]
		names := strings.Join(mm[1:], "/")
		if strings.Contains(names, ":") {
			nn := strings.Split(names, ":")
			i.Name = nn[0]
			i.Tag = nn[1]
		} else {
			i.Name = names
			i.Tag = "latest"
		}
	} else {
		if strings.Contains(imageName, ":") {
			nn := strings.Split(imageName, ":")
			i.Name = nn[0]
			i.Tag = nn[1]
		} else {
			i.Name = imageName
			i.Tag = "latest"
		}
	}
	return &i
}

//ImageNameWithNamespaceHandle if have namespace,will parse namespace
func ImageNameWithNamespaceHandle(imageName string) *model.ImageName {
	var i model.ImageName
	if strings.Contains(imageName, "/") {
		mm := strings.Split(imageName, "/")
		i.Host = mm[0]
		names := strings.Join(mm[1:], "/")
		if len(mm) >= 3 {
			i.Namespace = mm[1]
			names = strings.Join(mm[2:], "/")
		}
		if strings.Contains(names, ":") {
			nn := strings.Split(names, ":")
			i.Name = nn[0]
			i.Tag = nn[1]
		} else {
			i.Name = names
			i.Tag = "latest"
		}
	} else {
		if strings.Contains(imageName, ":") {
			nn := strings.Split(imageName, ":")
			i.Name = nn[0]
			i.Tag = nn[1]
		} else {
			i.Name = imageName
			i.Tag = "latest"
		}
	}
	return &i
}

// GenSaveImageName generates the final name of the image, which is the name of
// the image in the exported tar package.
func GenSaveImageName(name string) string {
	imageName := ImageNameWithNamespaceHandle(name)
	return fmt.Sprintf("%s:%s", imageName.Name, imageName.Tag)
}

//ImagePush push image to registry
//timeout minutes of the unit
func ImagePush(containerdClient *containerd.Client, image, user, pass string, logger event.Logger, timeout int) error {
	printLog(logger, "info", fmt.Sprintf("start push image：%s", image), map[string]string{"step": "pushimage"})
	if timeout < 1 {
		timeout = 1
	}
	if user == "" {
		user = os.Getenv("LOCAL_HUB_USER")
	}
	if pass == "" {
		pass = os.Getenv("LOCAL_HUB_PASS")
	}
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}
	ctx := namespaces.WithNamespace(context.Background(), "rbd-ctr")
	getImage, err := containerdClient.GetImage(ctx, ref.String())
	if err != nil {
		logrus.Errorf("containerdClient get Image error: %s", err.Error())
		return err
	}
	defaultTLS := &tls.Config{
		InsecureSkipVerify: true,
	}
	hostOpt := config.HostOptions{}
	hostOpt.DefaultTLS = defaultTLS
	hostOpt.Credentials = func(host string) (string, string, error) {
		return user, pass, nil
	}
	options := docker.ResolverOptions{
		Tracker: docker.NewInMemoryTracker(),
		Hosts:   config.ConfigureHosts(ctx, hostOpt),
	}
	pushOpts := []containerd.RemoteOpt{
		containerd.WithResolver(docker.NewResolver(options)),
	}

	logrus.Info("getImage.Target", getImage.Target())
	err = containerdClient.Push(ctx, image, getImage.Target(), pushOpts...)
	if err != nil {
		logrus.Errorf("containerdClient Push Image error: %s", err.Error())
		return err
	}
	// create a container
	printLog(logger, "info", fmt.Sprintf("success push image：%s", image), map[string]string{"step": "pushimage"})
	return nil
}

//TrustedImagePush push image to trusted registry
func TrustedImagePush(containerdClient *containerd.Client, image, user, pass string, logger event.Logger, timeout int) error {
	if err := CheckTrustedRepositories(image, user, pass); err != nil {
		return err
	}
	return ImagePush(containerdClient, image, user, pass, logger, timeout)
}

//CheckTrustedRepositories check Repositories is exist ,if not create it.
func CheckTrustedRepositories(image, user, pass string) error {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}
	var server string
	if reference.IsNameOnly(ref) {
		server = "docker.io"
	} else {
		server = reference.Domain(ref)
	}
	cli, err := createTrustedRegistryClient(server, user, pass)
	if err != nil {
		return err
	}
	var namespace, repName string
	infos := strings.Split(reference.TrimNamed(ref).String(), "/")
	if len(infos) == 3 && infos[0] == server {
		namespace = infos[1]
		repName = infos[2]
	}
	if len(infos) == 2 {
		namespace = infos[0]
		repName = infos[1]
	}
	_, err = cli.GetRepository(namespace, repName)
	if err != nil {
		if err.Error() == "resource does not exist" {
			rep := Repostory{
				Name:             repName,
				ShortDescription: image, // The maximum length is 140
				LongDescription:  fmt.Sprintf("push image for %s", image),
				Visibility:       "private",
			}
			if len(rep.ShortDescription) > 140 {
				rep.ShortDescription = rep.ShortDescription[0:140]
			}
			err := cli.CreateRepository(namespace, &rep)
			if err != nil {
				return fmt.Errorf("create repostory error,%s", err.Error())
			}
			return nil
		}
		return fmt.Errorf("get repostory error,%s", err.Error())
	}
	return err
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func EncodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

//ImageBuild use kaniko build image
func ImageBuild(contextDir, RbdNamespace, ServiceID, DeployVersion string, logger event.Logger, buildType string) error {
	// create image name
	buildImageName := CreateImageName(ServiceID, DeployVersion)
	// The same component retains only one build task to perform
	jobList, err := jobc.GetJobController().GetServiceJobs(ServiceID)
	if err != nil {
		logrus.Errorf("get pre build job for service %s failure ,%s", ServiceID, err.Error())
	}
	if len(jobList) > 0 {
		for _, job := range jobList {
			jobc.GetJobController().DeleteJob(job.Name)
		}
	}
	name := fmt.Sprintf("%s-%s", ServiceID, DeployVersion)
	namespace := RbdNamespace
	job := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"service": ServiceID,
				"job":     "codebuild",
			},
		},
	}
	podSpec := corev1.PodSpec{RestartPolicy: corev1.RestartPolicyOnFailure} // only support never and onfailure
	volumes, volumeMounts := CreateVolumesAndMounts(contextDir, buildType)
	podSpec.Volumes = volumes
	// container config
	container := corev1.Container{
		Name:      name,
		Image:     "yangk/executor:latest",
		Stdin:     true,
		StdinOnce: true,
		Args:      []string{"--context=dir:///workspace", fmt.Sprintf("--destination=%s", buildImageName), "--skip-tls-verify"},
	}
	container.VolumeMounts = volumeMounts
	podSpec.Containers = append(podSpec.Containers, container)
	job.Spec = podSpec
	writer := logger.GetWriter("builder", "info")
	reChan := channels.NewRingChannel(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logrus.Debugf("create job[name: %s; namespace: %s]", job.Name, job.Namespace)
	err = jobc.GetJobController().ExecJob(ctx, &job, writer, reChan)
	if err != nil {
		logrus.Errorf("create new job:%s failed: %s", name, err.Error())
		return err
	}
	logger.Info(util.Translation("create build code job success"), map[string]string{"step": "build-exector"})
	// delete job after complete
	defer jobc.GetJobController().DeleteJob(job.Name)
	err = WaitingComplete(reChan)
	if err != nil {
		logrus.Errorf("waiting complete failed: %s", err.Error())
		return err
	}
	return nil
}

//ImageInspectWithRaw get image inspect
func ImageInspectWithRaw(dockerCli *client.Client, image string) (*types.ImageInspect, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ins, _, err := dockerCli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

//ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func ImageSave(dockerCli *client.Client, image, destination string, logger event.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rc, err := dockerCli.ImageSave(ctx, []string{image})
	if err != nil {
		return err
	}
	defer rc.Close()
	return CopyToFile(destination, rc)
}

//MultiImageSave save multi image to tar file
// destination destination file name eg. /tmp/xxx.tar
func MultiImageSave(ctx context.Context, dockerCli *client.Client, destination string, logger event.Logger, images ...string) error {
	rc, err := dockerCli.ImageSave(ctx, images)
	if err != nil {
		return err
	}
	defer rc.Close()
	return CopyToFile(destination, rc)
}

//ImageLoad load image from  tar file
// destination destination file name eg. /tmp/xxx.tar
func ImageLoad(dockerCli *client.Client, tarFile string, logger event.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := os.OpenFile(tarFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer reader.Close()

	rc, err := dockerCli.ImageLoad(ctx, reader, false)
	if err != nil {
		return err
	}
	if rc.Body != nil {
		defer rc.Body.Close()
		dec := json.NewDecoder(rc.Body)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var jm JSONMessage
			if err := dec.Decode(&jm); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if jm.Error != nil {
				return jm.Error
			}
			logger.Info(jm.JSONString(), map[string]string{"step": "build-progress"})
		}
	}

	return nil
}

//ImageImport save image to tar file
// source source file name eg. /tmp/xxx.tar
func ImageImport(dockerCli *client.Client, image, source string, logger event.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	isource := types.ImageImportSource{
		Source:     file,
		SourceName: "-",
	}

	options := types.ImageImportOptions{}

	readcloser, err := dockerCli.ImageImport(ctx, isource, image, options)
	if err != nil {
		return err
	}
	if readcloser != nil {
		defer readcloser.Close()
		r := bufio.NewReader(readcloser)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if line, _, err := r.ReadLine(); err == nil {
				if logger != nil {
					logger.Debug(string(line), map[string]string{"step": "progress"})
				} else {
					fmt.Println(string(line))
				}
			} else {
				if err.Error() == "EOF" {
					return nil
				}
				return err
			}
		}
	}
	return nil
}

// CopyToFile writes the content of the reader to the specified file
func CopyToFile(outfile string, r io.Reader) error {
	// We use sequential file access here to avoid depleting the standby list
	// on Windows. On Linux, this is a call directly to ioutil.TempFile
	tmpFile, err := os.OpenFile(path.Join(filepath.Dir(outfile), ".docker_temp_"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer tmpFile.Close()
	tmpPath := tmpFile.Name()
	_, err = io.Copy(tmpFile, r)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err = os.Rename(tmpPath, outfile); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

//ImageRemove remove image
func ImageRemove(containerdClient *containerd.Client, image string) error {
	ctx := namespaces.WithNamespace(context.Background(), "rbd-ctr")
	imageStore := containerdClient.ImageService()
	err := imageStore.Delete(ctx, image)
	if err != nil {
		logrus.Errorf("image remove ")
	}
	return err
}

// CheckIfImageExists -
func CheckIfImageExists(containerdClient *containerd.Client, image string) (imageName string, isExists bool, err error) {
	repo, err := reference.Parse(image)
	if err != nil {
		return "", false, fmt.Errorf("parse image %s: %v", image, err)
	}
	named := repo.(reference.Named)
	tag := "latest"
	if t, ok := repo.(reference.Tagged); ok {
		tag = t.Tag()
	}
	imageFullName := named.Name() + ":" + tag

	ctx := namespaces.WithNamespace(context.Background(), "rbd-ctr")
	imageSummarys, err := containerdClient.ListImages(ctx)
	if err != nil {
		return "", false, fmt.Errorf("list images: %v", err)
	}
	for _, imageSummary := range imageSummarys {
		fmt.Printf("%#v", imageSummary.Name())
	}

	_ = imageSummarys

	return imageFullName, len(imageSummarys) > 0, nil
}

// ImagesPullAndPush Used to process mirroring of non local components, example: builder, runner, /rbd-mesh-data-panel
func ImagesPullAndPush(sourceImage, targetImage, username, password string, logger event.Logger) error {
	var sock string
	sock = os.Getenv("CONTAINERD_SOCK")
	if sock == "" {
		sock = "/run/containerd/containerd.sock"
	}
	containerdClient, err := containerd.New(sock)
	if err != nil {
		logrus.Errorf("create docker client failed: %s", err.Error())
		return err
	}
	sourceImage, exists, err := CheckIfImageExists(containerdClient, sourceImage)
	if err != nil {
		logrus.Errorf("failed to check whether the builder mirror exists: %s", err.Error())
		return err
	}
	logrus.Debugf("source image %v, targetImage %v, exists %v", sourceImage, exists)
	if !exists {
		hubUser, hubPass := builder.GetImageUserInfoV2(sourceImage, username, password)
		if _, err := ImagePull(containerdClient, targetImage, hubUser, hubPass, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("pull image %s failed %v", targetImage, err), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if err := ImageTag(containerdClient, targetImage, sourceImage, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("change image tag %s to %s failed", targetImage, sourceImage), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if err := ImagePush(containerdClient, sourceImage, hubUser, hubPass, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("push image %s failed %v", sourceImage, err), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
	}
	return nil
}

func printLog(logger event.Logger, level, msg string, info map[string]string) {
	switch level {
	case "info":
		if logger != nil {
			logger.Info(msg, info)
		} else {
			logrus.Info(msg)
		}
	case "debug":
		if logger != nil {
			logger.Debug(msg, info)
		} else {
			logrus.Debug(msg)
		}
	case "error":
		if logger != nil {
			logger.Error(msg, info)
		} else {
			logrus.Error(msg)
		}
	}
}

func CreateImageName(ServiceID, DeployVersion string) string {
	imageName := strings.ToLower(fmt.Sprintf("%s/%s:%s", builder.REGISTRYDOMAIN, ServiceID, DeployVersion))
	logrus.Info("imageName:", imageName)
	component, err := db.GetManager().TenantServiceDao().GetServiceByID(ServiceID)
	if err != nil {
		logrus.Errorf("image build get service by id error: %v", err)
		return imageName
	}
	app, err := db.GetManager().ApplicationDao().GetByServiceID(ServiceID)
	if err != nil {
		logrus.Errorf("image build get app by serviceid error: %v", err)
		return imageName
	}
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(component.TenantID)
	if err != nil {
		logrus.Errorf("image build get tenant by uuid error: %v", err)
		return imageName
	}
	workloadName := fmt.Sprintf("%s-%s-%s", tenant.Namespace, app.K8sApp, component.K8sComponentName)
	return strings.ToLower(fmt.Sprintf("%s/%s:%s", builder.REGISTRYDOMAIN, workloadName, DeployVersion))
}

func CreateVolumesAndMounts(contextDir, buildType string) (volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) {
	pathSplit := strings.Split(contextDir, "/")
	subPath := strings.Join(pathSplit[2:], "/")
	hostPathType := corev1.HostPathDirectoryOrCreate
	hostsFilePathType := corev1.HostPathFile
	// kaniko volumes volumeMounts config
	volumes = []corev1.Volume{
		{
			Name: "kaniko-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "rbd-hub-credentials",
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
	volumeMounts = []corev1.VolumeMount{
		{
			Name:      "kaniko-secret",
			MountPath: "/kaniko/.docker",
		},
		{
			Name:      "hosts",
			MountPath: "/etc/hosts",
		},
	}
	// Customize it according to how it is built volumes volumeMounts config
	if buildType == "plug-build" {
		volume := corev1.Volume{
			Name: "plug-build",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: contextDir,
					Type: &hostPathType,
				},
			},
		}
		volumes = append(volumes, volume)
		volumeMount := corev1.VolumeMount{
			Name:      "plug-build",
			MountPath: "/workspace",
		}
		volumeMounts = append(volumeMounts, volumeMount)
	}
	if buildType == "run-build" {
		volume := corev1.Volume{
			Name: "run-build",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "rbd-cpt-grdata",
				},
			},
		}
		volumes = append(volumes, volume)
		volumeMount := corev1.VolumeMount{
			Name:      "run-build",
			MountPath: "/workspace",
			SubPath:   subPath,
		}
		volumeMounts = append(volumeMounts, volumeMount)
	}
	return volumes, volumeMounts
}

func WaitingComplete(reChan *channels.RingChannel) (err error) {
	var logComplete = false
	var jobComplete = false
	timeOut := time.NewTimer(time.Minute * 60)
	for {
		select {
		case <-timeOut.C:
			return fmt.Errorf("build time out (more than 60 minute)")
		case jobStatus := <-reChan.Out():
			status := jobStatus.(string)
			switch status {
			case "complete":
				jobComplete = true
				if logComplete {
					return nil
				}
				logrus.Info(util.Translation("build code job exec completed"), map[string]string{"step": "build-exector"})
			case "failed":
				jobComplete = true
				err = fmt.Errorf("build code job exec failure")
				if logComplete {
					return err
				}
				logrus.Info(util.Translation("build code job exec failed"), map[string]string{"step": "build-exector"})
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
