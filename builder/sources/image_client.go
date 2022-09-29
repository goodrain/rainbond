package sources

import (
	"fmt"
	"github.com/containerd/containerd"
	dockercli "github.com/docker/docker/client"
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
	ImageLoad(tarFile string, logger event.Logger) error
	TrustedImagePush(image, user, pass string, logger event.Logger, timeout int) error
}

// ImageClientFactory client factory
type ImageClientFactory interface {
	NewClient(endpoint string, timeout time.Duration) (ImageClient, error)
}

// NewImageClient new image client
func NewImageClient(containerRuntime, endpoint string, timeout time.Duration) (c ImageClient, err error) {
	logrus.Infof("create container client runtime %s endpoint %s", containerRuntime, endpoint)
	switch containerRuntime {
	case ContainerRuntimeDocker:
		factory := &dockerImageCliFactory{}
		c, err = factory.NewClient(
			endpoint, timeout,
		)
	case ContainerRuntimeContainerd:
		factory := &containerdImageCliFactory{}
		c, err = factory.NewClient(
			endpoint, timeout,
		)
		return
	default:
		err = fmt.Errorf("unknown runtime %s", containerRuntime)
		return
	}
	return
}
