package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/api/core/v1"
)

type VMLiveUpdateCapability struct {
	CPUHotUpdateSupported    bool   `json:"cpu_hot_update_supported"`
	MemoryHotUpdateSupported bool   `json:"memory_hot_update_supported"`
	HotUpdateReason          string `json:"hot_update_reason,omitempty"`
}

const vmInstallerMediaRemovalRequiredMessage = "初始化安装光盘尚未删除，请先删除后再热更新。"

type statusCoder interface {
	StatusCode() int
}

type vmLiveUpdateError struct {
	status  int
	message string
}

func (e *vmLiveUpdateError) Error() string {
	return e.message
}

func (e *vmLiveUpdateError) StatusCode() int {
	return e.status
}

func newVMLiveUpdateError(status int, message string) error {
	return &vmLiveUpdateError{
		status:  status,
		message: message,
	}
}

func isVMLiveUpdateUnsupportedError(err error) bool {
	_, ok := err.(*vmLiveUpdateError)
	return ok
}

func (s *ServiceAction) applyVMLiveUpdateIfPossible(service *dbmodel.TenantServices, oldCPU, oldMemory int) error {
	if service == nil || !service.IsVM() {
		return nil
	}
	if s == nil || s.kubevirtClient == nil {
		return s.syncVirtualMachineSpecAfterResourceUpdate(service.ServiceID)
	}

	vm, err := s.getVirtualMachineByServiceID(service.ServiceID)
	if err != nil {
		return err
	}
	if vm == nil {
		return s.syncVirtualMachineSpecAfterResourceUpdate(service.ServiceID)
	}

	if vm.Status.PrintableStatus != v1.VirtualMachineStatusRunning {
		return s.syncVirtualMachineSpecAfterResourceUpdate(service.ServiceID)
	}
	if hasVMInstallerMediaAttached(vm) {
		return newVMLiveUpdateError(409, vmInstallerMediaRemovalRequiredMessage)
	}

	vmi, err := s.getVirtualMachineInstanceByServiceID(service.ServiceID)
	if err != nil {
		return err
	}
	if vmi == nil || vmi.Status.Phase != v1.Running {
		return s.syncVirtualMachineSpecAfterResourceUpdate(service.ServiceID)
	}

	if !isConditionTrue(vmi.Status.Conditions, v1.VirtualMachineInstanceIsMigratable) {
		return s.syncVirtualMachineSpecAndRestart(service.ServiceID, vm)
	}

	if !s.isVMLiveUpdateClusterConfigured(context.Background()) {
		return s.syncVirtualMachineSpecAndRestart(service.ServiceID, vm)
	}
	if err := s.ensureVMLiveUpdateMigrationTargetAvailable(context.Background(), service.ServiceID); err != nil {
		if isVMLiveUpdateUnsupportedError(err) {
			return s.syncVirtualMachineSpecAndRestart(service.ServiceID, vm)
		}
		return err
	}

	if service.ContainerCPU != oldCPU && service.ContainerMemory != oldMemory {
		return newVMLiveUpdateError(409, "运行中虚拟机 CPU 和内存热更新请分两次操作，不支持一次同时修改。")
	}

	patchOps := make([]map[string]any, 0, 2)

	if service.ContainerCPU < oldCPU {
		return newVMLiveUpdateError(409, "虚拟机 CPU 热更新仅支持扩容，不支持缩容，请停机后再修改规格。")
	}
	if service.ContainerCPU > oldCPU {
		if vm.Spec.Template == nil || vm.Spec.Template.Spec.Domain.CPU == nil {
			return newVMLiveUpdateError(409, "vm cpu topology is not configured for live update")
		}
		if !supportsVMSocketCPUHotUpdate(vm.Spec.Template.Spec.Domain.CPU) {
			return s.syncVirtualMachineSpecAndRestart(service.ServiceID, vm)
		}
		targetSockets, err := vmSocketsFromMilliCPU(service.ContainerCPU)
		if err != nil {
			return err
		}
		maxSockets := vm.Spec.Template.Spec.Domain.CPU.MaxSockets
		if maxSockets == 0 {
			return newVMLiveUpdateError(409, "vm maxSockets is required for cpu live update")
		}
		if targetSockets > maxSockets {
			return newVMLiveUpdateError(409, fmt.Sprintf("vm cpu live update target exceeds maxSockets (%d)", maxSockets))
		}
		if vm.Spec.Template.Spec.Domain.CPU.Sockets != targetSockets {
			patchOps = append(patchOps, map[string]any{
				"op":    "replace",
				"path":  "/spec/template/spec/domain/cpu/sockets",
				"value": targetSockets,
			})
		}
	}

	if service.ContainerMemory < oldMemory {
		return newVMLiveUpdateError(409, "虚拟机内存热更新仅支持扩容，不支持缩容，请停机后再修改规格。")
	}
	if service.ContainerMemory > oldMemory {
		if vm.Spec.Template == nil || vm.Spec.Template.Spec.Domain.Memory == nil || vm.Spec.Template.Spec.Domain.Memory.MaxGuest == nil {
			return newVMLiveUpdateError(409, "vm maxGuest is required for memory live update")
		}
		targetGuest := buildAlignedVMMemoryQuantity(service.ContainerMemory)
		if targetGuest.Cmp(*vm.Spec.Template.Spec.Domain.Memory.MaxGuest) >= 0 {
			return newVMLiveUpdateError(409, "vm memory live update target must be lower than maxGuest")
		}
		if vm.Spec.Template.Spec.Domain.Memory.Guest == nil || vm.Spec.Template.Spec.Domain.Memory.Guest.Cmp(targetGuest) != 0 {
			patchOps = append(patchOps, map[string]any{
				"op":    "replace",
				"path":  "/spec/template/spec/domain/memory/guest",
				"value": targetGuest.String(),
			})
		}
	}

	if len(patchOps) == 0 {
		return nil
	}

	payload, err := json.Marshal(patchOps)
	if err != nil {
		return err
	}
	_, err = s.kubevirtClient.VirtualMachine(vm.Namespace).Patch(context.Background(), vm.Name, types.JSONPatchType, payload, metav1.PatchOptions{})
	if err != nil {
		if isVMLiveMigrationUnsupportedPatchError(err) {
			return s.syncVirtualMachineSpecAndRestart(service.ServiceID, vm)
		}
		return err
	}
	s.enqueueCompletedVMLauncherCleanup(service.ServiceID)
	return nil
}

func (s *ServiceAction) syncVirtualMachineSpecAndRestart(serviceID string, vm *v1.VirtualMachine) error {
	if err := s.syncVirtualMachineSpecAfterResourceUpdate(serviceID); err != nil {
		return err
	}
	if s == nil || s.kubevirtClient == nil || vm == nil {
		return nil
	}
	if vm.Status.PrintableStatus == v1.VirtualMachineStatusStopped {
		return nil
	}
	return s.kubevirtClient.VirtualMachine(vm.Namespace).Restart(context.Background(), vm.Name, &v1.RestartOptions{})
}

func isVMLiveMigrationUnsupportedPatchError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "cannot migrate") ||
		strings.Contains(message, "live migration") ||
		(strings.Contains(message, "pvc") && strings.Contains(message, "not shared")) ||
		strings.Contains(message, "readwritemany")
}

func (s *ServiceAction) GetVMLiveUpdateCapability(serviceID string) VMLiveUpdateCapability {
	capability := VMLiveUpdateCapability{}
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil || service == nil || !service.IsVM() {
		capability.HotUpdateReason = "当前组件不是虚拟机，不能使用虚拟机热更新。"
		return capability
	}
	if !s.isVMLiveUpdateClusterConfigured(context.Background()) {
		capability.HotUpdateReason = "集群没有开启 KubeVirt LiveMigrate 和 LiveUpdate，暂时不能热更新。"
		return capability
	}

	vm, err := s.getVirtualMachineByServiceID(serviceID)
	if err != nil || vm == nil {
		capability.HotUpdateReason = "当前虚拟机资源还没有准备完成，暂时不能热更新。"
		return capability
	}
	if vm.Status.PrintableStatus != v1.VirtualMachineStatusRunning {
		capability.HotUpdateReason = "虚拟机未运行，修改配置将在下次启动后生效。"
		return capability
	}
	if hasVMInstallerMediaAttached(vm) {
		capability.HotUpdateReason = vmInstallerMediaRemovalRequiredMessage
		return capability
	}

	vmi, err := s.getVirtualMachineInstanceByServiceID(serviceID)
	if err != nil || vmi == nil || vmi.Status.Phase != v1.Running {
		capability.HotUpdateReason = "虚拟机实例未运行，修改配置将在下次启动后生效。"
		return capability
	}
	if !isConditionTrue(vmi.Status.Conditions, v1.VirtualMachineInstanceIsMigratable) {
		capability.HotUpdateReason = liveMigratableMessage(vmi.Status.Conditions)
		return capability
	}
	if err := s.ensureVMLiveUpdateMigrationTargetAvailable(context.Background(), serviceID); err != nil {
		capability.HotUpdateReason = err.Error()
		return capability
	}

	deviceExtensions, err := s.loadVMRuntimeDeviceExtensionSetForCapability(serviceID)
	if err == nil {
		if extensionEnabled(deviceExtensions["vm_gpu_enabled"]) {
			capability.HotUpdateReason = "GPU 直通虚拟机暂不支持热更新，请停机后修改。"
			return capability
		}
		if extensionEnabled(deviceExtensions["vm_usb_enabled"]) {
			capability.HotUpdateReason = "USB 透传虚拟机暂不支持热更新，请停机后修改。"
			return capability
		}
	}

	if vm.Spec.Template != nil && vm.Spec.Template.Spec.Domain.CPU != nil && !supportsVMSocketCPUHotUpdate(vm.Spec.Template.Spec.Domain.CPU) {
		capability.MemoryHotUpdateSupported = true
		capability.HotUpdateReason = "当前虚拟机 CPU 拓扑不支持热更新，CPU 修改将在重启后生效。"
		return capability
	}

	capability.CPUHotUpdateSupported = true
	capability.MemoryHotUpdateSupported = true
	return capability
}

func (s *ServiceAction) getVirtualMachineInstanceByServiceID(serviceID string) (*v1.VirtualMachineInstance, error) {
	if s != nil && s.getVirtualMachineInstanceByServiceIDHook != nil {
		return s.getVirtualMachineInstanceByServiceIDHook(serviceID)
	}
	vmis, err := s.kubevirtClient.VirtualMachineInstance("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "service_id=" + serviceID,
	})
	if err != nil {
		return nil, err
	}
	if len(vmis.Items) == 0 {
		return nil, nil
	}
	vmi := vmis.Items[0]
	return &vmi, nil
}

func (s *ServiceAction) isVMLiveUpdateClusterConfigured(ctx context.Context) bool {
	if s != nil && s.isVMLiveUpdateClusterConfiguredHook != nil {
		return s.isVMLiveUpdateClusterConfiguredHook(ctx)
	}
	if s == nil || s.kubevirtClient == nil {
		return false
	}

	kvs, err := s.kubevirtClient.KubeVirt("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, kv := range kvs.Items {
		if kv.Spec.Configuration.VMRolloutStrategy == nil || *kv.Spec.Configuration.VMRolloutStrategy != v1.VMRolloutStrategyLiveUpdate {
			continue
		}
		for _, method := range kv.Spec.WorkloadUpdateStrategy.WorkloadUpdateMethods {
			if method == v1.WorkloadUpdateMethodLiveMigrate {
				return true
			}
		}
	}
	return false
}

func (s *ServiceAction) loadVMRuntimeDeviceExtensionSetForCapability(componentID string) (map[string]string, error) {
	if s != nil && s.loadVMRuntimeDeviceExtensionSetHook != nil {
		return s.loadVMRuntimeDeviceExtensionSetHook(componentID)
	}
	return s.loadVMRuntimeDeviceExtensionSet(componentID)
}

func (s *ServiceAction) loadVMRuntimeSpecExtensionSetForCapability(componentID string) (map[string]string, error) {
	if s != nil && s.loadVMRuntimeSpecExtensionSetHook != nil {
		return s.loadVMRuntimeSpecExtensionSetHook(componentID)
	}
	return s.loadVMRuntimeSpecExtensionSet(componentID)
}

func vmSocketsFromMilliCPU(cpuMilli int) (uint32, error) {
	if cpuMilli <= 0 {
		return 0, newVMLiveUpdateError(409, "vm cpu live update requires cpu greater than 0")
	}
	if cpuMilli%1000 != 0 {
		return 0, newVMLiveUpdateError(409, "vm cpu live update requires whole CPU cores")
	}
	return uint32(cpuMilli / 1000), nil
}

func supportsVMSocketCPUHotUpdate(cpu *v1.CPU) bool {
	if cpu == nil {
		return false
	}
	return cpu.Cores == 1 && cpu.Threads == 1 && cpu.MaxSockets > 0
}

func buildAlignedVMMemoryQuantity(memoryMB int) resource.Quantity {
	aligned := alignVMMemoryMi(memoryMB)
	return resource.MustParse(fmt.Sprintf("%dMi", aligned))
}

func alignVMMemoryMi(memoryMB int) int {
	if memoryMB <= 0 {
		return 0
	}
	const alignmentMi = 2
	remainder := memoryMB % alignmentMi
	if remainder == 0 {
		return memoryMB
	}
	return memoryMB + (alignmentMi - remainder)
}

func isConditionTrue(conditions []v1.VirtualMachineInstanceCondition, conditionType v1.VirtualMachineInstanceConditionType) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == "True"
		}
	}
	return false
}

func liveMigratableMessage(conditions []v1.VirtualMachineInstanceCondition) string {
	for _, condition := range conditions {
		if condition.Type == v1.VirtualMachineInstanceIsMigratable {
			if strings.TrimSpace(condition.Message) != "" {
				return condition.Message
			}
			break
		}
	}
	return "当前虚拟机不满足 LiveMigratable 条件，暂时不能热更新。"
}

func hasVMInstallerMediaAttached(vm *v1.VirtualMachine) bool {
	if vm == nil || vm.Spec.Template == nil {
		return false
	}
	for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if disk.Name == "vmimage" && disk.DiskDevice.CDRom != nil {
			return true
		}
	}
	return false
}

func (s *ServiceAction) ensureVMLiveUpdateMigrationTargetAvailable(ctx context.Context, serviceID string) error {
	if s == nil || s.kubeClient == nil || strings.TrimSpace(serviceID) == "" {
		return nil
	}
	launcherPod, err := s.getVMLauncherPodForLiveUpdate(ctx, serviceID)
	if err != nil || launcherPod == nil {
		return err
	}
	if strings.TrimSpace(launcherPod.Spec.NodeName) == "" {
		return nil
	}
	nodes, err := s.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if node.Name == launcherPod.Spec.NodeName || node.Spec.Unschedulable {
			continue
		}
		if !nodeMatchesVMLauncherSelector(node, launcherPod.Spec.NodeSelector) {
			continue
		}
		if !podToleratesNodeTaints(launcherPod.Spec.Tolerations, node.Spec.Taints) {
			continue
		}
		return nil
	}
	return newVMLiveUpdateError(409, "当前虚拟机没有可用的迁移目标节点，热更新依赖 LiveMigration。请至少准备一个额外节点满足 CPU/标签调度条件。")
}

func (s *ServiceAction) getVMLauncherPodForLiveUpdate(ctx context.Context, serviceID string) (*corev1.Pod, error) {
	pods, err := s.kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("service_id=%s,kubevirt.io=virt-launcher", serviceID),
	})
	if err != nil {
		return nil, err
	}
	var fallback *corev1.Pod
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase == corev1.PodRunning {
			return pod, nil
		}
		if fallback == nil && strings.TrimSpace(pod.Spec.NodeName) != "" {
			fallback = pod
		}
	}
	return fallback, nil
}

func nodeMatchesVMLauncherSelector(node *corev1.Node, selector map[string]string) bool {
	if node == nil {
		return false
	}
	for key, value := range selector {
		if node.Labels[key] != value {
			return false
		}
	}
	return true
}

func podToleratesNodeTaints(tolerations []corev1.Toleration, taints []corev1.Taint) bool {
	for _, taint := range taints {
		if taint.Effect != corev1.TaintEffectNoSchedule && taint.Effect != corev1.TaintEffectNoExecute {
			continue
		}
		if !toleratesTaint(tolerations, taint) {
			return false
		}
	}
	return true
}

func toleratesTaint(tolerations []corev1.Toleration, taint corev1.Taint) bool {
	for _, toleration := range tolerations {
		if toleration.Key != taint.Key {
			continue
		}
		if toleration.Effect != "" && toleration.Effect != taint.Effect {
			continue
		}
		operator := toleration.Operator
		if operator == "" {
			operator = corev1.TolerationOpEqual
		}
		if operator == corev1.TolerationOpExists {
			return true
		}
		if toleration.Value == taint.Value {
			return true
		}
	}
	return false
}

func extensionEnabled(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "true" || normalized == "1" || normalized == "yes" || normalized == "on"
}
