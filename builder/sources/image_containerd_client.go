package sources

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	ctrcontent "github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/containerd/platforms"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util/criutil"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"os"
	"strings"
	"sync"
	"time"
)

type containerdImageCliFactory struct{}

func (f containerdImageCliFactory) NewClient(endpoint string, timeout time.Duration) (ImageClient, error) {
	var (
		containerdCli *containerd.Client
		imageClient   runtimeapi.ImageServiceClient
		grpcConn      *grpc.ClientConn
		err           error
	)
	imageClient, grpcConn, err = criutil.GetImageClient(context.Background(), endpoint, time.Second*3)
	if err != nil {
		return nil, err
	}
	if os.Getenv("CONTAINERD_SOCK") != "" {
		endpoint = os.Getenv("CONTAINERD_SOCK")
	}
	containerdCli, err = containerd.New(endpoint, containerd.WithTimeout(timeout))
	if err != nil {
		return nil, err
	}
	return &containerdImageCliImpl{
		client:      containerdCli,
		conn:        grpcConn,
		imageClient: imageClient,
	}, nil
}

type containerdImageCliImpl struct {
	client      *containerd.Client
	conn        *grpc.ClientConn
	imageClient runtimeapi.ImageServiceClient
}

func (c *containerdImageCliImpl) CheckIfImageExists(imageName string) (imageRef string, exists bool, err error) {
	named, err := refdocker.ParseDockerRef(imageName)
	if err != nil {
		return "", false, fmt.Errorf("parse image %s: %v", imageName, err)
	}
	imageFullName := named.String()
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	containers, err := c.imageClient.ListImages(ctx, &runtimeapi.ListImagesRequest{})
	if err != nil {
		return imageFullName, false, err
	}
	if len(containers.GetImages()) > 0 {
		for _, image := range containers.GetImages() {
			for _, repoTag := range image.GetRepoTags() {
				if repoTag == imageFullName {
					return imageFullName, true, nil
				}
			}
		}
	}
	return imageFullName, false, nil
}

func (c *containerdImageCliImpl) GetContainerdClient() *containerd.Client {
	return c.client
}

func (c *containerdImageCliImpl) GetDockerClient() *dockercli.Client {
	return nil
}

func (c *containerdImageCliImpl) ImagePull(image string, username, password string, logger event.Logger, timeout int) (*ocispec.ImageConfig, error) {
	printLog(logger, "info", fmt.Sprintf("start get image:%s", image), map[string]string{"step": "pullimage"})
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return nil, err
	}
	reference := named.String()
	ongoing := NewJobs(reference)
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	pctx, stopProgress := context.WithCancel(ctx)
	progress := make(chan struct{})

	go func() {
		ShowProgress(pctx, ongoing, c.client.ContentStore(), logger)
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
	img, err = c.client.Pull(pctx, reference, opts...)
	if err != nil {
		logrus.Infof("pull image failure with HTTPS, try HTTP protocol")
		printLog(logger, "warn", fmt.Sprintf("HTTPS镜像拉取失败，尝试HTTP协议: %s", err.Error()), map[string]string{"step": "pullimage"})
		hostOpt.DefaultScheme = "http"
		options := docker.ResolverOptions{
			Tracker: Tracker,
			Hosts:   config.ConfigureHosts(pctx, hostOpt),
		}
		opts = []containerd.RemoteOpt{
			containerd.WithImageHandler(h),
			//nolint:staticcheck
			containerd.WithSchema1Conversion, //lint:ignore SA1019 nerdctl should support schema1 as well.
			containerd.WithPlatformMatcher(platformMC),
			containerd.WithResolver(docker.NewResolver(options)),
		}
		img, err = c.client.Pull(pctx, reference, opts...)
	}
	stopProgress()
	if err != nil {
		// 增强错误处理，提供更详细的错误信息
		enhancedErr := c.enhanceImagePullError(err, reference, logger)
		return nil, enhancedErr
	}
	<-progress
	printLog(logger, "info", fmt.Sprintf("Success Pull Image：%s", reference), map[string]string{"step": "pullimage"})
	return getImageConfig(ctx, img)
}

// enhanceImagePullError 增强镜像拉取错误信息，提供更详细的错误描述和解决建议
func (c *containerdImageCliImpl) enhanceImagePullError(err error, image string, logger event.Logger) error {
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

// Helper function to check if the error is protocol related
func isProtocolError(err error) bool {
	return strings.Contains(err.Error(), "invalid protocol") || strings.Contains(err.Error(), "unsupported protocol scheme")
}

func getImageConfig(ctx context.Context, image containerd.Image) (*ocispec.ImageConfig, error) {
	desc, err := image.Config(ctx)
	if err != nil {
		return nil, err
	}
	switch desc.MediaType {
	case ocispec.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
		var ocispecImage ocispec.Image
		b, err := content.ReadBlob(ctx, image.ContentStore(), desc)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(b, &ocispecImage); err != nil {
			return nil, err
		}
		return &ocispecImage.Config, nil
	default:
		return nil, fmt.Errorf("unknown media type %q", desc.MediaType)
	}
}

func (c *containerdImageCliImpl) ImagePush(image, user, pass string, logger event.Logger, timeout int) error {
	printLog(logger, "info", fmt.Sprintf("开始推送镜像：%s", image), map[string]string{"step": "pushimage"})

	hostOptions := config.HostOptions{
		DefaultTLS: &tls.Config{
			InsecureSkipVerify: true,
		},
		Credentials: func(host string) (string, string, error) {
			return user, pass, nil
		},
	}

	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return errors.Wrap(err, "解析镜像引用失败")
	}
	reference := named.String()
	ctx := namespaces.WithNamespace(context.Background(), Namespace)

	// 获取镜像并进行验证
	img, err := c.client.ImageService().Get(ctx, reference)
	if err != nil {
		return errors.Wrap(err, "无法获取镜像清单")
	}
	desc := img.Target
	cs := c.client.ContentStore()

	// 获取子镜像清单
	manifests, err := images.Children(ctx, cs, desc)
	if err != nil {
		return errors.Wrap(err, "获取镜像子清单失败")
	}

	matcher := platforms.NewMatcher(platforms.DefaultSpec())
	for _, manifest := range manifests {
		if manifest.Platform != nil && matcher.Match(*manifest.Platform) {
			desc = manifest
			break
		}
	}

	NewTracker := docker.NewInMemoryTracker()
	options := docker.ResolverOptions{
		Tracker: NewTracker,
	}

	ongoing := newPushJobs(NewTracker)

	var pushFunc = func(scheme string) error {
		// 配置指定协议
		hostOptions.DefaultScheme = scheme
		options.Hosts = config.ConfigureHosts(ctx, hostOptions)
		resolver := docker.NewResolver(options)

		// 镜像推送任务
		eg, ctx := errgroup.WithContext(ctx)
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

			// 尝试多次推送，确保清理缓存后重新拉取并推送
			for attempts := 0; attempts < 3; attempts++ {
				// 从 image 中提取仓库名称，使用 Name() 获取
				repository := named.Name() // 使用 .Name() 获取仓库路径
				// 使用 WithScope 设置作用域，采用 repository:镜像名称:操作类型 格式
				scope := fmt.Sprintf("repository:%s:%s", repository, "push") // 构造作用域
				ctx := namespaces.WithNamespace(context.Background(), Namespace)
				ctx = docker.WithScope(ctx, scope)
				if err := c.client.Push(ctx, reference, desc, ropts...); err != nil {
					if attempts < 2 {
						printLog(logger, "warn", fmt.Sprintf("推送失败，重试中... (%d/3)", attempts+1), map[string]string{"step": "pushimage"})
						_ = c.client.ImageService().Delete(ctx, reference) // 清理缓存镜像
						time.Sleep(5 * time.Second)
						continue
					}
					return errors.Wrap(err, "推送过程中发生错误")
				}
				break
			}
			return nil
		})

		// 进度显示任务
		eg.Go(func() error {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			start := time.Now()
			done := false

			for {
				select {
				case <-ticker.C:
					Display(ongoing.status(), start, logger)
					if done {
						return nil
					}
				case <-doneCh:
					done = true
				case <-ctx.Done():
					done = true
				}
			}
		})

		// 等待所有 goroutine 完成
		return eg.Wait()
	}

	// 先尝试 HTTPS
	err = pushFunc("https")
	if err != nil {
		// 如果是协议相关错误，尝试降级为 HTTP
		printLog(logger, "warn", fmt.Sprintf("HTTPS 推送失败，尝试使用 HTTP：%s", err.Error()), map[string]string{"step": "pushimage"})
		err = pushFunc("http")
	}

	if err != nil {
		return errors.Wrap(err, "推送失败")
	}

	printLog(logger, "info", fmt.Sprintf("成功推送镜像：%s", reference), map[string]string{"step": "pushimage"})
	return nil
}

// ImageTag change docker image tag
func (c *containerdImageCliImpl) ImageTag(source, target string, logger event.Logger, timeout int) error {
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
	imageService := c.client.ImageService()
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

// ImagesPullAndPush Used to process mirroring of non local components, example: builder, runner, /rbd-mesh-data-panel
func (c *containerdImageCliImpl) ImagesPullAndPush(sourceImage, targetImage, username, password string, logger event.Logger) error {
	sourceImage, exists, err := c.CheckIfImageExists(sourceImage)
	if err != nil {
		logrus.Errorf("failed to check whether the builder mirror exists: %s", err.Error())
		return err
	}
	logrus.Infof("source image %v, targetImage %v, exists %v", sourceImage, targetImage, exists)
	if !exists {
		hubUser, hubPass := builder.GetImageUserInfoV2(sourceImage, username, password)
		if _, err := c.ImagePull(targetImage, username, password, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("pull image %s failed %v", targetImage, err), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if err := c.ImageTag(targetImage, sourceImage, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("change image tag %s to %s failed", targetImage, sourceImage), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if err := c.ImagePush(sourceImage, hubUser, hubPass, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("push image %s failed %v", sourceImage, err), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
	}
	return nil
}

// ImageRemove remove image
func (c *containerdImageCliImpl) ImageRemove(image string) error {
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return err
	}
	reference := named.String()
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	imageStore := c.client.ImageService()
	err = imageStore.Delete(ctx, reference)
	if err != nil {
		logrus.Errorf("image remove ")
	}
	return err
}

// ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func (c *containerdImageCliImpl) ImageSave(image, destination string) error {
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return err
	}
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	var exportOpts = []archive.ExportOpt{archive.WithImage(c.client.ImageService(), named.String())}
	w, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer w.Close()
	return c.client.Export(ctx, w, exportOpts...)
}

// TrustedImagePush push image to trusted registry
func (c *containerdImageCliImpl) TrustedImagePush(image, user, pass string, logger event.Logger, timeout int) error {
	if err := CheckTrustedRepositories(image, user, pass); err != nil {
		return err
	}
	return c.ImagePush(image, user, pass, logger, timeout)
}

// ImageLoad load image from  tar file
// destination destination file name eg. /tmp/xxx.tar
func (c *containerdImageCliImpl) ImageLoad(tarFile string, logger event.Logger) ([]string, error) {
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	reader, err := os.OpenFile(tarFile, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	imgs, err := c.client.Import(ctx, reader)
	if err != nil {
		return nil, err
	}
	var imageNames []string
	for _, img := range imgs {
		imageNames = append(imageNames, img.Name)
	}
	return imageNames, nil
}

// ShowProgress continuously updates the output with job progress
// by checking status in the content store.
func ShowProgress(ctx context.Context, ongoing *Jobs, cs content.Store, logger event.Logger) {
	var (
		ticker   = time.NewTicker(100 * time.Millisecond)
		start    = time.Now()
		statuses = map[string]ctrcontent.StatusInfo{}
		done     bool
	)
	defer ticker.Stop()
outer:
	for {
		select {
		case <-ticker.C:
			resolved := "resolved"
			if !ongoing.IsResolved() {
				resolved = "resolving"
			}
			statuses[ongoing.name] = ctrcontent.StatusInfo{
				Ref:    ongoing.name,
				Status: resolved,
			}
			keys := []string{ongoing.name}

			activeSeen := map[string]struct{}{}
			if !done {
				actives, err := cs.ListStatuses(ctx, "")
				if err != nil {
					log.G(ctx).WithError(err).Error("active check failed")
					continue
				}
				// update status of active entries!
				for _, active := range actives {
					statuses[active.Ref] = ctrcontent.StatusInfo{
						Ref:       active.Ref,
						Status:    "downloading",
						Offset:    active.Offset,
						Total:     active.Total,
						StartedAt: active.StartedAt,
						UpdatedAt: active.UpdatedAt,
					}
					activeSeen[active.Ref] = struct{}{}
				}
			}

			// now, update the items in jobs that are not in active
			for _, j := range ongoing.Jobs() {
				key := remotes.MakeRefKey(ctx, j)
				keys = append(keys, key)
				if _, ok := activeSeen[key]; ok {
					continue
				}

				status, ok := statuses[key]
				if !done && (!ok || status.Status == "downloading") {
					info, err := cs.Info(ctx, j.Digest)
					if err != nil {
						if !errdefs.IsNotFound(err) {
							log.G(ctx).WithError(err).Errorf("failed to get content info")
							continue outer
						} else {
							statuses[key] = ctrcontent.StatusInfo{
								Ref:    key,
								Status: "waiting",
							}
						}
					} else if info.CreatedAt.After(start) {
						statuses[key] = ctrcontent.StatusInfo{
							Ref:       key,
							Status:    "done",
							Offset:    info.Size,
							Total:     info.Size,
							UpdatedAt: info.CreatedAt,
						}
					} else {
						statuses[key] = ctrcontent.StatusInfo{
							Ref:    key,
							Status: "exists",
						}
					}
				} else if done {
					if ok {
						if status.Status != "done" && status.Status != "exists" {
							status.Status = "done"
							statuses[key] = status
						}
					} else {
						statuses[key] = ctrcontent.StatusInfo{
							Ref:    key,
							Status: "done",
						}
					}
				}
			}
			var ordered []ctrcontent.StatusInfo
			for _, key := range keys {
				ordered = append(ordered, statuses[key])
			}

			Display(ordered, start, logger)

			if done {
				//tt.Flush()
				return
			}
		case <-ctx.Done():
			done = true // allow ui to update once more
		}
	}
}

// Jobs provides a way of identifying the download keys for a particular task
// encountering during the pull walk.
//
// This is very minimal and will probably be replaced with something more
// featured.
type Jobs struct {
	name     string
	added    map[digest.Digest]struct{}
	descs    []ocispec.Descriptor
	mu       sync.Mutex
	resolved bool
}

// NewJobs creates a new instance of the job status tracker
func NewJobs(name string) *Jobs {
	return &Jobs{
		name:  name,
		added: map[digest.Digest]struct{}{},
	}
}

// Add adds a descriptor to be tracked
func (j *Jobs) Add(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.resolved = true

	if _, ok := j.added[desc.Digest]; ok {
		return
	}
	j.descs = append(j.descs, desc)
	j.added[desc.Digest] = struct{}{}
}

// Jobs returns a list of all tracked descriptors
func (j *Jobs) Jobs() []ocispec.Descriptor {
	j.mu.Lock()
	defer j.mu.Unlock()

	var descs []ocispec.Descriptor
	return append(descs, j.descs...)
}

// IsResolved checks whether a descriptor has been resolved
func (j *Jobs) IsResolved() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.resolved
}

// Display pretty prints out the download or upload progress
func Display(statuses []ctrcontent.StatusInfo, start time.Time, logger event.Logger) {
	var total int64
	for _, status := range statuses {
		total += status.Offset
		elapsed := fmt.Sprintf("elapsed: %-4.1fs\ttotal: %7.6v\t(%v)\t\n",
			time.Since(start).Seconds(),
			// TODO(stevvooe): These calculations are actually way off.
			// Need to account for previously downloaded data. These
			// will basically be right for a download the first time
			// but will be skewed if restarting, as it includes the
			// data into the start time before.
			progress.Bytes(total),
			progress.NewBytesPerSecond(total, time.Since(start)))
		switch status.Status {
		case "downloading", "uploading":
			var bar progress.Bar
			if status.Total > 0.0 {
				bar = progress.Bar(float64(status.Offset) / float64(status.Total))
			}
			barFormat := fmt.Sprintf("%40r\t%8.8s/%s\t%s", bar, progress.Bytes(status.Offset), progress.Bytes(status.Total), elapsed)
			containerdLogFormat(status, barFormat, logger)
		case "resolving", "waiting":
			bar := progress.Bar(0.0)
			barFormat := fmt.Sprintf("%40r\t%s", bar, elapsed)
			containerdLogFormat(status, barFormat, logger)
		default:
			bar := progress.Bar(1.0)
			barFormat := fmt.Sprintf("%40r\t%s", bar, elapsed)
			containerdLogFormat(status, barFormat, logger)
		}
	}
}

func containerdLogFormat(status ctrcontent.StatusInfo, barFormat string, logger event.Logger) {
	var jm JSONMessage
	jm = JSONMessage{
		Status: status.Status,
		Progress: &JSONProgress{
			Current: status.Offset,
			Total:   status.Total,
		},
		ProgressMessage: barFormat,
		ID:              status.Ref,
	}
	printLog(logger, "debug", fmt.Sprintf(jm.JSONString()), map[string]string{"step": "progress"})
}
