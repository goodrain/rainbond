package testcontainers

import (
	"context"

	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ DeprecatedContainer = (*DockerContainer)(nil)

// GetHostEndpoint is deprecated and kept for backwards compat
// Deprecated: Use Endpoint()
func (c *DockerContainer) GetHostEndpoint(ctx context.Context, port string) (string, string, error) {
	outerPort, err := c.MappedPort(ctx, nat.Port(port))
	if err != nil {
		return "", "", err
	}

	ip, err := c.Host(ctx)
	if err != nil {
		return "", "", err
	}

	return ip, outerPort.Port(), nil
}

// GetIPAddress is deprecated and kept for backwards compat
// Deprecated: Use IPAddress()
func (c *DockerContainer) GetIPAddress(ctx context.Context) (string, error) {
	return c.Host(ctx)
}

// LivenessCheckPorts is deprecated and kept for backwards compat
// Deprecated: Use Ports()
func (c *DockerContainer) LivenessCheckPorts(ctx context.Context) (nat.PortSet, error) {
	ports, err := c.Ports(ctx)
	var portSet nat.PortSet
	for port := range ports {
		portSet[port] = struct{}{}
	}
	return portSet, err
}

// RequestContainer supplies input parameters for a container
// Deprecated: Use ContainerRequest with provider pattern
type RequestContainer struct {
	Env          map[string]string
	ExportedPort []string
	Cmd          string
	RegistryCred string
	WaitingFor   wait.Strategy
}

// RunContainer takes a RequestContainer as input and it runs a container via the docker sdk
// Deprecated: Use GenericContainer()
func RunContainer(ctx context.Context, containerImage string, input RequestContainer) (DeprecatedContainer, error) {
	req := ContainerRequest{
		Image:        containerImage,
		Env:          input.Env,
		ExposedPorts: input.ExportedPort,
		Cmd:          input.Cmd,
		RegistryCred: input.RegistryCred,
		WaitingFor:   input.WaitingFor,
	}

	container, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: req,
		ProviderType:     ProviderDocker,
		Started:          true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch generic container")
	}

	legacyContainer, ok := container.(*DockerContainer)
	if !ok {
		return nil, errors.New("failed to get docker container from provider")
	}

	return legacyContainer, nil
}
