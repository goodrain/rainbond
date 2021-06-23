package handler

import (
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
)

// PodHandler defines handler methods about k8s pods.
type PodHandler interface {
	PodDetail(namespace, podName string) (*pb.PodDetail, error)
}

// NewPodHandler creates a new PodHandler.
func NewPodHandler(statusCli *client.AppRuntimeSyncClient) PodHandler {
	return &PodAction{
		statusCli: statusCli,
	}
}
