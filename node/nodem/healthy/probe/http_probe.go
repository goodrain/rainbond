package probe

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/service"
)

type HttpProbe struct {
	Name         string
	Address      string
	ResultsChan  chan *service.HealthStatus
	Ctx          context.Context
	Cancel       context.CancelFunc
	TimeInterval int
	HostNode     *client.HostNode
	MaxErrorsNum int
}

//Check check
func (h *HttpProbe) Check() {
	go h.HTTPCheck()
}

//Stop stop
func (h *HttpProbe) Stop() {
	h.Cancel()
}

//HTTPCheck http check
func (h *HttpProbe) HTTPCheck() {
	timer := time.NewTimer(time.Second * time.Duration(h.TimeInterval))
	defer timer.Stop()
	for {
		HealthMap := GetHTTPHealth(h.Address)
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

//GetHTTPHealth get http health
func GetHTTPHealth(address string) map[string]string {
	c := &http.Client{
		Timeout: 5 * time.Second,
	}
	if !strings.Contains(address, "://") {
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
