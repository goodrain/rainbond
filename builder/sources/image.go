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
	ctrcontent "github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/containerd/platforms"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/remotes"
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
	"github.com/goodrain/rainbond/util/constants"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"io"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

// ErrorNoAuth error no auth
var ErrorNoAuth = fmt.Errorf("pull image require docker login")

// ErrorNoImage error no image
var ErrorNoImage = fmt.Errorf("image not exist")

// Namespace containerd image namespace
var Namespace = "k8s.io"

// ImagePull pull docker image
// Deprecated: use sources.ImageClient.ImagePull instead
func ImagePull(client *containerd.Client, ref string, username, password string, logger event.Logger, timeout int) (*containerd.Image, error) {
	printLog(logger, "info", fmt.Sprintf("start get image:%s", ref), map[string]string{"step": "pullimage"})
	srcNamed, err := refdocker.ParseDockerRef(ref)
	if err != nil {
		return nil, err
	}
	image := srcNamed.String()
	ongoing := ctrcontent.NewJobs(image)
	pctx, stopProgress := context.WithCancel(namespaces.WithNamespace(context.Background(), Namespace))
	progress := make(chan struct{})
	go func() {
		ctrcontent.ShowProgress(pctx, ongoing, client.ContentStore(), os.Stdout)
		close(progress)
	}()
	h := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			ongoing.Add(desc)
		}
		return nil, nil
	})
	defaultTLS := &tls.Config{
		InsecureSkipVerify: true,
	}
	hostOpt := config.HostOptions{}
	hostOpt.DefaultTLS = defaultTLS
	hostOpt.Credentials = func(host string) (string, string, error) {
		return username, password, nil
	}
	Tracker := docker.NewInMemoryTracker()
	options := docker.ResolverOptions{
		Tracker: Tracker,
		Hosts:   config.ConfigureHosts(pctx, hostOpt),
	}

	platformMC := platforms.Ordered([]ocispec.Platform{platforms.DefaultSpec()}...)
	opts := []containerd.RemoteOpt{
		containerd.WithImageHandler(h),
		//nolint:staticcheck
		containerd.WithSchema1Conversion, //lint:ignore SA1019 nerdctl should support schema1 as well.
		containerd.WithPlatformMatcher(platformMC),
		containerd.WithResolver(docker.NewResolver(options)),
	}
	var img containerd.Image
	img, err = client.Pull(pctx, image, opts...)
	stopProgress()
	if err != nil {
		// 增强错误处理，提供更明确的错误信息
		enhancedErr := enhanceImagePullErrorLegacy(err, image, logger)
		return nil, enhancedErr
	}
	<-progress
	printLog(logger, "info", fmt.Sprintf("Success Pull Image：%s", image), map[string]string{"step": "pullimage"})
	return &img, nil
}

// enhanceImagePullErrorLegacy 为旧版本ImagePull函数增强错误处理
func enhanceImagePullErrorLegacy(err error, image string, logger event.Logger) error {
	errMsg := err.Error()
	var userFriendlyMsg string
	var logMsg string
	var adviceMsg string

	// 检查是否是goodrain.me相关的错误
	isGoodrainRepo := strings.Contains(image, "goodrain.me")

	switch {
	case strings.Contains(errMsg, "EOF"):
		if isGoodrainRepo {
			userFriendlyMsg = "连接goodrain.me镜像仓库时连接被意外中断"
			logMsg = fmt.Sprintf("Pull image %s failed: connection terminated unexpectedly (EOF)", image)
			adviceMsg = "请检查: 1) 网络连接是否稳定; 2) goodrain.me服务是否可访问; 3) 是否需要配置代理或DNS; 4) 建议更换镜像仓库地址"
		} else {
			userFriendlyMsg = "镜像仓库连接被意外中断"
			logMsg = fmt.Sprintf("Pull image %s failed: connection terminated unexpectedly (EOF)", image)
			adviceMsg = "请检查网络连接和镜像仓库服务状态"
		}

	case strings.Contains(errMsg, "context deadline exceeded") || strings.Contains(errMsg, "timeout"):
		if isGoodrainRepo {
			userFriendlyMsg = "连接goodrain.me镜像仓库超时"
			logMsg = fmt.Sprintf("Pull image %s failed: connection timeout", image)
			adviceMsg = "请检查: 1) 网络连接速度; 2) goodrain.me的可访问性; 3) 防火墙设置; 4) 建议更换为国内镜像仓库"
		} else {
			userFriendlyMsg = "镜像拉取超时"
			logMsg = fmt.Sprintf("Pull image %s failed: connection timeout", image)
			adviceMsg = "请检查网络连接速度和镜像仓库可访问性"
		}

	case strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "Name or service not known"):
		if isGoodrainRepo {
			userFriendlyMsg = "无法解析goodrain.me域名"
			logMsg = fmt.Sprintf("Pull image %s failed: DNS resolution failed for goodrain.me", image)
			adviceMsg = "请检查: 1) DNS配置是否正确; 2) 网络连接是否正常; 3) goodrain.me域名是否可访问; 4) 建议更换镜像仓库地址"
		} else {
			userFriendlyMsg = "无法解析镜像仓库域名"
			logMsg = fmt.Sprintf("Pull image %s failed: DNS resolution failed", image)
			adviceMsg = "请检查DNS配置和网络连接"
		}

	case strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "Connection refused"):
		if isGoodrainRepo {
			userFriendlyMsg = "goodrain.me镜像仓库拒绝连接"
			logMsg = fmt.Sprintf("Pull image %s failed: connection refused by goodrain.me", image)
			adviceMsg = "请检查: 1) goodrain.me服务状态; 2) 网络连接; 3) 端口是否被阻止; 4) 建议更换镜像仓库地址"
		} else {
			userFriendlyMsg = "镜像仓库拒绝连接"
			logMsg = fmt.Sprintf("Pull image %s failed: connection refused", image)
			adviceMsg = "请检查镜像仓库服务状态和网络连接"
		}

	case strings.Contains(errMsg, "network is unreachable") || strings.Contains(errMsg, "Network is unreachable"):
		userFriendlyMsg = "网络不可达，无法连接到镜像仓库"
		logMsg = fmt.Sprintf("Pull image %s failed: network unreachable", image)
		adviceMsg = "请检查网络连接配置和路由设置"

	case strings.Contains(errMsg, "certificate verify failed") || strings.Contains(errMsg, "x509"):
		userFriendlyMsg = "镜像仓库SSL证书验证失败"
		logMsg = fmt.Sprintf("Pull image %s failed: SSL certificate verification failed", image)
		adviceMsg = "请检查镜像仓库SSL证书是否有效，或尝试使用HTTP协议"

	case strings.Contains(errMsg, "authentication failed") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "unauthorized"):
		userFriendlyMsg = "镜像仓库身份验证失败"
		logMsg = fmt.Sprintf("Pull image %s failed: authentication failed", image)
		adviceMsg = "请检查用户名密码或访问令牌是否正确"

	case strings.Contains(errMsg, "403") || strings.Contains(errMsg, "Forbidden"):
		userFriendlyMsg = "没有访问该镜像的权限"
		logMsg = fmt.Sprintf("Pull image %s failed: access forbidden", image)
		adviceMsg = "请检查是否有访问该镜像仓库的权限"

	case strings.Contains(errMsg, "404") || strings.Contains(errMsg, "not found"):
		userFriendlyMsg = "镜像不存在"
		logMsg = fmt.Sprintf("Pull image %s failed: image not found", image)
		adviceMsg = "请检查镜像名称和标签是否正确"

	case strings.Contains(errMsg, "proxyconnect") || strings.Contains(errMsg, "proxy"):
		userFriendlyMsg = "代理连接失败"
		logMsg = fmt.Sprintf("Pull image %s failed: proxy connection failed", image)
		adviceMsg = "请检查代理设置是否正确"

	default:
		if isGoodrainRepo {
			userFriendlyMsg = "从goodrain.me拉取镜像失败"
			logMsg = fmt.Sprintf("Pull image %s failed: %s", image, errMsg)
			adviceMsg = "建议更换镜像仓库地址，或检查goodrain.me的服务状态"
		} else {
			userFriendlyMsg = "镜像拉取失败"
			logMsg = fmt.Sprintf("Pull image %s failed: %s", image, errMsg)
			adviceMsg = "请检查镜像名称和网络连接"
		}
	}

	// 记录详细的错误日志
	logrus.Errorf("[ImagePull] %s", logMsg)
	printLog(logger, "error", fmt.Sprintf("%s: %s", userFriendlyMsg, adviceMsg), map[string]string{"step": "pullimage", "status": "failure"})

	// 返回用户友好的错误信息
	return fmt.Errorf("%s。%s。原始错误: %s", userFriendlyMsg, adviceMsg, errMsg)
}

// ImageTag -
// Deprecated: use sources.ImageClient.ImagePull instead
func ImageTag(containerdClient *containerd.Client, source, target string, logger event.Logger, timeout int) error {
	srcNamed, err := refdocker.ParseDockerRef(source)
	if err != nil {
		return err
	}
	srcImage := srcNamed.String()
	targetNamed, err := refdocker.ParseDockerRef(target)
	if err != nil {
		return err
	}
	targetImage := targetNamed.String()
	logrus.Infof(fmt.Sprintf("change image tag：%s -> %s", srcImage, targetImage))
	printLog(logger, "info", fmt.Sprintf("change image tag：%s -> %s", source, target), map[string]string{"step": "changetag"})
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	imageService := containerdClient.ImageService()
	image, err := imageService.Get(ctx, srcImage)
	if err != nil {
		logrus.Errorf("imagetag imageService Get error: %s", err.Error())
		return err
	}
	image.Name = targetImage
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

// ImageNameHandle 解析imagename
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

// ImageNameWithNamespaceHandle if have namespace,will parse namespace
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

// ImagePush push image to registry
// timeout minutes of the unit
// Deprecated: use sources.ImageClient.ImagePush instead
func ImagePush(client *containerd.Client, rawRef, user, pass string, logger event.Logger, timeout int) error {
	printLog(logger, "info", fmt.Sprintf("start push image：%s", rawRef), map[string]string{"step": "pushimage"})
	named, err := refdocker.ParseDockerRef(rawRef)
	if err != nil {
		return err
	}
	image := named.String()
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	img, err := client.ImageService().Get(ctx, image)
	if err != nil {
		return errors.Wrap(err, "unable to resolve image to manifest")
	}
	desc := img.Target
	cs := client.ContentStore()
	if manifests, err := images.Children(ctx, cs, desc); err == nil && len(manifests) > 0 {
		matcher := platforms.NewMatcher(platforms.DefaultSpec())
		for _, manifest := range manifests {
			if manifest.Platform != nil && matcher.Match(*manifest.Platform) {
				if _, err := images.Children(ctx, cs, manifest); err != nil {
					return errors.Wrap(err, "no matching manifest")
				}
				desc = manifest
				break
			}
		}
	}
	NewTracker := docker.NewInMemoryTracker()
	options := docker.ResolverOptions{
		Tracker: NewTracker,
	}
	hostOptions := config.HostOptions{
		DefaultTLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	hostOptions.Credentials = func(host string) (string, string, error) {
		return user, pass, nil
	}
	options.Hosts = config.ConfigureHosts(ctx, hostOptions)
	resolver := docker.NewResolver(options)
	ongoing := newPushJobs(NewTracker)

	eg, ctx := errgroup.WithContext(ctx)
	// used to notify the progress writer
	doneCh := make(chan struct{})
	eg.Go(func() error {
		defer close(doneCh)
		jobHandler := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			ongoing.add(remotes.MakeRefKey(ctx, desc))
			return nil, nil
		})

		ropts := []containerd.RemoteOpt{
			containerd.WithResolver(resolver),
			containerd.WithImageHandler(jobHandler),
		}
		return client.Push(ctx, image, desc, ropts...)
	})

	eg.Go(func() error {
		var (
			ticker = time.NewTicker(100 * time.Millisecond)
			fw     = progress.NewWriter(os.Stdout)
			start  = time.Now()
			done   bool
		)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fw.Flush()
				tw := tabwriter.NewWriter(fw, 1, 8, 1, ' ', 0)
				ctrcontent.Display(tw, ongoing.status(), start)
				tw.Flush()
				if done {
					fw.Flush()
					return nil
				}
			case <-doneCh:
				done = true
			case <-ctx.Done():
				done = true // allow ui to update once more
			}
		}
	})
	// create a container
	printLog(logger, "info", fmt.Sprintf("success push image：%s", image), map[string]string{"step": "pushimage"})
	return nil
}

// TrustedImagePush push image to trusted registry
func TrustedImagePush(containerdClient *containerd.Client, image, user, pass string, logger event.Logger, timeout int) error {
	if err := CheckTrustedRepositories(image, user, pass); err != nil {
		return err
	}
	return ImagePush(containerdClient, image, user, pass, logger, timeout)
}

// CheckTrustedRepositories check Repositories is exist ,if not create it.
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

func getHostAlias(kubeClient kubernetes.Interface) []corev1.HostAlias {
	var hostAliases []corev1.HostAlias
	ds, err := kubeClient.AppsV1().DaemonSets(util.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).Get(context.Background(), "rbd-chaos", metav1.GetOptions{})
	if err != nil {
		logrus.Debugf("get hostAliases from daemonset rbd-chaos error: %s", err.Error())
		return hostAliases
	}
	for _, host := range ds.Spec.Template.Spec.HostAliases {
		hostAliases = append(hostAliases, host)
	}
	return hostAliases
}

// ImageBuild use buildkit build image
func ImageBuild(arch, contextDir, RbdNamespace, ServiceID, DeployVersion string, logger event.Logger, buildType, plugImageName, BuildKitImage string, BuildKitArgs []string, BuildKitCache bool, kubeClient kubernetes.Interface) error {
	// create image name
	var buildImageName string
	if buildType == "plug-build" || buildType == "vm-build" {
		buildImageName = plugImageName
	} else {
		buildImageName = CreateImageName(ServiceID, DeployVersion)
	}
	// The same component retains only one build task to perform
	jobList, err := jobc.GetJobController().GetServiceJobs(ServiceID)
	if err != nil {
		logrus.Errorf("get pre build job for service %s failure ,%s", ServiceID, err.Error())
	}
	name := fmt.Sprintf("%s-%s-dockerfile", ServiceID, DeployVersion)
	for _, job := range jobList {
		if job.Name == name {
			jobc.GetJobController().DeleteJob(job.Name)
		}
	}
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
	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyOnFailure,
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{arch},
							},
							{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{os.Getenv("HOST_IP")},
							},
						},
					},
					},
				},
			},
		},
		HostAliases: getHostAlias(kubeClient),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	imageDomain, buildKitTomlCMName := GetImageFirstPart(builder.REGISTRYDOMAIN)
	err = PrepareBuildKitTomlCM(ctx, kubeClient, RbdNamespace, buildKitTomlCMName, imageDomain)
	if err != nil {
		return err
	}
	// only support never and onfailure
	volumes, volumeMounts := CreateVolumeAndMounts(contextDir, buildType, buildKitTomlCMName)
	podSpec.Volumes = volumes
	privileged := true
	// container config
	container := corev1.Container{
		Name:      name,
		Image:     BuildKitImage,
		Stdin:     true,
		StdinOnce: true,
		Command:   []string{"buildctl-daemonless.sh"},
		Env: []corev1.EnvVar{{
			Name:  "BUILDCTL_CONNECT_RETRIES_MAX",
			Value: "20",
		},
		},
		Args: []string{
			"build",
			"--frontend",
			"dockerfile.v0",
			"--local",
			"context=/workspace",
			"--local",
			"dockerfile=/workspace",
			"--output",
			fmt.Sprintf("type=image,name=%s,push=true", buildImageName),
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
		},
	}
	logrus.Infof("buildkt args: %v", BuildKitArgs)
	if len(BuildKitArgs) > 0 {
		container.Args = append(container.Args, BuildKitArgs...)
	}
	container.VolumeMounts = volumeMounts
	podSpec.Containers = append(podSpec.Containers, container)
	job.Spec = podSpec
	writer := logger.GetWriter("builder", "info")
	reChan := channels.NewRingChannel(10)
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

// ImageInspectWithRaw get image inspect
func ImageInspectWithRaw(dockerCli *client.Client, image string) (*types.ImageInspect, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ins, _, err := dockerCli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

// ImageSave save image to tar file
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

// MultiImageSave save multi image to tar file
// destination destination file name eg. /tmp/xxx.tar
func MultiImageSave(ctx context.Context, dockerCli *client.Client, destination string, logger event.Logger, images ...string) error {
	rc, err := dockerCli.ImageSave(ctx, images)
	if err != nil {
		return err
	}
	defer rc.Close()
	return CopyToFile(destination, rc)
}

// ImageLoad load image from  tar file
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

// ImageImport save image to tar file
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

// ImageRemove remove image
func ImageRemove(containerdClient *containerd.Client, image string) error {
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
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

	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	imageSummarys, err := containerdClient.ListImages(ctx, imageFullName)
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
// Deprecated: ImagesPullAndPush
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
	logrus.Debugf("source image %v, targetImage %v, exists %v", sourceImage, targetImage, exists)
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

// CreateImageName -
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

// GetImageFirstPart -
func GetImageFirstPart(str string) (string, string) {
	imageDomain, imageName := str, ""
	if strings.Contains(str, "/") {
		parts := strings.Split(str, "/")
		imageDomain = parts[0]
	}
	imageName = strings.Replace(imageDomain, ".", "-", -1)
	imageName = strings.Replace(imageName, ":", "-", -1)
	return imageDomain, imageName
}

// PrepareBuildKitTomlCM -
func PrepareBuildKitTomlCM(ctx context.Context, kubeClient kubernetes.Interface, namespace, buildKitTomlCMName, imageDomain string) error {
	buildKitTomlCM, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, buildKitTomlCMName, metav1.GetOptions{})
	if err != nil && !k8serror.IsNotFound(err) {
		return err
	}
	if k8serror.IsNotFound(err) {
		configStr := fmt.Sprintf("debug = true\n[registry.\"%v\"]\n  http = false\n  insecure = true", imageDomain)
		buildKitTomlCM = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: buildKitTomlCMName,
			},
			Data: map[string]string{
				"buildkittoml": configStr,
			},
		}
		_, err = kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, buildKitTomlCM, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "create buildkittoml cm failure")
		}
	}
	return nil
}

// CreateVolumeAndMounts -
func CreateVolumeAndMounts(contextDir, buildType string, buildKitTomlCMName string) (volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) {
	hostPathType := corev1.HostPathDirectoryOrCreate
	hostsFilePathType := corev1.HostPathFile
	// buildkit volumes volumeMounts config
	volumes = []corev1.Volume{
		{
			Name: "buildkit-secret",
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
		{
			Name: "buildkittoml",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: buildKitTomlCMName},
					Items: []corev1.KeyToPath{
						{
							Key:  "buildkittoml",
							Path: "buildkitd.toml",
						},
					},
				},
			},
		},
	}
	volumeMounts = []corev1.VolumeMount{
		{
			Name:      "buildkit-secret",
			MountPath: "/root/.docker",
		},
		{
			Name:      "hosts",
			MountPath: "/etc/hosts",
		},
		{
			Name:      "buildkittoml",
			MountPath: "/etc/buildkit",
		},
	}
	volume := corev1.Volume{
		Name: "plug-build",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path.Join("/opt/rainbond", contextDir),
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
	return volumes, volumeMounts
}

// WaitingComplete -
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

type pushjobs struct {
	jobs    map[string]struct{}
	ordered []string
	tracker docker.StatusTracker
	mu      sync.Mutex
}

func newPushJobs(tracker docker.StatusTracker) *pushjobs {
	return &pushjobs{
		jobs:    make(map[string]struct{}),
		tracker: tracker,
	}
}

func (j *pushjobs) add(ref string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.jobs[ref]; ok {
		return
	}
	j.ordered = append(j.ordered, ref)
	j.jobs[ref] = struct{}{}
}

func (j *pushjobs) status() []ctrcontent.StatusInfo {
	j.mu.Lock()
	defer j.mu.Unlock()

	statuses := make([]ctrcontent.StatusInfo, 0, len(j.jobs))
	for _, name := range j.ordered {
		si := ctrcontent.StatusInfo{
			Ref: name,
		}

		status, err := j.tracker.GetStatus(name)
		if err != nil {
			si.Status = "waiting"
		} else {
			si.Offset = status.Offset
			si.Total = status.Total
			si.StartedAt = status.StartedAt
			si.UpdatedAt = status.UpdatedAt
			if status.Offset >= status.Total {
				if status.UploadUUID == "" {
					si.Status = "done"
				} else {
					si.Status = "committing"
				}
			} else {
				si.Status = "uploading"
			}
		}
		statuses = append(statuses, si)
	}

	return statuses
}
