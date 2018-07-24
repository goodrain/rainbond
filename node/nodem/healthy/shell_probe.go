package healthy

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

type SHELLProbe interface {
	Check()
}

type ShellProbe struct {
	name         string
	address      string
	resultsChan  chan *service.HealthStatus
	ctx          context.Context
	cancel       context.CancelFunc
	TimeInterval int
	hostNode     *client.HostNode
}

func (h *ShellProbe) ShellCheck() {

	util.Exec(h.ctx, func() error {
		HealthMap := GetShellHealth(h.address)
		result := &service.HealthStatus{
			Name:   h.name,
			Status: HealthMap["status"],
			Info:   HealthMap["info"],
		}
		if result.Status != service.Stat_healthy {
			v := client.NodeCondition{
				Type:    client.NodeConditionType(result.Name),
				Status:  client.ConditionFalse,
				LastHeartbeatTime:time.Now(),
				LastTransitionTime:time.Now(),
				Message: result.Info,
			}
			v2 := client.NodeCondition{
				Type:   client.NodeReady,
				Status: client.ConditionFalse,
				LastHeartbeatTime:time.Now(),
				LastTransitionTime:time.Now(),
			}
			h.hostNode.UpdataCondition(v, v2)
		}
		h.resultsChan <- result

		return nil
	}, time.Second*time.Duration(h.TimeInterval))
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
