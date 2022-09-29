package sources

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockercli "github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"os"
	"strings"
	"time"
)

var handleAction = []string{CONTAINER_ACTION_CREATE, CONTAINER_ACTION_START, CONTAINER_ACTION_STOP, CONTAINER_ACTION_DIE, CONTAINER_ACTION_DESTROY}

func checkEventAction(action string) bool {
	for _, enable := range handleAction {
		if enable == action {
			return true
		}
	}
	return false
}

type dockerClientFactory struct{}

var _ ClientFactory = &dockerClientFactory{}

func (f dockerClientFactory) NewClient(endpoint string, timeout time.Duration) (ContainerImageCli, error) {
	if os.Getenv("DOCKER_API_VERSION") == "" {
		os.Setenv("DOCKER_API_VERSION", "1.40")
	}
	cli, err := dockercli.NewClientWithOpts(dockercli.FromEnv)
	if err != nil {
		return nil, err
	}
	return &dockerClientImpl{
		client: cli,
	}, nil
}

var _ ContainerImageCli = &dockerClientImpl{}

type dockerClientImpl struct {
	client *dockercli.Client
}

func (d *dockerClientImpl) ListContainers() ([]*runtimeapi.Container, error) {
	lictctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	containers, err := d.client.ContainerList(lictctx, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}
	// convert to runtimeapi.Container
	var runtimeContainers []*runtimeapi.Container
	for _, container := range containers {
		runtimeContainers = append(runtimeContainers, &runtimeapi.Container{
			Id: container.ID,
			Metadata: &runtimeapi.ContainerMetadata{
				Name: container.Names[0],
			},
			Image: &runtimeapi.ImageSpec{
				Image: container.Image,
			},
			ImageRef:  container.ImageID,
			Labels:    container.Labels,
			CreatedAt: container.Created,
		})
	}
	return runtimeContainers, nil
}

func (d *dockerClientImpl) InspectContainer(containerID string) (*ContainerDesc, error) {
	inspectctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	container, err := d.client.ContainerInspect(inspectctx, containerID)
	if err != nil {
		return nil, err
	}
	return &ContainerDesc{
		ContainerRuntime: ContainerRuntimeDocker,
		ContainerJSON:    &container,
	}, nil
}

func (d *dockerClientImpl) WatchContainers(ctx context.Context, cchan chan ContainerEvent) error {
	containerFileter := filters.NewArgs()
	containerFileter.Add("type", "container")
	eventchan, eventerrchan := d.client.Events(ctx, types.EventsOptions{
		Filters: containerFileter,
	})
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-eventerrchan:
			return err
		case event, ok := <-eventchan:
			if !ok {
				return fmt.Errorf("event chan is closed")
			}
			if event.Type == events.ContainerEventType && checkEventAction(event.Action) {
				container, err := d.InspectContainer(event.ID)
				if err != nil {
					if !strings.Contains(err.Error(), "No such container") {
						logrus.Errorf("get container detail info failure %s", err.Error())
					}
					break
				}
				CacheContainer(cchan, ContainerEvent{Action: event.Action, Container: container})
			}
		}
	}
}

func (d *dockerClientImpl) GetRuntimeClient() (*runtimeapi.RuntimeServiceClient, error) {
	return nil, fmt.Errorf("docker client not support get runtime client")
}

func (d *dockerClientImpl) GetDockerClient() (*dockercli.Client, error) {
	return d.client, nil
}
