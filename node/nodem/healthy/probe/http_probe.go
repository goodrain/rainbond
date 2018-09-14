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
	MaxErrorsTime int
}

func (h *HttpProbe) HttpCheck() {

	util.Exec(h.Ctx, func() error {
		HealthMap := GetHttpHealth(h.Address,h.MaxErrorsTime)
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

func GetHttpHealth(address string, maxErrorsTime int) map[string]string {
	var result map[string]string
	for num:=0; num <= maxErrorsTime;num++{
		resp, err := http.Get("http://" + address)
		if err != nil {
			result = map[string]string{"status": service.Stat_death, "info": "Request service is unreachable"}
			time.Sleep(1*time.Second)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			result = map[string]string{"status": service.Stat_unhealthy, "info": "Service unhealthy"}
			time.Sleep(1*time.Second)
			continue
		}
		return map[string]string{"status": service.Stat_healthy, "info": "service health"}
	}
	return result
}
