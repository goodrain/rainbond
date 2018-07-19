package healthy

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"net/http"
	"time"
)

var errorNum int = 0

type Probe interface {
	Check() map[string]string
}

type HttpProbe struct {
	name           string
	address        string
	resultsChan    chan *service.HealthStatus
	ctx            context.Context
	cancel         context.CancelFunc
	TimeInterval   int
	MaxErrorNumber int
}

func (h *HttpProbe) Check() {
	util.Exec(h.ctx, func() error {
		HealthMap := GetHttpHealth(h.address)

		if HealthMap["status"] != "health" {
			errorNum += 1
		} else {
			errorNum = 0
		}

		if errorNum >= h.MaxErrorNumber {
			result := &service.HealthStatus{
				Name:   h.name,
				Status: "death",
				Info:   "More than the maximum number of errors, needs to be restarted",
			}
			h.resultsChan <- result
		} else {
			result := &service.HealthStatus{
				Name:   h.name,
				Status: HealthMap["status"],
				Info:   HealthMap["info"],
			}
			h.resultsChan <- result
		}

		return nil
	}, time.Second*time.Duration(h.TimeInterval))
}

func GetHttpHealth(address string) map[string]string {
	resp, err := http.Get("http://" + address)
	if err != nil {
		return map[string]string{"status": "disconnect", "info": "Request service is unreachable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return map[string]string{"status": "unhealthy", "info": "Service unhealthy"}
	}
	return map[string]string{"status": "health", "info": "service health"}

}
