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
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

//SetUpgradePatch create and set upgrade pathch for deployment and statefulset
func (a *AppService) SetUpgradePatch(new *AppService) error {
	if a.statefulset != nil && new.statefulset != nil {
		// If the controller originally had a startup sequence, then the startup sequence needs to be updated
		if isContainsBootSequence(a.statefulset.Spec.Template.Spec.InitContainers) &&
			!isContainsBootSequence(new.statefulset.Spec.Template.Spec.InitContainers) && new.BootSeqContainer != nil {
			new.statefulset.Spec.Template.Spec.InitContainers = append(new.statefulset.Spec.Template.Spec.InitContainers, *new.BootSeqContainer)
		}
		statefulsetPatch, err := getStatefulsetModifiedConfiguration(a.statefulset, new.statefulset)
		if err != nil {
			return err
		}
		if len(statefulsetPatch) == 0 {
			return fmt.Errorf("no upgrade")
		}
		logrus.Debugf("stateful patch %s", string(statefulsetPatch))
		new.UpgradePatch["statefulset"] = statefulsetPatch
	}
	if a.deployment != nil && new.deployment != nil {
		// If the controller originally had a startup sequence, then the startup sequence needs to be updated
		if isContainsBootSequence(a.deployment.Spec.Template.Spec.InitContainers) &&
			!isContainsBootSequence(new.deployment.Spec.Template.Spec.InitContainers) && new.BootSeqContainer != nil {
			new.deployment.Spec.Template.Spec.InitContainers = append(new.deployment.Spec.Template.Spec.InitContainers, *new.BootSeqContainer)
		}
		deploymentPatch, err := getDeploymentModifiedConfiguration(a.deployment, new.deployment)
		if err != nil {
			return err
		}
		if len(deploymentPatch) == 0 {
			return fmt.Errorf("no upgrade")
		}
		new.UpgradePatch["deployment"] = deploymentPatch
	}
	//update cache app service base info by new app service
	a.AppServiceBase = new.AppServiceBase
	return nil
}

//EncodeNode encode node
type EncodeNode struct {
	body  []byte
	value []byte
	Field map[string]EncodeNode
}

//UnmarshalJSON custom yaml decoder
func (e *EncodeNode) UnmarshalJSON(code []byte) error {
	e.body = code
	if len(code) < 1 {
		return nil
	}
	if code[0] != '{' {
		e.value = code
		return nil
	}
	var fields = make(map[string]EncodeNode)
	if err := json.Unmarshal(code, &fields); err != nil {
		return err
	}
	e.Field = fields
	return nil
}

//MarshalJSON custom marshal json
func (e *EncodeNode) MarshalJSON() ([]byte, error) {
	if e.value != nil {
		return e.value, nil
	}
	if e.Field != nil {
		var buffer = bytes.NewBufferString("{")
		count := 0
		length := len(e.Field)
		for k, v := range e.Field {
			buffer.WriteString(fmt.Sprintf("\"%s\":", k))
			value, err := v.MarshalJSON()
			if err != nil {
				return nil, err
			}
			buffer.Write(value)
			count++
			if count < length {
				buffer.WriteString(",")
			}
		}
		buffer.WriteByte('}')
		return buffer.Bytes(), nil
	}
	if e.body != nil {
		return e.body, nil
	}
	return nil, fmt.Errorf("marshal error")
}

//Contrast Compare value
func (e *EncodeNode) Contrast(endpoint *EncodeNode) bool {
	return util.BytesSliceEqual(e.value, endpoint.value)
}

//GetChange get change fields
func (e *EncodeNode) GetChange(endpoint *EncodeNode) *EncodeNode {
	if util.BytesSliceEqual(e.body, endpoint.body) {
		return nil
	}
	return getChange(*e, *endpoint)
}

func getChange(old, new EncodeNode) *EncodeNode {
	var result EncodeNode
	if util.BytesSliceEqual(old.body, new.body) {
		return nil
	}
	if old.Field == nil && new.Field == nil {
		if !util.BytesSliceEqual(old.value, new.value) {
			result.value = new.value
			return &result
		}
	}
	for k, v := range new.Field {
		if result.Field == nil {
			result.Field = make(map[string]EncodeNode)
		}
		if value := getChange(old.Field[k], v); value != nil {
			result.Field[k] = *value
		}
	}

	// keep the modifies of removed field
	for k, v := range old.Field {
		if _, ok := new.Field[k]; !ok {
			if result.Field == nil {
				result.Field = make(map[string]EncodeNode)
			}
			if _, ok := result.Field[k]; !ok {
				if v.body[0] == '[' {
					result.Field[k] = EncodeNode{
						body: []byte("[]"),
					}
				} else {
					result.Field[k] = EncodeNode{
						body: []byte("\"\""),
					}
				}
			}
		}
	}

	return &result
}

//stateful label can not be patch
func getStatefulsetModifiedConfiguration(old, new *v1.StatefulSet) ([]byte, error) {
	old.Status = new.Status
	oldNeed := getStatefulsetAllowFields(old)
	newNeed := getStatefulsetAllowFields(new)
	return getchange(oldNeed, newNeed)
}

// updates to statefulset spec for fields other than 'replicas', 'template', and 'updateStrategy' are forbidden.
func getStatefulsetAllowFields(s *v1.StatefulSet) *v1.StatefulSet {
	return &v1.StatefulSet{
		Spec: v1.StatefulSetSpec{
			Replicas: s.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes:          s.Spec.Template.Spec.Volumes,
					InitContainers:   s.Spec.Template.Spec.InitContainers,
					Containers:       s.Spec.Template.Spec.Containers,
					ImagePullSecrets: s.Spec.Template.Spec.ImagePullSecrets,
					NodeSelector:     s.Spec.Template.Spec.NodeSelector,
					Tolerations:      s.Spec.Template.Spec.Tolerations,
					Affinity:         s.Spec.Template.Spec.Affinity,
					HostAliases:      s.Spec.Template.Spec.HostAliases,
					Hostname:         s.Spec.Template.Spec.Hostname,
					NodeName:         s.Spec.Template.Spec.NodeName,
					HostNetwork:      s.Spec.Template.Spec.HostNetwork,
					SchedulerName:    s.Spec.Template.Spec.SchedulerName,
				},
			},
			UpdateStrategy: s.Spec.UpdateStrategy,
		},
		ObjectMeta: s.Spec.Template.ObjectMeta,
	}
}

//deployment label can not be patch
func getDeploymentModifiedConfiguration(old, new *v1.Deployment) ([]byte, error) {
	old.Status = new.Status
	oldNeed := getDeploymentAllowFields(old)
	newNeed := getDeploymentAllowFields(new)
	return getchange(oldNeed, newNeed)
}

// updates to deployment spec for fields other than 'replicas' and 'template' are forbidden.
func getDeploymentAllowFields(d *v1.Deployment) *v1.Deployment {
	return &v1.Deployment{
		Spec: v1.DeploymentSpec{
			Replicas: d.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes:          d.Spec.Template.Spec.Volumes,
					InitContainers:   d.Spec.Template.Spec.InitContainers,
					Containers:       d.Spec.Template.Spec.Containers,
					ImagePullSecrets: d.Spec.Template.Spec.ImagePullSecrets,
					NodeSelector:     d.Spec.Template.Spec.NodeSelector,
					Tolerations:      d.Spec.Template.Spec.Tolerations,
					Affinity:         d.Spec.Template.Spec.Affinity,
					HostAliases:      d.Spec.Template.Spec.HostAliases,
					Hostname:         d.Spec.Template.Spec.Hostname,
					NodeName:         d.Spec.Template.Spec.NodeName,
					HostNetwork:      d.Spec.Template.Spec.HostNetwork,
					SchedulerName:    d.Spec.Template.Spec.SchedulerName,
				},
				ObjectMeta: d.Spec.Template.ObjectMeta,
			},
		},
	}
}

func getchange(old, new interface{}) ([]byte, error) {
	oldbuffer := bytes.NewBuffer(nil)
	newbuffer := bytes.NewBuffer(nil)
	err := json.NewEncoder(oldbuffer).Encode(old)
	if err != nil {
		return nil, fmt.Errorf("encode old body error %s", err.Error())
	}
	err = json.NewEncoder(newbuffer).Encode(new)
	if err != nil {
		return nil, fmt.Errorf("encode new body error %s", err.Error())
	}
	var en EncodeNode
	if err := json.NewDecoder(oldbuffer).Decode(&en); err != nil {
		return nil, err
	}
	var ennew EncodeNode
	if err := json.NewDecoder(newbuffer).Decode(&ennew); err != nil {
		return nil, err
	}
	change := en.GetChange(&ennew)
	changebody, err := json.Marshal(change)
	if err != nil {
		return nil, err
	}
	return changebody, nil
}

func isContainsBootSequence(initContainers []corev1.Container) bool {
	for _, initContainer := range initContainers {
		if strings.Contains(initContainer.Name, "probe-mesh-") {
			return true
		}
	}
	return false
}
