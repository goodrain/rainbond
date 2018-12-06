package probe

import (
	"context"
	"net"
	"time"

	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/service"
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

func (h *TcpProbe) Check() {
	go h.TcpCheck()
}
func (h *TcpProbe) Stop() {
	h.Cancel()
}
func (h *TcpProbe) TcpCheck() {
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
		timer.Reset(time.Second * time.Duration(h.TimeInterval))
		select {
		case <-h.Ctx.Done():
			return
		case <-timer.C:
		}
	}
}

//GetTcpHealth get tcp health
func GetTcpHealth(address string) map[string]string {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return map[string]string{"status": service.Stat_death, "info": "Tcp connection error"}
	}
	defer conn.Close()
	return map[string]string{"status": service.Stat_healthy, "info": "service health"}
}
