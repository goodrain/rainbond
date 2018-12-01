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

package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/controller"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type rainbondsslcProvisioner struct {
	// The directory to create PV-backing directories in
	name    string
	kubecli *kubernetes.Clientset
}

// NewRainbondsslcProvisioner creates a new Rainbond statefulset share volume provisioner
func NewRainbondsslcProvisioner() controller.Provisioner {
	return &rainbondsslcProvisioner{
		name: "rainbond.io/provisioner-sslc",
	}
}

var _ controller.Provisioner = &rainbondsslcProvisioner{}

//selectNode select an appropriate node with the largest resource surplus
func (p *rainbondsslcProvisioner) selectNode() (*v1.Node, error) {
	allnode, err := p.kubecli.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var maxavailable int64
	var selectnode *v1.Node
	for _, node := range allnode.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				if condition.Status == v1.ConditionTrue {
					ip := ""
					for _, address := range node.Status.Addresses {
						if address.Type == v1.NodeInternalIP {
							ip = address.Address
							break
						}
					}
					if ip == "" {
						break
					}
					pods, err := p.kubecli.CoreV1().Pods(v1.NamespaceAll).List(metav1.ListOptions{
						FieldSelector: "status.nodeIP=" + ip,
					})
					if err != nil {
						logrus.Errorf("list pods list from node ip error %s", err.Error())
						break
					}
					var nodeUsedMemory int64
					for _, pod := range pods.Items {
						for _, con := range pod.Spec.Containers {
							memory := con.Resources.Requests.Memory()
							nodeUsedMemory += memory.Value()
						}
					}
					available := node.Status.Allocatable.Memory().Value() - nodeUsedMemory
					if available >= maxavailable {
						maxavailable = available
						selectnode = &node
					}
				}
			}
		}
	}
	return selectnode, nil
}
func (p *rainbondsslcProvisioner) createPath(options controller.VolumeOptions) (string, error) {
	tenantID := options.PVC.Labels["tenant_id"]
	serviceID := options.PVC.Labels["service_id"]
	reqoptions := map[string]string{"tenant_id": tenantID, "service_id": serviceID, "pvcname": options.PVC.Name}
	var ip string
	for _, address := range options.SelectedNode.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			ip = address.Address
		}
	}
	if ip == "" {
		return "", fmt.Errorf("do not find node ip")
	}
	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(reqoptions); err != nil {
		return "", fmt.Errorf("create volume body failure %s", err.Error())
	}
	retry := 3
	var path string
	for retry > 0 {
		retry--
		res, err := http.Post(fmt.Sprintf("http://%s:6100/v2/localvolumes/create", ip), "application/json", body)
		if err != nil {
			logrus.Errorf("do request node api failure %s", err.Error())
		}
		var result = make(map[string]string)
		if res != nil && res.StatusCode == 200 && res.Body != nil {
			if err := json.NewDecoder(res.Body).Decode(result); err == nil {
				path = result["path"]
				break
			}
		}
		time.Sleep(time.Second * 2)
	}
	return path, nil
}

// Provision creates a storage asset and returns a PV object representing it.
func (p *rainbondsslcProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	//runtime select an appropriate node with the largest resource surplus
	if options.SelectedNode == nil {
		var err error
		options.SelectedNode, err = p.selectNode()
		if err != nil || options.SelectedNode == nil {
			return nil, fmt.Errorf("do not select an appropriate node for local volume")
		}
	}
	path, err := p.createPath(options)
	if err != nil {
		return nil, fmt.Errorf("create local volume from node %s failure %s", options.SelectedNode.Name, err.Error())
	}
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: path,
				},
			},
			NodeAffinity: &v1.VolumeNodeAffinity{
				Required: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						v1.NodeSelectorTerm{MatchFields: []v1.NodeSelectorRequirement{
							v1.NodeSelectorRequirement{
								Key:      "name",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{options.SelectedNode.GetName()},
							},
						},
						},
					},
				},
			},
		},
	}
	logrus.Infof("create rainbondssc pv %s for pvc %s", pv.Name, options.PVC.Name)
	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *rainbondsslcProvisioner) Delete(volume *v1.PersistentVolume) error {

	return nil
}

func (p *rainbondsslcProvisioner) Name() string {
	return p.name
}
