package probe

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/service"
)

type ShellProbe struct {
	Name         string
	Address      string
	ResultsChan  chan *service.HealthStatus
	Ctx          context.Context
	Cancel       context.CancelFunc
	TimeInterval int
	HostNode     *client.HostNode
	MaxErrorsNum int
}

func (h *ShellProbe) Check() {
	go h.ShellCheck()
}
func (h *ShellProbe) Stop() {
	h.Cancel()
}
func (h *ShellProbe) ShellCheck() {
	timer := time.NewTimer(time.Second * time.Duration(h.TimeInterval))
	defer timer.Stop()
	for {
		HealthMap := GetShellHealth(h.Address)
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

func GetShellHealth(address string) map[string]string {

	cmd := exec.Command("/bin/bash", "-c", address)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errStr := string(stderr.Bytes())
		return map[string]string{"status": service.Stat_death, "info": strings.TrimSpace(errStr)}
	}
	return map[string]string{"status": service.Stat_healthy, "info": "service healthy"}
}
