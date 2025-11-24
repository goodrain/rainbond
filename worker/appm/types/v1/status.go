// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package v1

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	v1 "kubevirt.io/api/core/v1"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// IsEmpty is empty
func (a *AppService) IsEmpty() bool {
	empty := len(a.pods) == 0
	if !empty {
		//The remaining pod is at the missing node and is considered closed successfully
		for _, pod := range a.pods {
			if !IsPodNodeLost(pod) {
				return false
			}
		}
		return true
	}
	return empty
}

// IsClosed is closed
func (a *AppService) IsClosed() bool {
	if a.IsCustomComponent() {
		return a.workload == nil
	}
	if a.IsThirdComponent() {
		if a.endpoints == nil || len(a.endpoints) == 0 {
			return true
		}
	} else {
		if a.IsEmpty() && a.statefulset == nil && a.deployment == nil && a.cronjob == nil && a.betaCronJob == nil {
			return true
		}
		if a.IsEmpty() && a.statefulset != nil && a.statefulset.ResourceVersion == "" {
			return true
		}
		if a.IsEmpty() && a.deployment != nil && a.deployment.ResourceVersion == "" {
			return true
		}
		if a.IsEmpty() && a.virtualmachine != nil && a.virtualmachine.ResourceVersion == "" {
			return true
		}
	}
	return false
}

var (
	//RUNNING if stateful or deployment exist and ready pod number is equal to the service Replicas
	RUNNING = "running"
	//CLOSED if app service is not in store
	CLOSED = "closed"
	//STARTING if stateful or deployment exist and ready pod number is less than service Replicas
	STARTING = "starting"
	//STOPPING if stateful and deployment is nil and pod number is not 0
	STOPPING = "stopping"
	//ABNORMAL if stateful or deployment exist and ready pod number is less than service Replicas and all pod status is Error
	ABNORMAL = "abnormal"
	//SOMEABNORMAL if stateful or deployment exist and ready pod number is less than service Replicas and some pod status is Error
	SOMEABNORMAL = "some_abnormal"
	//UNKNOW indeterminacy status
	UNKNOW = "unknow"
	//UPGRADE if store have more than 1 app service
	UPGRADE = "upgrade"
	//BUILDING app service is building
	BUILDING = "building"
	//BUILDEFAILURE app service is build failure
	BUILDEFAILURE = "build_failure"
	//UNDEPLOY init status
	UNDEPLOY = "undeploy"
	//SUCCEEDED if job and cronjob is succeeded
	SUCCEEDED = "succeeded"
	//PAUSED -
	PAUSED = "paused"
)

func conversionThirdComponent(obj runtime.Object) *v1alpha1.ThirdComponent {
	if third, ok := obj.(*v1alpha1.ThirdComponent); ok {
		return third
	}
	if struc, ok := obj.(*unstructured.Unstructured); ok {
		data, _ := struc.MarshalJSON()
		var third v1alpha1.ThirdComponent
		if err := json.Unmarshal(data, &third); err != nil {
			logrus.Errorf("unmarshal object to ThirdComponent failure")
			return nil
		}
		return &third
	}
	return nil
}

// GetServiceStatus get service status
func (a *AppService) GetServiceStatus() string {
	logrus.Debugf("[GetServiceStatus] Calculating status for service: %s (alias=%s, replicas=%d)",
		a.ServiceID, a.ServiceAlias, a.Replicas)

	if a.ServiceType == TypeKubeBlocks {
		logrus.Debugf("[GetServiceStatus] Service %s is KubeBlocks type, returning RUNNING", a.ServiceID)
		return RUNNING
	}

	if a.IsThirdComponent() {
		logrus.Debugf("[GetServiceStatus] Service %s is third-party component", a.ServiceID)
		endpoints := a.GetEndpoints(false)
		if len(endpoints) == 0 {
			return CLOSED
		}
		var readyEndpointSize int
		for _, ed := range endpoints {
			for _, s := range ed.Subsets {
				readyEndpointSize += len(s.Addresses)
			}
		}
		if readyEndpointSize > 0 {
			return RUNNING
		}
		return ABNORMAL
	}
	//TODO: support custom component status
	if a.IsCustomComponent() {
		if a.workload != nil {
			switch a.workload.GetObjectKind().GroupVersionKind().Kind {
			case "ThirdComponent":
				third := conversionThirdComponent(a.workload)
				if third != nil {
					switch third.Status.Phase {
					case v1alpha1.ComponentFailed:
						return ABNORMAL
					case v1alpha1.ComponentRunning:
						return RUNNING
					case v1alpha1.ComponentPending:
						return STARTING
					}
				}
				return RUNNING
			default:
				return RUNNING
			}
		}
		return CLOSED
	}
	if a == nil {
		logrus.Warnf("[GetServiceStatus] AppService is nil, returning CLOSED")
		return CLOSED
	}
	if a.IsClosed() {
		logrus.Infof("[GetServiceStatus] Service %s is closed (IsClosed=true), returning CLOSED", a.ServiceID)
		return CLOSED
	}

	// Log current state for debugging
	logrus.Debugf("[GetServiceStatus] Service %s state: deployment=%v, statefulset=%v, pods=%d, replicas=%d",
		a.ServiceID, a.deployment != nil, a.statefulset != nil, len(a.pods), a.Replicas)
	if a.deployment != nil {
		logrus.Debugf("[GetServiceStatus] Deployment status: ReadyReplicas=%d, Replicas=%d",
			a.deployment.Status.ReadyReplicas, a.deployment.Status.Replicas)
	}
	if a.statefulset != nil {
		logrus.Debugf("[GetServiceStatus] StatefulSet status: ReadyReplicas=%d, Replicas=%d",
			a.statefulset.Status.ReadyReplicas, a.statefulset.Status.Replicas)
	}

	if a.virtualmachine != nil {
		if a.virtualmachine.Status.PrintableStatus == v1.VirtualMachineStatusPaused {
			return PAUSED
		}
		succeed := 0
		failed := 0
		for _, po := range a.pods {
			if po.Status.Phase == "Succeeded" {
				succeed++
			}
			if po.Status.Phase == "Failed" {
				failed++
			}
			if po.Status.Phase == "Pending" {
				failed++
			}
		}
		if len(a.pods) == succeed {
			return SUCCEEDED
		}
		if failed > 0 {
			return ABNORMAL
		}
		return RUNNING
	}
	if a.cronjob != nil {
		succeed := 0
		failed := 0
		if len(a.pods) == 0 {
			return RUNNING
		}
		for _, po := range a.pods {
			if po.Status.Phase == "Succeeded" {
				succeed++
			}
			if po.Status.Phase == "Failed" {
				failed++
			}
		}
		if len(a.pods) == succeed {
			return RUNNING
		}
		if failed > 0 {
			return ABNORMAL
		}
		return RUNNING
	}

	if a.job != nil {
		succeed := 0
		failed := 0
		for _, po := range a.pods {
			if po.Status.Phase == "Succeeded" {
				succeed++
			}
			if po.Status.Phase == "Failed" {
				failed++
			}
		}
		if len(a.pods) == succeed {
			return SUCCEEDED
		}
		if failed > 0 {
			return ABNORMAL
		}
		return RUNNING
	}

	if a.statefulset == nil && a.deployment == nil && len(a.pods) > 0 {
		logrus.Infof("[GetServiceStatus] Service %s: no deployment/statefulset but has %d pods, returning STOPPING",
			a.ServiceID, len(a.pods))
		return STOPPING
	}
	if (a.statefulset != nil || a.deployment != nil) && len(a.pods) < a.Replicas {
		logrus.Infof("[GetServiceStatus] Service %s: has workload but pods(%d) < replicas(%d), returning STARTING",
			a.ServiceID, len(a.pods), a.Replicas)
		return STARTING
	}
	if a.statefulset != nil && a.statefulset.Status.ReadyReplicas >= int32(a.Replicas) {
		if a.UpgradeComlete() {
			logrus.Debugf("[GetServiceStatus] Service %s: StatefulSet ready and upgrade complete, returning RUNNING", a.ServiceID)
			return RUNNING
		}
		logrus.Infof("[GetServiceStatus] Service %s: StatefulSet ready but upgrade incomplete, returning UPGRADE", a.ServiceID)
		return UPGRADE
	}
	if a.deployment != nil && a.deployment.Status.ReadyReplicas >= int32(a.Replicas) {
		if a.UpgradeComlete() {
			logrus.Debugf("[GetServiceStatus] Service %s: Deployment ready and upgrade complete, returning RUNNING", a.ServiceID)
			return RUNNING
		}
		logrus.Infof("[GetServiceStatus] Service %s: Deployment ready but upgrade incomplete, returning UPGRADE", a.ServiceID)
		return UPGRADE
	}

	if a.deployment != nil && (a.deployment.Status.ReadyReplicas < int32(a.Replicas) && a.deployment.Status.ReadyReplicas != 0) {
		logrus.Infof("[GetServiceStatus] Service %s: Deployment ReadyReplicas(%d) < Replicas(%d) but not 0",
			a.ServiceID, a.deployment.Status.ReadyReplicas, a.Replicas)
		if isHaveTerminatedContainer(a.pods) {
			logrus.Infof("[GetServiceStatus] Service %s: has terminated containers, returning SOMEABNORMAL", a.ServiceID)
			return SOMEABNORMAL
		}
		if isHaveNormalTerminatedContainer(a.pods) {
			logrus.Infof("[GetServiceStatus] Service %s: has normal terminated containers, returning STOPPING", a.ServiceID)
			return STOPPING
		}
		logrus.Infof("[GetServiceStatus] Service %s: returning STARTING", a.ServiceID)
		return STARTING
	}
	if a.deployment != nil && a.deployment.Status.ReadyReplicas == 0 {
		logrus.Warnf("[GetServiceStatus] Service %s: Deployment has 0 ReadyReplicas", a.ServiceID)
		if isHaveTerminatedContainer(a.pods) {
			logrus.Errorf("[GetServiceStatus] Service %s: has terminated containers, returning ABNORMAL", a.ServiceID)
			return ABNORMAL
		}
		if isHaveNormalTerminatedContainer(a.pods) {
			logrus.Infof("[GetServiceStatus] Service %s: has normal terminated containers, returning STOPPING", a.ServiceID)
			return STOPPING
		}
		logrus.Infof("[GetServiceStatus] Service %s: returning STARTING", a.ServiceID)
		return STARTING
	}
	if a.statefulset != nil && (a.statefulset.Status.ReadyReplicas < int32(a.Replicas) && a.statefulset.Status.ReadyReplicas != 0) {
		logrus.Infof("[GetServiceStatus] Service %s: StatefulSet ReadyReplicas(%d) < Replicas(%d) but not 0",
			a.ServiceID, a.statefulset.Status.ReadyReplicas, a.Replicas)
		if isHaveTerminatedContainer(a.pods) {
			logrus.Infof("[GetServiceStatus] Service %s: has terminated containers, returning SOMEABNORMAL", a.ServiceID)
			return SOMEABNORMAL
		}
		if isHaveNormalTerminatedContainer(a.pods) {
			logrus.Infof("[GetServiceStatus] Service %s: has normal terminated containers, returning STOPPING", a.ServiceID)
			return STOPPING
		}
		logrus.Infof("[GetServiceStatus] Service %s: returning STARTING", a.ServiceID)
		return STARTING
	}
	if a.statefulset != nil && a.statefulset.Status.ReadyReplicas == 0 {
		logrus.Warnf("[GetServiceStatus] Service %s: StatefulSet has 0 ReadyReplicas", a.ServiceID)
		if isHaveTerminatedContainer(a.pods) {
			logrus.Errorf("[GetServiceStatus] Service %s: has terminated containers, returning ABNORMAL", a.ServiceID)
			return ABNORMAL
		}
		if isHaveNormalTerminatedContainer(a.pods) {
			logrus.Infof("[GetServiceStatus] Service %s: has normal terminated containers, returning STOPPING", a.ServiceID)
			return STOPPING
		}
		logrus.Infof("[GetServiceStatus] Service %s: returning STARTING", a.ServiceID)
		return STARTING
	}
	logrus.Warnf("[GetServiceStatus] Service %s: no condition matched, returning UNKNOW", a.ServiceID)
	return UNKNOW
}

func isHaveTerminatedContainer(pods []*corev1.Pod) bool {
	for _, pod := range pods {
		for _, con := range pod.Status.ContainerStatuses {
			//have Terminated container
			if con.State.Terminated != nil && con.State.Terminated.ExitCode != 0 {
				return true
			}
			if con.LastTerminationState.Terminated != nil {
				return true
			}
		}
	}
	return false
}
func isHaveNormalTerminatedContainer(pods []*corev1.Pod) bool {
	for _, pod := range pods {
		for _, con := range pod.Status.ContainerStatuses {
			//have Terminated container
			if con.State.Terminated != nil && con.State.Terminated.ExitCode == 0 {
				return true
			}
		}
	}
	return false
}

// Ready Whether ready
func (a *AppService) Ready() bool {
	if a.statefulset != nil {
		if a.statefulset.Status.ReadyReplicas >= int32(a.Replicas) {
			return true
		}
	}
	if a.deployment != nil {
		if a.deployment.Status.ReadyReplicas >= int32(a.Replicas) {
			return true
		}
	}
	if a.virtualmachine != nil {
		if a.virtualmachine.Status.Ready {
			return true
		}
	}
	if a.job != nil {
		return true
	}
	if a.cronjob != nil {
		return true
	}
	if a.betaCronJob != nil {
		return true
	}
	return false
}

// IsWaitting service status is waitting
// init container init-probe is running
func (a *AppService) IsWaitting() bool {
	var initcontainer []corev1.Container
	if a.statefulset != nil {
		initcontainer = a.statefulset.Spec.Template.Spec.InitContainers
		if len(initcontainer) == 0 {
			return false
		}
	}
	if a.deployment != nil {
		initcontainer = a.deployment.Spec.Template.Spec.InitContainers
		if len(initcontainer) == 0 {
			return false
		}
	}
	var haveProbeInitContainer bool
	for _, init := range initcontainer {
		if init.Image == GetProbeMeshImageName() || init.Image == GetOnlineProbeMeshImageName() {
			haveProbeInitContainer = true
			break
		}
	}
	if haveProbeInitContainer {
		if len(a.pods) == 0 {
			return true
		}
		firstPod := a.pods[0]
		for _, initconteir := range firstPod.Status.InitContainerStatuses {
			if initconteir.Image == GetProbeMeshImageName() || initconteir.Image == GetOnlineProbeMeshImageName() {
				if initconteir.State.Terminated == nil || initconteir.State.Terminated.ExitCode != 0 {
					return true
				}
			}
		}
	}
	return false
}

// GetReadyReplicas get already ready pod number
func (a *AppService) GetReadyReplicas() int32 {
	if a.statefulset != nil {
		return a.statefulset.Status.ReadyReplicas
	}
	if a.deployment != nil {
		return a.deployment.Status.ReadyReplicas
	}
	return 0
}

// GetRunningVersion get running version
func (a *AppService) GetRunningVersion() string {
	if a.statefulset != nil {
		return a.statefulset.Labels["version"]
	}
	if a.deployment != nil {
		return a.deployment.Labels["version"]
	}
	return ""
}

// UpgradeComlete upgrade comlete
func (a *AppService) UpgradeComlete() bool {
	for _, pod := range a.pods {
		if pod.Labels["version"] != a.DeployVersion {
			return false
		}
	}
	return a.Ready()
}

// AbnormalInfo pod Abnormal info
// Record the container exception exit information in pod.
type AbnormalInfo struct {
	ServiceID     string    `json:"service_id"`
	TenantID      string    `json:"tenant_id"`
	ServiceAlias  string    `json:"service_alias"`
	PodName       string    `json:"pod_name"`
	ContainerName string    `json:"container_name"`
	Reason        string    `json:"reson"`
	Message       string    `json:"message"`
	CreateTime    time.Time `json:"create_time"`
	Count         int       `json:"count"`
}

// Hash get AbnormalInfo hash
func (a AbnormalInfo) Hash() string {
	hash := sha256.New()
	hash.Write([]byte(a.ServiceID + a.ServiceAlias))
	return fmt.Sprintf("%x", hash.Sum(nil))
}
func (a AbnormalInfo) String() string {
	return fmt.Sprintf("ServiceID: %s;ServiceAlias: %s;PodName: %s ; ContainerName: %s; Reason: %s; Message: %s",
		a.ServiceID, a.ServiceAlias, a.PodName, a.ContainerName, a.Reason, a.Message)
}
