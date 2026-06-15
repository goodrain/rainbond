package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/server/pb"
	v1 "kubevirt.io/api/core/v1"
)

const (
	vmFixedIPEnabledAttribute = "vm_fixed_ip_enabled"
	vmFixedIPAttribute        = "vm_fixed_ip"
)

type VMFixedPodIPResult struct {
	FixedIPEnabled bool   `json:"fixed_ip_enabled"`
	FixedIP        string `json:"fixed_ip"`
	Restarted      bool   `json:"restarted"`
}

func (s *ServiceAction) SetVMFixedPodIP(ctx context.Context, serviceID string, enabled bool) (*VMFixedPodIPResult, error) {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, err
	}
	if service == nil || !service.IsVM() {
		return nil, newVMLiveUpdateError(400, "当前组件不是虚拟机，不能设置固定 IP。")
	}

	result := &VMFixedPodIPResult{FixedIPEnabled: enabled}
	if enabled {
		fixedIP, err := s.currentRunningPodIP(serviceID)
		if err != nil {
			return nil, err
		}
		result.FixedIP = fixedIP
		if err := s.saveVMFixedPodIPAttributes(service, true, fixedIP); err != nil {
			return nil, err
		}
	} else if err := s.saveVMFixedPodIPAttributes(service, false, ""); err != nil {
		return nil, err
	}

	vm, err := s.getVirtualMachineByServiceID(serviceID)
	if err != nil {
		return nil, err
	}
	if err := s.syncVirtualMachineSpecAndRestart(serviceID, vm); err != nil {
		return nil, err
	}
	result.Restarted = vm != nil && vm.Status.PrintableStatus != v1.VirtualMachineStatusStopped
	return result, nil
}

func (s *ServiceAction) currentRunningPodIP(serviceID string) (string, error) {
	pods, err := s.getServicePods(serviceID)
	if err != nil {
		return "", err
	}
	if ip := firstRunningPodIP(pods.GetNewPods()); ip != "" {
		return ip, nil
	}
	if ip := firstRunningPodIP(pods.GetOldPods()); ip != "" {
		return ip, nil
	}
	return "", newVMLiveUpdateError(409, "虚拟机 Pod IP 尚未准备完成，请启动虚拟机后再固定当前 IP。")
}

func firstRunningPodIP(pods []*pb.ServiceAppPod) string {
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		podIP := strings.TrimSpace(pod.PodIp)
		if podIP == "" {
			continue
		}
		status := strings.ToUpper(strings.TrimSpace(pod.PodStatus))
		if status == "" || strings.Contains(status, "RUNNING") {
			return podIP
		}
	}
	return ""
}

func (s *ServiceAction) saveVMFixedPodIPAttributes(service *dbmodel.TenantServices, enabled bool, fixedIP string) error {
	value := "false"
	if enabled {
		value = "true"
	}
	attrs := []*dbmodel.ComponentK8sAttributes{
		{
			TenantID:       service.TenantID,
			ComponentID:    service.ServiceID,
			Name:           vmFixedIPEnabledAttribute,
			SaveType:       "string",
			AttributeValue: value,
		},
	}
	if enabled {
		attrs = append(attrs, &dbmodel.ComponentK8sAttributes{
			TenantID:       service.TenantID,
			ComponentID:    service.ServiceID,
			Name:           vmFixedIPAttribute,
			SaveType:       "string",
			AttributeValue: fixedIP,
		})
	}
	if err := db.GetManager().ComponentK8sAttributeDao().CreateOrUpdateAttributesInBatch(attrs); err != nil {
		return fmt.Errorf("save vm fixed pod ip attributes: %w", err)
	}
	if !enabled {
		return db.GetManager().ComponentK8sAttributeDao().DeleteByComponentIDAndName(service.ServiceID, vmFixedIPAttribute)
	}
	return nil
}
