package probe

import (
	"context"
	"github.com/goodrain/rainbond/node/nodem/service"
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
	MaxErrorsNum int
}

func (h *ShellProbe) ShellCheck() {
	errNum := 1
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
		if HealthMap["status"] != service.Stat_healthy {
			if errNum > h.MaxErrorsNum {
				v := client.NodeCondition{
					Type:               client.NodeConditionType(h.Name),
					Status:             client.ConditionFalse,
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
					Message:            result.Info,
				}
				h.HostNode.UpdataCondition(v)
			} else {
				v := client.NodeCondition{
					Type:               client.NodeConditionType(h.Name),
					Status:             client.ConditionTrue,
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
				}
				h.HostNode.UpdataCondition(v)
			}
			errNum += 1
		} else {
			v := client.NodeCondition{
				Type:               client.NodeConditionType(h.Name),
				Status:             client.ConditionTrue,
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
			}
			h.HostNode.UpdataCondition(v)
			errNum = 1
		}
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
