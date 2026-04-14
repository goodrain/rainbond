// RAINBOND, Application Management Platform
// Copyright (C) 2022-2022 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package handler

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// ReviseAttributeAffinityByArch -
func (s *ServiceAction) ReviseAttributeAffinityByArch(attributeValue string, arch string) (string, error) {
	var affinity corev1.Affinity
	if attributeValue != "" {
		AffinityAttributeJSON, err := yaml.YAMLToJSON([]byte(attributeValue))
		if err != nil {
		}
		err = json.Unmarshal(AffinityAttributeJSON, &affinity)
		if err != nil {
			return "", err
		}
		if affinity.NodeAffinity == nil {
			affinity.NodeAffinity = &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{arch},
							},
						},
					},
					},
				},
			}
		} else {
			if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{arch},
							},
						},
					},
					},
				}
			} else {
				affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{arch},
							},
						},
					},
				}
			}
		}
	} else {

		affinity = corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{arch},
							},
						},
					},
					},
				},
			},
		}
	}
	affinityByte, err := yaml.Marshal(affinity)
	return string(affinityByte), err
}

// GetK8sAttribute -
func (s *ServiceAction) GetK8sAttribute(componentID, name string) (*dbmodel.ComponentK8sAttributes, error) {
	return s.getDBManager().ComponentK8sAttributeDao().GetByComponentIDAndName(componentID, name)
}

// CreateK8sAttribute -
func (s *ServiceAction) CreateK8sAttribute(tenantID, componentID string, k8sAttr *apimodel.ComponentK8sAttribute) error {
	if err := s.getDBManager().ComponentK8sAttributeDao().AddModel(k8sAttr.DbModel(tenantID, componentID)); err != nil {
		return err
	}
	return s.syncVMRuntimeDevicesForAttribute(componentID, k8sAttr.Name)
}

// UpdateK8sAttribute -
func (s *ServiceAction) UpdateK8sAttribute(componentID string, k8sAttributes *apimodel.ComponentK8sAttribute) error {
	attr, err := s.getDBManager().ComponentK8sAttributeDao().GetByComponentIDAndName(componentID, k8sAttributes.Name)
	if err != nil {
		return err
	}
	attr.AttributeValue = k8sAttributes.AttributeValue
	if err := s.getDBManager().ComponentK8sAttributeDao().UpdateModel(attr); err != nil {
		return err
	}
	return s.syncVMRuntimeDevicesForAttribute(componentID, k8sAttributes.Name)
}

// DeleteK8sAttribute -
func (s *ServiceAction) DeleteK8sAttribute(componentID, name string) error {
	if err := s.getDBManager().ComponentK8sAttributeDao().DeleteByComponentIDAndName(componentID, name); err != nil {
		return err
	}
	return s.syncVMRuntimeDevicesForAttribute(componentID, name)
}

var vmRuntimeDeviceAttributeNames = []string{
	"vm_gpu_enabled",
	"vm_gpu_resources",
	"vm_gpu_count",
	"vm_usb_enabled",
	"vm_usb_resources",
}

var vmRuntimeSpecAttributeNames = []string{
	"vm_network_mode",
	"vm_network_name",
	"vm_fixed_ip",
	"vm_gateway",
	"vm_dns_servers",
	"vm_os_family",
	"vm_os_name",
}

func (s *ServiceAction) getDBManager() db.Manager {
	if s.dbmanager != nil {
		return s.dbmanager
	}
	return db.GetManager()
}

func isVMRuntimeDeviceAttribute(name string) bool {
	for _, candidate := range vmRuntimeDeviceAttributeNames {
		if name == candidate {
			return true
		}
	}
	return false
}

func isVMRuntimeSpecAttribute(name string) bool {
	for _, candidate := range vmRuntimeSpecAttributeNames {
		if name == candidate {
			return true
		}
	}
	return false
}

func (s *ServiceAction) syncVMRuntimeDevicesForAttribute(componentID, name string) error {
	switch {
	case isVMRuntimeDeviceAttribute(name):
		return s.syncVMRuntimeDevices(componentID)
	case isVMRuntimeSpecAttribute(name):
		return s.syncVirtualMachineSpecForService(componentID)
	default:
		return nil
	}
}

func (s *ServiceAction) syncVMRuntimeDevices(componentID string) error {
	service, err := s.getDBManager().TenantServiceDao().GetServiceByID(componentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if service == nil || !service.IsVM() {
		return nil
	}
	vm, err := s.getVirtualMachineByServiceID(componentID)
	if err != nil || vm == nil {
		return err
	}
	extensionSet, err := s.loadVMRuntimeDeviceExtensionSet(componentID)
	if err != nil {
		return err
	}
	return s.syncVMRuntimeDeviceConfig(vm, extensionSet)
}

func (s *ServiceAction) loadVMRuntimeDeviceExtensionSet(componentID string) (map[string]string, error) {
	extensionSet := make(map[string]string, len(vmRuntimeDeviceAttributeNames))
	dao := s.getDBManager().ComponentK8sAttributeDao()
	for _, name := range vmRuntimeDeviceAttributeNames {
		attr, err := dao.GetByComponentIDAndName(componentID, name)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		if attr != nil && attr.AttributeValue != "" {
			extensionSet[name] = attr.AttributeValue
		}
	}
	return extensionSet, nil
}

func (s *ServiceAction) syncVMRuntimeDeviceConfig(vm *kubevirtv1.VirtualMachine, extensionSet map[string]string) error {
	updatedVM, changed := applyVMRuntimeDeviceConfig(vm, extensionSet)
	if !changed {
		return nil
	}
	_, err := s.kubevirtClient.VirtualMachine(updatedVM.Namespace).Update(context.Background(), updatedVM, metav1.UpdateOptions{})
	return err
}

func applyVMRuntimeDeviceConfig(vm *kubevirtv1.VirtualMachine, extensionSet map[string]string) (*kubevirtv1.VirtualMachine, bool) {
	if vm == nil || vm.Spec.Template == nil {
		return vm, false
	}
	gpus, hostDevices := conversion.BuildVMAccelerationDevices(extensionSet)
	devices := vm.Spec.Template.Spec.Domain.Devices
	if reflect.DeepEqual(devices.GPUs, gpus) && reflect.DeepEqual(devices.HostDevices, hostDevices) {
		return vm, false
	}
	updatedVM := vm.DeepCopy()
	updatedVM.Spec.Template.Spec.Domain.Devices.GPUs = gpus
	updatedVM.Spec.Template.Spec.Domain.Devices.HostDevices = hostDevices
	return updatedVM, true
}

func (s *ServiceAction) syncVirtualMachineSpecForService(serviceID string) error {
	service, err := s.getDBManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if service == nil || !service.IsVM() {
		return nil
	}
	existingVM, err := s.getVirtualMachineByServiceID(serviceID)
	if err != nil || existingVM == nil {
		return err
	}
	extensionSet, err := s.loadVMRuntimeSpecExtensionSet(serviceID)
	if err != nil {
		return err
	}
	if shouldDeferVirtualMachineSpecSync(extensionSet) {
		return nil
	}
	desiredVM, err := s.buildDesiredVirtualMachine(serviceID)
	if err != nil || desiredVM == nil {
		return err
	}
	return s.syncVirtualMachineSpec(existingVM, desiredVM)
}

func (s *ServiceAction) loadVMRuntimeSpecExtensionSet(componentID string) (map[string]string, error) {
	extensionSet := make(map[string]string, len(vmRuntimeSpecAttributeNames))
	dao := s.getDBManager().ComponentK8sAttributeDao()
	for _, name := range vmRuntimeSpecAttributeNames {
		attr, err := dao.GetByComponentIDAndName(componentID, name)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		if attr != nil && attr.AttributeValue != "" {
			extensionSet[name] = attr.AttributeValue
		}
	}
	return extensionSet, nil
}

func shouldDeferVirtualMachineSpecSync(extensionSet map[string]string) bool {
	return strings.EqualFold(strings.TrimSpace(extensionSet["vm_network_mode"]), "fixed") &&
		strings.TrimSpace(extensionSet["vm_fixed_ip"]) == ""
}

func (s *ServiceAction) buildDesiredVirtualMachine(serviceID string) (*kubevirtv1.VirtualMachine, error) {
	appService, err := conversion.InitAppService(false, s.getDBManager(), serviceID, nil)
	if err != nil {
		return nil, err
	}
	return appService.GetVirtualMachine(), nil
}

func (s *ServiceAction) syncVirtualMachineSpec(existingVM, desiredVM *kubevirtv1.VirtualMachine) error {
	updatedVM, changed := applyVirtualMachineSpec(existingVM, desiredVM)
	if !changed {
		return nil
	}
	_, err := s.kubevirtClient.VirtualMachine(updatedVM.Namespace).Update(context.Background(), updatedVM, metav1.UpdateOptions{})
	return err
}

func applyVirtualMachineSpec(existingVM, desiredVM *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, bool) {
	if existingVM == nil || desiredVM == nil {
		return existingVM, false
	}
	if reflect.DeepEqual(existingVM.Spec, desiredVM.Spec) {
		return existingVM, false
	}
	updatedVM := existingVM.DeepCopy()
	updatedVM.Spec = desiredVM.Spec
	return updatedVM, true
}
