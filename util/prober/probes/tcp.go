package probe

import (
	"context"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/util/prober/types/v1"
	"net"
	"time"
)

// TCPProbe probes through the tcp protocol
type TCPProbe struct {
	Name         string
	Address      string
	ResultsChan  chan *v1.HealthStatus
	Ctx          context.Context
	Cancel       context.CancelFunc
	TimeInterval int
	MaxErrorsNum int
}

// Check starts tcp probe.
func (h *TCPProbe) Check() {
	go h.TCPCheck()
}

// Stop stops tcp probe.
func (h *TCPProbe) Stop() {
	h.Cancel()
}

// TCPCheck -
func (h *TCPProbe) TCPCheck() {
	logrus.Debug("tcp check...")
	timer := time.NewTimer(time.Second * time.Duration(h.TimeInterval))
	defer timer.Stop()
	for {
		HealthMap := GetTCPHealth(h.Address)
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

//GetTCPHealth get tcp health
func GetTCPHealth(address string) map[string]string {
	conn, err := net.DialTimeout("tcp", address, 5 * time.Second)
	if err != nil {
		return map[string]string{"status": v1.StatDeath,
			"info": fmt.Sprintf("Address: %s; Tcp connection error", address)}
	}
	defer conn.Close()
	return map[string]string{"status": v1.StatHealthy, "info": "service health"}
}
