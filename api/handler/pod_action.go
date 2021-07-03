package handler

import (
	"github.com/goodrain/rainbond/worker/server/pb"
	"strings"

	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server"
)

// PodAction is an implementation of PodHandler
type PodAction struct {
	statusCli *client.AppRuntimeSyncClient
}

// PodDetail -
func (p *PodAction) PodDetail(namespace, podName string) (*pb.PodDetail, error) {
	pd, err := p.statusCli.GetPodDetail(namespace, podName)
	if err != nil {
		if strings.Contains(err.Error(), server.ErrPodNotFound.Error()) {
			return nil, server.ErrPodNotFound
		}
		return nil, err
	}
	return pd, nil
}
