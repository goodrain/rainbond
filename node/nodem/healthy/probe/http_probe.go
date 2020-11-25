package probe

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

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
	if h.TimeInterval == 0 {
		h.TimeInterval = 5
	}
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

// Return true if the underlying error indicates a http.Client timeout.
//
// Use for errors returned from http.Client methods (Get, Post).
func isClientTimeout(err error) bool {
	if uerr, ok := err.(*url.Error); ok {
		if nerr, ok := uerr.Err.(net.Error); ok && nerr.Timeout() {
			return true
		}
	}
	return false
}

//GetHTTPHealth get http health
func GetHTTPHealth(address string) map[string]string {
	c := &http.Client{
		Timeout: 10 * time.Second,
	}
	if !strings.HasPrefix(address, "http") {
		address = "http://" + address
	}
	addr, err := url.Parse(address)
	if err != nil {
		logrus.Errorf("%s is invalid %s", address, err.Error())
		return map[string]string{"status": service.Stat_healthy, "info": "check url is invalid"}
	}
	if addr.Scheme == "" {
		addr.Scheme = "http"
	}
	resp, err := c.Get(addr.String())
	if err != nil {
		if isClientTimeout(err) {
			return map[string]string{"status": service.Stat_death, "info": "Request service timeout"}
		}
		logrus.Debugf("http probe request error %s", err.Error())
		return map[string]string{"status": service.Stat_unhealthy, "info": err.Error()}
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode >= 500 {
		logrus.Debugf("http probe check address %s return code %d", address, resp.StatusCode)
		return map[string]string{"status": service.Stat_unhealthy, "info": "Service unhealthy"}
	}
	return map[string]string{"status": service.Stat_healthy, "info": "service health"}
}
