package probe

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"net/http"
	"time"
	"github.com/goodrain/rainbond/node/nodem/client"
	"strings"
)

type HttpProbe struct {
	Name          string
	Address       string
	ResultsChan   chan *service.HealthStatus
	Ctx           context.Context
	Cancel        context.CancelFunc
	TimeInterval  int
	HostNode      *client.HostNode
	MaxErrorsNum int
}

func (h *HttpProbe) HttpCheck() {
	errNum := 1
	timer := time.NewTimer(time.Second * time.Duration(h.TimeInterval))
	defer timer.Stop()
	for {
		HealthMap := GetHttpHealth(h.Address)
		result := &service.HealthStatus{
			Name:   h.Name,
			Status: HealthMap["status"],
			Info:   HealthMap["info"],
		}
		h.ResultsChan <- result
		if HealthMap["status"] != service.Stat_healthy {
			if errNum > h.MaxErrorsNum {
				v := client.NodeCondition{
					Type:               client.NodeConditionType(h.Name),
					Status:             client.ConditionFalse,
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
					Message:            result.Info,
				}
				h.HostNode.UpdataCondition(v)
			} else {
				v := client.NodeCondition{
					Type:               client.NodeConditionType(h.Name),
					Status:             client.ConditionTrue,
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
				}
				h.HostNode.UpdataCondition(v)
			}
			errNum += 1
		} else {
			v := client.NodeCondition{
				Type:               client.NodeConditionType(h.Name),
				Status:             client.ConditionTrue,
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
			}
			h.HostNode.UpdataCondition(v)
			errNum = 1
		}
		timer.Reset(time.Second * time.Duration(h.TimeInterval))
		select {
		case <-h.Ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func GetHttpHealth(address string) map[string]string {
	c := &http.Client{
		Timeout: 5 * time.Second,
	}
	if !strings.Contains(address, "://"){
		address = "http://" + address
	}
	resp, err := c.Get(address)
	if err != nil {
		return map[string]string{"status": service.Stat_death, "info": "Request service is unreachable"}

	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return map[string]string{"status": service.Stat_unhealthy, "info": "Service unhealthy"}
	}
	return map[string]string{"status": service.Stat_healthy, "info": "service health"}

}
