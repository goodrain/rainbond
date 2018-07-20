package healthy

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"time"
	"net"
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
}


func (h *TcpProbe) TcpCheck() {

	util.Exec(h.ctx, func() error {
		HealthMap := GetTcpHealth(h.address)
		result := &service.HealthStatus{
			Name:        h.name,
			Status:      HealthMap["status"],
			Info:        HealthMap["info"],
		}
		h.resultsChan <- result
		return nil
	}, time.Second*time.Duration(h.TimeInterval))
}

func GetTcpHealth(address string) map[string]string {
	conn, err := net.Dial("tcp",address)
	if err != nil {
		return map[string]string{"status": service.Stat_death, "info": "Tcp connection error"}
	}
	defer conn.Close()
	return map[string]string{"status": service.Stat_healthy, "info": "service health"}

}
