package handler

import (
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/worker/client"
)

// PodHandler defines handler methods about k8s pods.
type PodHandler interface {
	PodDetail(serviceID, podName string) (*model.PodDetail, error)
}

// NewPodHandler creates a new PodHandler.
func NewPodHandler(statusCli *client.AppRuntimeSyncClient) PodHandler {
	return &PodAction{
		statusCli: statusCli,
	}
}
