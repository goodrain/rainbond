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

package model

const (
	//K8sAttributeNameNodeSelector -
	K8sAttributeNameNodeSelector = "nodeSelector"
	//K8sAttributeNameLabels -
	K8sAttributeNameLabels = "labels"
	//K8sAttributeNameTolerations -
	K8sAttributeNameTolerations = "tolerations"
	//K8sAttributeNameVolumes -
	K8sAttributeNameVolumes = "volumes"
	//K8sAttributeNameServiceAccountName -
	K8sAttributeNameServiceAccountName = "serviceAccountName"
	//K8sAttributeNamePrivileged -
	K8sAttributeNamePrivileged = "privileged"
	//K8sAttributeNameAffinity -
	K8sAttributeNameAffinity = "affinity"
	//K8sAttributeNameVolumeMounts -
	K8sAttributeNameVolumeMounts = "volumeMounts"
	//K8sAttributeNameENV -
	K8sAttributeNameENV = "env"
	//K8sAttributeNameShareProcessNamespace -
	K8sAttributeNameShareProcessNamespace = "shareProcessNamespace"
	//K8sAttributeNameDnsPolicy -
	K8sAttributeNameDnsPolicy = "dnsPolicy"
	//K8sAttributeNameDnsPolicy -
	K8sAttributeNameDnsConfig = "dnsConfig"
	//K8sAttributeNameResources -
	K8sAttributeNameResources = "resources"
	//K8sAttributeNameResources -
	K8sAttributeNameHostIPC = "hostIPC"
	//K8sAttributeNameLifecycle -
	K8sAttributeNameLifecycle = "lifecycle"
)

// ComponentK8sAttributes -
type ComponentK8sAttributes struct {
	Model
	TenantID    string `gorm:"column:tenant_id;size:32" validate:"tenant_id|between:30,33" json:"tenant_id"`
	ComponentID string `gorm:"column:component_id" json:"component_id"`

	// Name Define the attribute name, which is currently supported
	// [nodeSelector/labels/tolerations/volumes/serviceAccountName/privileged/affinity/volumeMounts]
	// The field name should be the same as that in the K8s resource yaml file.
	Name string `gorm:"column:name" json:"name"`

	// The field type defines how the attribute is stored. Currently, `json/yaml/string` are supported
	SaveType string `gorm:"column:save_type" json:"save_type"`

	// Define the attribute value, which is stored in the database.
	// The value is stored in the database in the form of `json/yaml/string`.
	AttributeValue string `gorm:"column:attribute_value;type:longtext" json:"attribute_value"`
}

// TableName 表名
func (t *ComponentK8sAttributes) TableName() string {
	return "component_k8s_attributes"
}
