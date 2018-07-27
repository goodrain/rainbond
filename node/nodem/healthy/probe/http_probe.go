package probe

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"net/http"
	"time"
	"github.com/goodrain/rainbond/node/nodem/client"
)



type HttpProbe struct {
	Name         string
	Address      string
	ResultsChan  chan *service.HealthStatus
	Ctx          context.Context
	Cancel       context.CancelFunc
	TimeInterval int
	HostNode     *client.HostNode
}

func (h *HttpProbe) HttpCheck() {

	util.Exec(h.Ctx, func() error {
		HealthMap := GetHttpHealth(h.Address)
		result := &service.HealthStatus{
			Name:   h.Name,
			Status: HealthMap["status"],
			Info:   HealthMap["info"],
		}
		if HealthMap["status"] != service.Stat_healthy {
			v := client.NodeCondition{
				Type:               client.NodeConditionType(h.Name),
				Status:             client.ConditionFalse,
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
				Message:            result.Info,
			}
			h.HostNode.UpdataCondition(v)
		}
		if HealthMap["status"] == service.Stat_healthy {
			v := client.NodeCondition{
				Type:               client.NodeConditionType(h.Name),
				Status:             client.ConditionTrue,
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
			}
			h.HostNode.UpdataCondition(v)
		}
		h.ResultsChan <- result

		return nil
	}, time.Second*time.Duration(h.TimeInterval))
}

func GetHttpHealth(address string) map[string]string {
	resp, err := http.Get("http://" + address)
	if err != nil {
		return map[string]string{"status": service.Stat_death, "info": "Request service is unreachable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return map[string]string{"status": service.Stat_unhealthy, "info": "Service unhealthy"}
	}
	return map[string]string{"status": service.Stat_healthy, "info": "service health"}

}
