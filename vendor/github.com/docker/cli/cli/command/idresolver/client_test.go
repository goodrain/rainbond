package idresolver

import (
	"github.com/docker/cli/client"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"golang.org/x/net/context"
)

type fakeClient struct {
	client.Client
	nodeInspectFunc    func(string) (swarm.Node, []byte, error)
	serviceInspectFunc func(string) (swarm.Service, []byte, error)
}

func (cli *fakeClient) NodeInspectWithRaw(ctx context.Context, nodeID string) (swarm.Node, []byte, error) {
	if cli.nodeInspectFunc != nil {
		return cli.nodeInspectFunc(nodeID)
	}
	return swarm.Node{}, []byte{}, nil
}

func (cli *fakeClient) ServiceInspectWithRaw(ctx context.Context, serviceID string, options types.ServiceInspectOptions) (swarm.Service, []byte, error) {
	if cli.serviceInspectFunc != nil {
		return cli.serviceInspectFunc(serviceID)
	}
	return swarm.Service{}, []byte{}, nil
}
