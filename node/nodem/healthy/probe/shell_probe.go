package probe

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"time"
	"os/exec"
	"bytes"
	"strings"
	"github.com/goodrain/rainbond/node/nodem/client"
)


type ShellProbe struct {
	Name         string
	Address      string
	ResultsChan  chan *service.HealthStatus
	Ctx          context.Context
	Cancel       context.CancelFunc
	TimeInterval int
	HostNode     *client.HostNode
	MaxErrorsTime int
}

func (h *ShellProbe) ShellCheck() {

	util.Exec(h.Ctx, func() error {
		HealthMap := GetShellHealth(h.Address, h.MaxErrorsTime)
		result := &service.HealthStatus{
			Name:   h.Name,
			Status: HealthMap["status"],
			Info:   HealthMap["info"],
		}
		if HealthMap["status"] != service.Stat_healthy {
			v := client.NodeCondition{
				Type:    client.NodeConditionType(h.Name),
				Status:  client.ConditionFalse,
				LastHeartbeatTime:time.Now(),
				LastTransitionTime:time.Now(),
				Message: result.Info,
			}
			h.HostNode.UpdataCondition(v)
		}
		if HealthMap["status"] == service.Stat_healthy{
			v := client.NodeCondition{
				Type:client.NodeConditionType(h.Name),
				Status:client.ConditionTrue,
				LastHeartbeatTime:time.Now(),
				LastTransitionTime:time.Now(),
			}
			h.HostNode.UpdataCondition(v)
		}
		h.ResultsChan <- result

		return nil
	}, time.Second*time.Duration(h.TimeInterval))
}

func GetShellHealth(address string, maxErrorsTime int) map[string]string {
	var result map[string]string
	for num:=0; num <= maxErrorsTime;num++{
		cmd := exec.Command("/bin/bash", "-c", address)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			errStr := string(stderr.Bytes())
			result =  map[string]string{"status": service.Stat_death, "info": strings.TrimSpace(errStr)}
			time.Sleep(1*time.Second)
			continue
		}
		return map[string]string{"status": service.Stat_healthy, "info": "service healthy"}
	}
	return result
}
