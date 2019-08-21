package handler

import (
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
)

// PodAction is an implementation of PodHandler
type PodAction struct {
	statusCli *client.AppRuntimeSyncClient
}

// PodDetail -
func (p *PodAction) PodDetail(serviceID, podName string) (*pb.PodDetail, error) {
	return p.statusCli.GetPodDetail(serviceID, podName)
}
