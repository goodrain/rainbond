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
	"fmt"
	"time"

	"github.com/goodrain/rainbond/db/model"

	corev1 "k8s.io/api/core/v1"
)

//IsEmpty is empty
func (a *AppService) IsEmpty() bool {
	empty := len(a.pods) == 0
	return empty
}

//IsClosed is closed
func (a *AppService) IsClosed() bool {
	if a.ServiceKind == model.ServiceKindThirdParty && (a.endpoints == nil || len(a.endpoints) == 0) {
		return true
	}
	if a.IsEmpty() && a.statefulset == nil && a.deployment == nil {
		return true
	}
	if a.IsEmpty() && a.statefulset != nil && a.statefulset.ResourceVersion == "" {
		return true
	}
	if a.IsEmpty() && a.deployment != nil && a.deployment.ResourceVersion == "" {
		return true
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
)

//GetServiceStatus get service status
func (a *AppService) GetServiceStatus() string {
	if a.ServiceKind == model.ServiceKindThirdParty {
		var readyEndpointSize int
		endpoints := a.GetEndpoints()
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
	if a == nil {
		return CLOSED
	}
	if a.IsClosed() {
		return CLOSED
	}
	if a.statefulset == nil && a.deployment == nil && len(a.pods) > 0 {
		return STOPPING
	}
	if (a.statefulset != nil || a.deployment != nil) && len(a.pods) < a.Replicas {
		return STARTING
	}
	if a.statefulset != nil && a.statefulset.Status.ReadyReplicas >= int32(a.Replicas) {
		if a.UpgradeComlete() {
			return RUNNING
		}
		return UPGRADE
	}
	if a.deployment != nil && a.deployment.Status.ReadyReplicas >= int32(a.Replicas) {
		if a.UpgradeComlete() {
			return RUNNING
		}
		return UPGRADE
	}

	if a.deployment != nil && (a.deployment.Status.ReadyReplicas < int32(a.Replicas) && a.deployment.Status.ReadyReplicas != 0) {
		if isHaveTerminatedContainer(a.pods) {
			return SOMEABNORMAL
		}
		if isHaveNormalTerminatedContainer(a.pods) {
			return STOPPING
		}
		return STARTING
	}
	if a.deployment != nil && a.deployment.Status.ReadyReplicas == 0 {
		if isHaveTerminatedContainer(a.pods) {
			return ABNORMAL
		}
		if isHaveNormalTerminatedContainer(a.pods) {
			return STOPPING
		}
		return STARTING
	}
	if a.statefulset != nil && (a.statefulset.Status.ReadyReplicas < int32(a.Replicas) && a.statefulset.Status.ReadyReplicas != 0) {
		if isHaveTerminatedContainer(a.pods) {
			return SOMEABNORMAL
		}
		if isHaveNormalTerminatedContainer(a.pods) {
			return STOPPING
		}
		return STARTING
	}
	if a.statefulset != nil && a.statefulset.Status.ReadyReplicas == 0 {
		if isHaveTerminatedContainer(a.pods) {
			return ABNORMAL
		}
		if isHaveNormalTerminatedContainer(a.pods) {
			return STOPPING
		}
		return STARTING
	}
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

//Ready Whether ready
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
	return false
}

//GetReadyReplicas get already ready pod number
func (a *AppService) GetReadyReplicas() int32 {
	if a.statefulset != nil {
		return a.statefulset.Status.ReadyReplicas
	}
	if a.deployment != nil {
		return a.deployment.Status.ReadyReplicas
	}
	return 0
}

//GetRunningVersion get running version
func (a *AppService) GetRunningVersion() string {
	if a.statefulset != nil {
		return a.statefulset.Labels["version"]
	}
	if a.deployment != nil {
		return a.deployment.Labels["version"]
	}
	return ""
}

//UpgradeComlete upgrade comlete
func (a *AppService) UpgradeComlete() bool {
	for _, pod := range a.pods {
		if pod.Labels["version"] != a.DeployVersion {
			return false
		}
	}
	return a.Ready()
}

//AbnormalInfo pod Abnormal info
//Record the container exception exit information in pod.
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

//Hash get AbnormalInfo hash
func (a AbnormalInfo) Hash() string {
	hash := sha256.New()
	hash.Write([]byte(a.ServiceID + a.ServiceAlias))
	return fmt.Sprintf("%x", hash.Sum(nil))
}
func (a AbnormalInfo) String() string {
	return fmt.Sprintf("ServiceID: %s;ServiceAlias: %s;PodName: %s ; ContainerName: %s; Reason: %s; Message: %s",
		a.ServiceID, a.ServiceAlias, a.PodName, a.ContainerName, a.Reason, a.Message)
}
