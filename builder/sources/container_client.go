package sources

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"time"
)

const (
	// ContainerRuntimeDocker docker runtime
	ContainerRuntimeDocker = "docker"
	// ContainerRuntimeContainerd containerd runtime
	ContainerRuntimeContainerd = "containerd"
	// RuntimeEndpointDocker docker runtime endpoint
	RuntimeEndpointDocker = "/var/run/dockershim.sock"
	// RuntimeEndpointContainerd containerd runtime endpoint
	RuntimeEndpointContainerd = "/run/containerd/containerd.sock"
)

const (
	// CONTAINER_ACTION_START is start container event action
	CONTAINER_ACTION_START = "start"

	// CONTAINER_ACTION_STOP is stop container event action
	CONTAINER_ACTION_STOP = "stop"

	// CONTAINER_ACTION_CREATE is create container event action
	CONTAINER_ACTION_CREATE = "create"

	// CONTAINER_ACTION_DESTROY is destroy container event action
	CONTAINER_ACTION_DESTROY = "destroy"

	// CONTAINER_ACTION_DIE is die container event action
	CONTAINER_ACTION_DIE = "die"
)

type ContainerDesc struct {
	ContainerRuntime string
	// Info is extra information of the Container. The key could be arbitrary string, and
	// value should be in json format. The information could include anything useful for
	// debug, e.g. pid for linux container based container runtime.
	// It should only be returned non-empty when Verbose is true.
	Info map[string]string
	*runtimeapi.ContainerStatus
	// Docker container json
	*types.ContainerJSON
}

func (c *ContainerDesc) GetLogPath() string {
	if c.ContainerRuntime == ContainerRuntimeDocker {
		logrus.Infof("docker container log path %s", c.ContainerJSON.LogPath)
		return c.ContainerJSON.LogPath
	}
	logrus.Infof("containerd container log path %s", c.ContainerStatus.GetLogPath())
	return c.ContainerStatus.GetLogPath()
}

func (c *ContainerDesc) GetId() string {
	if c.ContainerRuntime == ContainerRuntimeDocker {
		logrus.Infof("docker container id %s", c.ContainerJSON.ID)
		return c.ContainerJSON.ID
	}
	logrus.Infof("containerd container id %s", c.ContainerStatus.GetId())
	return c.ContainerStatus.GetId()
}

// ContainerImageCli container image client
type ContainerImageCli interface {
	ListContainers() ([]*runtimeapi.Container, error)
	InspectContainer(containerID string) (*ContainerDesc, error)
	WatchContainers(ctx context.Context, cchan chan ContainerEvent) error
}

// ClientFactory client factory
type ClientFactory interface {
	NewClient(endpoint string, timeout time.Duration) (ContainerImageCli, error)
}

// NewContainerImageClient new container image client
func NewContainerImageClient(containerRuntime, endpoint string, timeout time.Duration) (c ContainerImageCli, err error) {
	logrus.Infof("create container client runtime %s endpoint %s", containerRuntime, endpoint)
	switch containerRuntime {
	case ContainerRuntimeDocker:
		factory := &dockerClientFactory{}
		c, err = factory.NewClient(
			endpoint, timeout,
		)
	case ContainerRuntimeContainerd:
		factory := &containerdClientFactory{}
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

//ContainerEvent container event
type ContainerEvent struct {
	Action    string
	Container *ContainerDesc
}

func CacheContainer(cchan chan ContainerEvent, cs ...ContainerEvent) {
	for _, container := range cs {
		logrus.Debugf("found a container %s %s", container.Container.GetMetadata().GetName(), container.Action)
		cchan <- container
	}
}
