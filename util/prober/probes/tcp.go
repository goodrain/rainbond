package probe

import (
	"context"
	"fmt"
	"net"
	"time"

	v1 "github.com/goodrain/rainbond/util/prober/types/v1"
	"github.com/sirupsen/logrus"
)

// TCPProbe probes through the tcp protocol
type TCPProbe struct {
	Name          string
	Address       string
	ResultsChan   chan *v1.HealthStatus
	Ctx           context.Context
	Cancel        context.CancelFunc
	TimeoutSecond int
	TimeInterval  int
	MaxErrorsNum  int
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
	logrus.Debugf("TCP check; Name: %s; Address: %s Interval %d", h.Name, h.Address, h.TimeInterval)
	timer := time.NewTimer(time.Second * time.Duration(h.TimeInterval))
	defer timer.Stop()
	for {
		HealthMap := h.GetTCPHealth()
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
func (h *TCPProbe) GetTCPHealth() map[string]string {
	address := h.Address
	conn, err := net.DialTimeout("tcp", address, time.Duration(h.TimeoutSecond)*time.Second)
	if conn != nil {
		defer conn.Close()
	}
	if err != nil {
		logrus.Debugf("probe health check, %s connection failure", address)
		return map[string]string{"status": v1.StatDeath,
			"info": fmt.Sprintf("Address: %s; Tcp connection error", address)}
	}
	return map[string]string{"status": v1.StatHealthy, "info": "service health"}
}
