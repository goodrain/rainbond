package sources

import (
	"fmt"
	"github.com/containerd/containerd"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/event"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"time"
)

// ImageClient image client
type ImageClient interface {
	GetContainerdClient() *containerd.Client
	GetDockerClient() *dockercli.Client
	CheckIfImageExists(imageName string) (imageRef string, exists bool, err error)
	ImagePull(image string, username, password string, logger event.Logger, timeout int) (*ocispec.ImageConfig, error)
	ImageTag(source, target string, logger event.Logger, timeout int) error
	ImagePush(image, user, pass string, logger event.Logger, timeout int) error
	ImagesPullAndPush(sourceImage, targetImage, username, password string, logger event.Logger) error
	ImageRemove(image string) error
	ImageSave(image, destination string) error
	ImageLoad(tarFile string, logger event.Logger) ([]string, error)
	TrustedImagePush(image, user, pass string, logger event.Logger, timeout int) error
	// GetImageMetadata 轻量级获取镜像元数据（不下载镜像层）
	// 只下载 manifest 和 config blob（通常 < 20KB），不下载完整镜像层
	// 返回镜像配置信息，失败时返回错误但不应阻塞构建流程
	GetImageMetadata(image string, username, password string, logger event.Logger) (*ocispec.ImageConfig, error)
}

// ImageClientFactory client factory
type ImageClientFactory interface {
	NewClient(endpoint string, timeout time.Duration) (ImageClient, error)
}

// NewImageClient new image client
func NewImageClient() (c ImageClient, err error) {
	containerRuntime := configs.Default().ChaosConfig.ContainerRuntime
	runtimeEndpoint := configs.Default().ChaosConfig.RuntimeEndpoint
	logrus.Infof("create container client runtime %s endpoint %s", containerRuntime, runtimeEndpoint)
	switch containerRuntime {
	case ContainerRuntimeDocker:
		factory := &dockerImageCliFactory{}
		c, err = factory.NewClient(
			runtimeEndpoint, time.Second*3,
		)
	case ContainerRuntimeContainerd:
		factory := &containerdImageCliFactory{}
		c, err = factory.NewClient(
			runtimeEndpoint, time.Second*3,
		)
		return
	default:
		err = fmt.Errorf("unknown runtime %s", containerRuntime)
		return
	}
	return
}
