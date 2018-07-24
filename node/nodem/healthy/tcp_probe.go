package healthy

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"time"
	"net"
	"github.com/goodrain/rainbond/node/nodem/client"
)

type TCPProbe interface {
	TcpCheck()
}

type TcpProbe struct {
	name         string
	address      string
	resultsChan  chan *service.HealthStatus
	ctx          context.Context
	cancel       context.CancelFunc
	TimeInterval int
	hostNode     *client.HostNode
}

func (h *TcpProbe) TcpCheck() {

	util.Exec(h.ctx, func() error {
		HealthMap := GetTcpHealth(h.address)
		result := &service.HealthStatus{
			Name:   h.name,
			Status: HealthMap["status"],
			Info:   HealthMap["info"],
		}
		if HealthMap["status"] != service.Stat_healthy {
			v := client.NodeCondition{
				Type:    client.NodeConditionType(h.name),
				Status:  client.ConditionFalse,
				LastHeartbeatTime:time.Now(),
				LastTransitionTime:time.Now(),
				Message: result.Info,
			}
			h.hostNode.UpdataCondition(v)
		}
		if HealthMap["status"] == service.Stat_healthy{
			v := client.NodeCondition{
				Type:client.NodeConditionType(h.name),
				Status:client.ConditionTrue,
				LastHeartbeatTime:time.Now(),
				LastTransitionTime:time.Now(),
			}
			h.hostNode.UpdataCondition(v)
		}
		h.resultsChan <- result
		return nil
	}, time.Second*time.Duration(h.TimeInterval))
}

func GetTcpHealth(address string) map[string]string {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return map[string]string{"status": service.Stat_death, "info": "Tcp connection error"}
	}
	defer conn.Close()
	return map[string]string{"status": service.Stat_healthy, "info": "service health"}

}
