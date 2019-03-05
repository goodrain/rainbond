package probe

import (
	"context"
	"github.com/goodrain/rainbond/prober/types/v1"
	"net"
	"time"
)

type TcpProbe struct {
	Name         string
	Address      string
	ResultsChan  chan *v1.HealthStatus
	Ctx          context.Context
	Cancel       context.CancelFunc
	TimeInterval int
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
		result := &v1.HealthStatus{
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
	conn, err := net.DialTimeout("tcp", address, 5 * time.Second)
	if err != nil {
		return map[string]string{"status": v1.StatDeath, "info": "Tcp connection error"}
	}
	defer conn.Close()
	return map[string]string{"status": v1.StatHealthy, "info": "service health"}
}
