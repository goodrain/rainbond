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
	"encoding/json"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
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
func (s *ServiceAction) GetK8sAttribute(componentID, name string) (*model.ComponentK8sAttributes, error) {
	return db.GetManager().ComponentK8sAttributeDao().GetByComponentIDAndName(componentID, name)
}

// CreateK8sAttribute -
func (s *ServiceAction) CreateK8sAttribute(tenantID, componentID string, k8sAttr *api_model.ComponentK8sAttribute) error {
	return db.GetManager().ComponentK8sAttributeDao().AddModel(k8sAttr.DbModel(tenantID, componentID))
}

// UpdateK8sAttribute -
func (s *ServiceAction) UpdateK8sAttribute(componentID string, k8sAttributes *api_model.ComponentK8sAttribute) error {
	attr, err := db.GetManager().ComponentK8sAttributeDao().GetByComponentIDAndName(componentID, k8sAttributes.Name)
	if err != nil {
		return err
	}
	attr.AttributeValue = k8sAttributes.AttributeValue
	return db.GetManager().ComponentK8sAttributeDao().UpdateModel(attr)
}

// DeleteK8sAttribute -
func (s *ServiceAction) DeleteK8sAttribute(componentID, name string) error {
	return db.GetManager().ComponentK8sAttributeDao().DeleteByComponentIDAndName(componentID, name)
}
