package healthy

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"net/http"
	"time"
)

type Probe interface {
	Check() map[string]string
}

type HttpProbe struct {
	name        string
	address     string
	path        string
	resultsChan chan service.HealthStatus
	ctx         context.Context
	cancel      context.CancelFunc
}

func (h *HttpProbe) Check() {
	util.Exec(h.ctx, func() error {
		HealthMap := GetHttpHealth(h.address, h.path)
		result := service.HealthStatus{
			Name:   h.name,
			Status: HealthMap["status"],
			Info:   HealthMap["info"],
		}
		h.resultsChan <- result
		return nil
	}, time.Second*8)
}

func GetHttpHealth(address string, path string) map[string]string {
	resp, err := http.Get("http://" + address + path)
	if err != nil {
		return map[string]string{"status": "unusual", "info": "Service exception, request error"}
	}
	if resp.StatusCode >= 400 {
		return map[string]string{"status": "unusual", "info": "Service unusual"}
	}
	return map[string]string{"status": "health", "info": "service health"}

}
