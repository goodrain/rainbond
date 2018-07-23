package healthy

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"net/http"
	"time"
	"github.com/goodrain/rainbond/node/nodem/client"
)

type Probe interface {
	Check()
}

type HttpProbe struct {
	name         string
	address      string
	resultsChan  chan *service.HealthStatus
	ctx          context.Context
	cancel       context.CancelFunc
	TimeInterval int
	hostNode     *client.HostNode
}


func (h *HttpProbe) Check() {

	util.Exec(h.ctx, func() error {
		HealthMap := GetHttpHealth(h.address)
			result := &service.HealthStatus{
				Name:        h.name,
				Status:      HealthMap["status"],
				Info:        HealthMap["info"],
			}
			if result.Status != service.Stat_healthy{
				v := client.NodeCondition{
					Type:client.NodeConditionType(result.Name),
					Status:client.ConditionFalse,
					Message:result.Info,
				}
				v2 := client.NodeCondition{
					Type:client.NodeReady,
					Status:client.ConditionFalse,
				}
				h.hostNode.UpdataCondition(v,v2)
			}
			h.resultsChan <- result


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
