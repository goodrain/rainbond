package testcontainers

import (
	"context"

	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go/wait"
)

// DeprecatedContainer shows methods that were supported before, but are now deprecated
// Deprecated: Use Container
type DeprecatedContainer interface {
	GetHostEndpoint(ctx context.Context, port string) (string, string, error)
	GetIPAddress(ctx context.Context) (string, error)
	LivenessCheckPorts(ctx context.Context) (nat.PortSet, error)
	Terminate(ctx context.Context) error
}

// ContainerProvider allows the creation of containers on an arbitrary system
type ContainerProvider interface {
	CreateContainer(context.Context, ContainerRequest) (Container, error) // create a container without starting it
	RunContainer(context.Context, ContainerRequest) (Container, error)    // create a container and start it
}

// Container allows getting info about and controlling a single container instance
type Container interface {
	Endpoint(context.Context, string) (string, error)               // get proto://ip:port string for the first exposed port
	PortEndpoint(context.Context, nat.Port, string) (string, error) // get proto://ip:port string for the given exposed port
	Host(context.Context) (string, error)                           // get host where the container port is exposed
	MappedPort(context.Context, nat.Port) (nat.Port, error)         // get externally mapped port for a container port
	Ports(context.Context) (nat.PortMap, error)                     // get all exposed ports
	SessionID() string                                              // get session id
	Start(context.Context) error                                    // start the container
	Terminate(context.Context) error                                // terminate the container
}

// ContainerRequest represents the parameters used to get a running container
type ContainerRequest struct {
	Image        string
	Env          map[string]string
	ExposedPorts []string // allow specifying protocol info
	Cmd          string
	Labels       map[string]string
	BindMounts   map[string]string
	RegistryCred string
	WaitingFor   wait.Strategy

	isReaper bool // indicates whether we skip setting up a reaper for this
}

// ProviderType is an enum for the possible providers
type ProviderType int

// possible provider types
const (
	ProviderDocker ProviderType = iota // Docker is default = 0
)

// GetProvider provides the provider implementation for a certain type
func (t ProviderType) GetProvider() (ContainerProvider, error) {
	switch t {
	case ProviderDocker:
		provider, err := NewDockerProvider()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create Docker provider")
		}
		return provider, nil
	}
	return nil, errors.New("unknown provider")
}
