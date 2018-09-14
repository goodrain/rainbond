package probe

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"time"
	"net"
	"github.com/goodrain/rainbond/node/nodem/client"
)

type TcpProbe struct {
	Name         string
	Address      string
	ResultsChan  chan *service.HealthStatus
	Ctx          context.Context
	Cancel       context.CancelFunc
	TimeInterval int
	HostNode     *client.HostNode
	MaxErrorsNum int
}

func (h *TcpProbe) TcpCheck() {
	errNum := 1
	timer := time.NewTimer(time.Second * time.Duration(h.TimeInterval))
	defer timer.Stop()
	for {
		HealthMap := GetTcpHealth(h.Address)
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

func GetTcpHealth(address string) map[string]string {

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return map[string]string{"status": service.Stat_death, "info": "Tcp connection error"}
	}
	defer conn.Close()
	return map[string]string{"status": service.Stat_healthy, "info": "service health"}
}
