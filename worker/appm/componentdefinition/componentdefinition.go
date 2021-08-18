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
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ErrNotSupport -
var ErrNotSupport = fmt.Errorf("not support component definition")

// ErrOnlyCUESupport -
var ErrOnlyCUESupport = fmt.Errorf("component definition only support cue template")

// Builder -
type Builder struct {
	logger      *logrus.Entry
	definitions map[string]*v1alpha1.ComponentDefinition
	namespace   string
	lock        sync.Mutex
}

var componentDefinitionBuilder *Builder

// NewComponentDefinitionBuilder -
func NewComponentDefinitionBuilder(namespace string) *Builder {
	componentDefinitionBuilder = &Builder{
		logger:      logrus.WithField("WHO", "Builder"),
		definitions: make(map[string]*v1alpha1.ComponentDefinition),
		namespace:   namespace,
	}
	return componentDefinitionBuilder
}

// GetComponentDefinitionBuilder -
func GetComponentDefinitionBuilder() *Builder {
	return componentDefinitionBuilder
}

// OnAdd -
func (c *Builder) OnAdd(obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cd, ok := obj.(*v1alpha1.ComponentDefinition)
	if ok {
		logrus.Infof("load componentdefinition %s", cd.Name)
		c.definitions[cd.Name] = cd
	}
}

// OnUpdate -
func (c *Builder) OnUpdate(oldObj, newObj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cd, ok := newObj.(*v1alpha1.ComponentDefinition)
	if ok {
		logrus.Infof("update componentdefinition %s", cd.Name)
		c.definitions[cd.Name] = cd
	}
}

// OnDelete -
func (c *Builder) OnDelete(obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cd, ok := obj.(*v1alpha1.ComponentDefinition)
	if ok {
		logrus.Infof("delete componentdefinition %s", cd.Name)
		delete(c.definitions, cd.Name)
	}
}

// GetComponentDefinition -
func (c *Builder) GetComponentDefinition(name string) *v1alpha1.ComponentDefinition {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.definitions[name]
}

// GetComponentProperties -
func (c *Builder) GetComponentProperties(as *v1.AppService, dbm db.Manager, cd *v1alpha1.ComponentDefinition) interface{} {
	//TODO: support custom component properties
	switch cd.Name {
	case thirdComponentDefineName:
		properties := &ThirdComponentProperties{}
		tpsd, err := dbm.ThirdPartySvcDiscoveryCfgDao().GetByServiceID(as.ServiceID)
		if err != nil {
			logrus.Errorf("query component %s third source config failure %s", as.ServiceID, err.Error())
		}
		if tpsd != nil {
			// support other source type
			if tpsd.Type == dbmodel.DiscorveryTypeKubernetes.String() {
				properties.Kubernetes = &ThirdComponentKubernetes{
					Name:      tpsd.ServiceName,
					Namespace: tpsd.Namespace,
				}
			}
		}

		// static endpoints
		endpoints, err := c.listStaticEndpoints(as.ServiceID)
		if err != nil {
			c.logger.Errorf("component id: %s; list static endpoints: %v", as.ServiceID, err)
		}
		properties.Endpoints = endpoints

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

		// probe
		probe, err := c.createProbe(as.ServiceID)
		if err != nil {
			c.logger.Warningf("create probe: %v", err)
		}
		properties.Probe = probe

		return properties
	default:
		return nil
	}
}

func (c *Builder) listStaticEndpoints(componentID string) ([]*v1alpha1.ThirdComponentEndpoint, error) {
	endpoints, err := db.GetManager().EndpointsDao().List(componentID)
	if err != nil {
		return nil, err
	}

	var res []*v1alpha1.ThirdComponentEndpoint
	for _, ep := range endpoints {
		res = append(res, &v1alpha1.ThirdComponentEndpoint{
			Address: ep.GetAddress(),
			Name:    ep.UUID,
		})
	}
	return res, nil
}

// BuildWorkloadResource -
func (c *Builder) BuildWorkloadResource(as *v1.AppService, dbm db.Manager) error {
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
func (c *Builder) InitCoreComponentDefinition(rainbondClient rainbondversioned.Interface) {
	coreComponentDefinition := []*v1alpha1.ComponentDefinition{&thirdComponentDefine}
	for _, ccd := range coreComponentDefinition {
		oldCoreComponentDefinition := c.GetComponentDefinition(ccd.Name)
		if oldCoreComponentDefinition == nil {
			logrus.Infof("create core componentdefinition %s", ccd.Name)
			if _, err := rainbondClient.RainbondV1alpha1().ComponentDefinitions(c.namespace).Create(context.Background(), ccd, metav1.CreateOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
				logrus.Errorf("create core componentdefinition %s failire %s", ccd.Name, err.Error())
			}
		} else {
			err := c.updateComponentDefinition(rainbondClient, oldCoreComponentDefinition, ccd)
			if err != nil {
				logrus.Errorf("update core componentdefinition(%s): %v", ccd.Name, err)
			}
		}
	}
	logrus.Infof("success check core componentdefinition from cluster")
}

func (c *Builder) updateComponentDefinition(rainbondClient rainbondversioned.Interface, oldComponentDefinition, newComponentDefinition *v1alpha1.ComponentDefinition) error {
	newVersion := getComponentDefinitionVersion(newComponentDefinition)
	oldVersion := getComponentDefinitionVersion(oldComponentDefinition)
	if newVersion == "" || !(oldVersion == "" || newVersion > oldVersion) {
		return nil
	}

	logrus.Infof("update core componentdefinition %s", newComponentDefinition.Name)
	newComponentDefinition.ResourceVersion = oldComponentDefinition.ResourceVersion
	if _, err := rainbondClient.RainbondV1alpha1().ComponentDefinitions(c.namespace).Update(context.Background(), newComponentDefinition, metav1.UpdateOptions{}); err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := rainbondClient.RainbondV1alpha1().ComponentDefinitions(c.namespace).Create(context.Background(), newComponentDefinition, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}
		return err
	}

	return nil
}

func getComponentDefinitionVersion(componentDefinition *v1alpha1.ComponentDefinition) string {
	if componentDefinition.ObjectMeta.Annotations == nil {
		return ""
	}
	return componentDefinition.ObjectMeta.Annotations["version"]
}

func (c *Builder) createProbe(componentID string) (*v1alpha1.Probe, error) {
	probe, err := db.GetManager().ServiceProbeDao().GetServiceUsedProbe(componentID, "readiness")
	if err != nil {
		return nil, err
	}
	if probe == nil {
		return nil, nil
	}

	p := &v1alpha1.Probe{
		TimeoutSeconds:   int32(probe.TimeoutSecond),
		PeriodSeconds:    int32(probe.PeriodSecond),
		SuccessThreshold: int32(probe.SuccessThreshold),
		FailureThreshold: int32(probe.FailureThreshold),
	}
	if probe.Scheme == "tcp" {
		p.TCPSocket = c.createTCPGetAction(probe)
	} else {
		p.HTTPGet = c.createHTTPGetAction(probe)
	}

	return p, nil
}

func (c *Builder) createHTTPGetAction(probe *dbmodel.TenantServiceProbe) *v1alpha1.HTTPGetAction {
	action := &v1alpha1.HTTPGetAction{Path: probe.Path}
	if probe.HTTPHeader != "" {
		hds := strings.Split(probe.HTTPHeader, ",")
		var headers []v1alpha1.HTTPHeader
		for _, hd := range hds {
			kv := strings.Split(hd, "=")
			if len(kv) == 1 {
				header := v1alpha1.HTTPHeader{
					Name:  kv[0],
					Value: "",
				}
				headers = append(headers, header)
			} else if len(kv) == 2 {
				header := v1alpha1.HTTPHeader{
					Name:  kv[0],
					Value: kv[1],
				}
				headers = append(headers, header)
			}
		}
		action.HTTPHeaders = headers
	}
	return action
}

func (c *Builder) createTCPGetAction(probe *dbmodel.TenantServiceProbe) *v1alpha1.TCPSocketAction {
	return &v1alpha1.TCPSocketAction{}
}
