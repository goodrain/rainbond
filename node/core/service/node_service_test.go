package service

import (
	"testing"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/event"

	etcdClient "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/masterserver"
	"github.com/goodrain/rainbond/node/masterserver/node"
	"github.com/goodrain/rainbond/node/nodem/client"
)

func TestAsynchronousInstall(t *testing.T) {
	config := &option.Conf{
		Etcd:           etcdClient.Config{Endpoints: []string{"192.168.195.1:2379"}},
		EventLogServer: []string{"192.168.195.1:6366"},
	}

	store.NewClient(config)
	// node *client.HostNode, eventID string
	eventID := "fanyangyang"
	// nodemanager := nodem.NewNodeManager(&option.Conf{})
	currentNode := client.HostNode{
		ID:         "123",
		Role:       []string{"manage"},
		InternalIP: "127.0.0.1",
		RootPass:   "password",
	}
	cluster := node.CreateCluster(nil, &currentNode, nil)
	cluster.CacheNode(&currentNode)
	ms := &masterserver.MasterServer{Cluster: cluster}
	nodeService := CreateNodeService(config, ms.Cluster, nil)
	nodeService.AsynchronousInstall(&currentNode, eventID)
	event.GetManager().Close()
}
