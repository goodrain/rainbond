// RAINBOND, Application Management Platform
// Copyright (C) 2021-2021 Goodrain Co., Ltd.

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

package componentdefinition

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rainbondversioned "github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ErrNotSupport = fmt.Errorf("not support component definition")
var ErrOnlyCUESupport = fmt.Errorf("component definition only support cue template")

type ComponentDefinitionBuilder struct {
	definitions map[string]*v1alpha1.ComponentDefinition
	namespace   string
	lock        sync.Mutex
}

var componentDefinitionBuilder *ComponentDefinitionBuilder

func NewComponentDefinitionBuilder(namespace string) *ComponentDefinitionBuilder {
	componentDefinitionBuilder = &ComponentDefinitionBuilder{
		definitions: make(map[string]*v1alpha1.ComponentDefinition),
		namespace:   namespace,
	}
	return componentDefinitionBuilder
}

func GetComponentDefinitionBuilder() *ComponentDefinitionBuilder {
	return componentDefinitionBuilder
}

func (c *ComponentDefinitionBuilder) OnAdd(obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cd, ok := obj.(*v1alpha1.ComponentDefinition)
	if ok {
		logrus.Infof("load componentdefinition %s", cd.Name)
		c.definitions[cd.Name] = cd
	}
}
func (c *ComponentDefinitionBuilder) OnUpdate(oldObj, newObj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cd, ok := newObj.(*v1alpha1.ComponentDefinition)
	if ok {
		logrus.Infof("update componentdefinition %s", cd.Name)
		c.definitions[cd.Name] = cd
	}
}
func (c *ComponentDefinitionBuilder) OnDelete(obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cd, ok := obj.(*v1alpha1.ComponentDefinition)
	if ok {
		logrus.Infof("delete componentdefinition %s", cd.Name)
		delete(c.definitions, cd.Name)
	}
}

func (c *ComponentDefinitionBuilder) GetComponentDefinition(name string) *v1alpha1.ComponentDefinition {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.definitions[name]
}

func (c *ComponentDefinitionBuilder) GetComponentProperties(as *v1.AppService, dbm db.Manager, cd *v1alpha1.ComponentDefinition) interface{} {
	//TODO: support custom component properties
	switch cd.Name {
	case thirdComponetDefineName:
		properties := &ThirdComponentProperties{}
		tpsd, err := dbm.ThirdPartySvcDiscoveryCfgDao().GetByServiceID(as.ServiceID)
		if err != nil {
			logrus.Errorf("query component %s third source config failure %s", as.ServiceID, err.Error())
		}
		if tpsd != nil {
			// support other source type
			if tpsd.Type == dbmodel.DiscorveryTypeKubernetes.String() {
				properties.Kubernetes = ThirdComponentKubernetes{
					Name:      tpsd.ServiceName,
					Namespace: tpsd.Namespace,
				}
			}
		}
		ports, err := dbm.TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
		if err != nil {
			logrus.Errorf("query component %s ports failure %s", as.ServiceID, err.Error())
		}

		for _, port := range ports {
			properties.Port = append(properties.Port, &ThirdComponentPort{
				Port:      port.ContainerPort,
				Name:      strings.ToLower(port.PortAlias),
				OpenInner: *port.IsInnerService,
				OpenOuter: *port.IsOuterService,
			})
		}
		if properties.Port == nil {
			properties.Port = []*ThirdComponentPort{}
		}
		return properties
	default:
		return nil
	}
}

func (c *ComponentDefinitionBuilder) BuildWorkloadResource(as *v1.AppService, dbm db.Manager) error {
	cd := c.GetComponentDefinition(as.GetComponentDefinitionName())
	if cd == nil {
		return ErrNotSupport
	}
	if cd.Spec.Schematic == nil || cd.Spec.Schematic.CUE == nil {
		return ErrOnlyCUESupport
	}
	ctx := NewTemplateContext(as, cd.Spec.Schematic.CUE.Template, c.GetComponentProperties(as, dbm, cd))
	manifests, err := ctx.GenerateComponentManifests()
	if err != nil {
		return err
	}
	ctx.SetContextValue(manifests)
	as.SetManifests(manifests)
	if len(manifests) > 0 {
		as.SetWorkload(manifests[0])
	}
	return nil
}

//InitCoreComponentDefinition init the built-in component type definition.
//Should be called after the store is initialized.
func (c *ComponentDefinitionBuilder) InitCoreComponentDefinition(rainbondClient rainbondversioned.Interface) {
	coreComponentDefinition := []*v1alpha1.ComponentDefinition{&thirdComponetDefine}
	for _, ccd := range coreComponentDefinition {
		if c.GetComponentDefinition(ccd.Name) == nil {
			logrus.Infof("create core componentdefinition %s", ccd.Name)
			if _, err := rainbondClient.RainbondV1alpha1().ComponentDefinitions(c.namespace).Create(context.Background(), ccd, metav1.CreateOptions{}); err != nil {
				logrus.Errorf("create core componentdefinition %s failire %s", ccd.Name, err.Error())
			}
		}
	}
	logrus.Infof("success check core componentdefinition from cluster")
}
