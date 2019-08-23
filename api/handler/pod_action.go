package handler

import (
	"encoding/json"
	"strings"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server"
)

// PodAction is an implementation of PodHandler
type PodAction struct {
	statusCli *client.AppRuntimeSyncClient
}

// PodDetail -
func (p *PodAction) PodDetail(serviceID, podName string) (*model.PodDetail, error) {
	pd, err := p.statusCli.GetPodDetail(serviceID, podName)
	if err != nil {
		if strings.Contains(err.Error(), server.ErrPodNotFound.Error()) {
			return nil, server.ErrPodNotFound
		}
		return nil, err
	}
	b, err := json.Marshal(pd)
	if err != nil {
		return nil, err
	}
	var podDetail model.PodDetail
	if err := json.Unmarshal(b, &podDetail); err != nil {
		return nil, err
	}
	podDetail.Status.Type = pd.Status.Type.String()
	return &podDetail, nil
}
