// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform
 
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

package k8s

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/pquerna/ffjson/ffjson"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/store"

	"github.com/Sirupsen/logrus"
	v3 "github.com/coreos/etcd/clientv3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	policy "k8s.io/client-go/pkg/apis/policy/v1beta1"
)

const (
	Force               = true
	IgnoreDaemonsets    = true
	DeleteLocalData     = true
	EvictionKind        = "Eviction"
	EvictionSubresource = "pods/eviction"

	kDaemonsetFatal      = "DaemonSet-managed pods (use --ignore-daemonsets to ignore)"
	kDaemonsetWarning    = "Ignoring DaemonSet-managed pods"
	kLocalStorageFatal   = "pods with local storage (use --delete-local-data to override)"
	kLocalStorageWarning = "Deleting pods with local storage"
	kUnmanagedFatal      = "pods not managed by ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet (use --force to override)"
	kUnmanagedWarning    = "Deleting pods not managed by ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet"
)

var (
	clientSet *kubernetes.Clientset
)

type podFilter func(v1.Pod) (include bool, w *warning, f *fatal)
type podStatuses map[string][]string
type warning struct {
	string
}
type fatal struct {
	string
}

//GetNodeByName get node
func GetNodeByName(nodename string) (*v1.Node, error) {
	opt := metav1.GetOptions{}
	return K8S.Clientset.CoreV1().Nodes().Get(nodename, opt)
}

//CordonOrUnCordon 节点是否调度属性处理
// drain:true 不可调度
func CordonOrUnCordon(nodeName string, drain bool) (*v1.Node, error) {
	data := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, drain)
	node, err := K8S.Clientset.CoreV1().Nodes().Patch(nodeName, types.StrategicMergePatchType, []byte(data))
	if err != nil {
		return node, err
	}
	return node, nil
}

//UpdateLabels update lables
func UpdateLabels(nodeName string, labels map[string]string) (*v1.Node, error) {
	labelStr, err := ffjson.Marshal(labels)
	if err != nil {
		return nil, err
	}
	data := fmt.Sprintf(`{"metadata":{"labels":%s}}`, string(labelStr))
	node, err := K8S.Clientset.CoreV1().Nodes().Patch(nodeName, types.StrategicMergePatchType, []byte(data))
	if err != nil {
		return node, err
	}
	return node, nil
}

//DeleteOrEvictPodsSimple 驱逐Pod
func DeleteOrEvictPodsSimple(nodeName string) error {
	pods, err := getPodsToDeletion(nodeName)
	if err != nil {
		logrus.Infof("get pods  of node %s failed ", nodeName)
		return err
	}
	policyGroupVersion, err := SupportEviction(K8S.Clientset)
	if err != nil {
		return err
	}
	if policyGroupVersion == "" {
		return fmt.Errorf("the server can not support eviction subresource")
	}
	for _, v := range pods {
		evictPod(v, policyGroupVersion)
	}
	return nil
}

// deleteOrEvictPods deletes or evicts the pods on the api server
func deleteOrEvictPods(pods []v1.Pod) error {
	if len(pods) == 0 {
		return nil
	}

	policyGroupVersion, err := SupportEviction(K8S.Clientset)
	if err != nil {
		return err
	}

	getPodFn := func(namespace, name string) (*v1.Pod, error) {
		return K8S.Clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	}

	return evictPods(pods, policyGroupVersion, getPodFn)
	//if len(policyGroupVersion) > 0 {
	//	//return evictPods(pods, policyGroupVersion, getPodFn)
	//} else {
	//	return deletePods(pods, getPodFn)
	//}
}

func deletePods(pods []v1.Pod, getPodFn func(namespace, name string) (*v1.Pod, error)) error {
	// 0 timeout means infinite, we use MaxInt64 to represent it.
	var globalTimeout time.Duration
	if conf.Config.ReqTimeout == 0 {
		//if Timeout == 0 {
		globalTimeout = time.Duration(math.MaxInt64)
	} else {
		globalTimeout = 1
		//globalTimeout = Timeout
	}
	for _, pod := range pods {
		err := deletePod(pod)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	_, err := waitForDelete(pods, time.Second*1, globalTimeout, false, getPodFn)
	return err
}
func waitForDelete(pods []v1.Pod, interval, timeout time.Duration, usingEviction bool, getPodFn func(string, string) (*v1.Pod, error)) ([]v1.Pod, error) {
	var verbStr string
	if usingEviction {
		verbStr = "evicted"
	} else {
		verbStr = "deleted"
	}
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		pendingPods := []v1.Pod{}
		for i, pod := range pods {
			p, err := getPodFn(pod.Namespace, pod.Name)
			if apierrors.IsNotFound(err) || (p != nil && p.ObjectMeta.UID != pod.ObjectMeta.UID) {
				fmt.Println(verbStr)
				//cmdutil.PrintSuccess(o.mapper, false, o.Out, "pod", pod.Name, false, verbStr)//todo
				continue
			} else if err != nil {
				return false, err
			} else {
				pendingPods = append(pendingPods, pods[i])
			}
		}
		pods = pendingPods
		if len(pendingPods) > 0 {
			return false, nil
		}
		return true, nil
	})
	return pods, err
}
func deletePod(pod v1.Pod) error {
	deleteOptions := &metav1.DeleteOptions{}
	//if GracePeriodSeconds >= 0 {
	//if 1 >= 0 {
	//	//gracePeriodSeconds := int64(GracePeriodSeconds)
	//	gracePeriodSeconds := int64(1)
	//	deleteOptions.GracePeriodSeconds = &gracePeriodSeconds
	//}
	gracePeriodSeconds := int64(1)
	deleteOptions.GracePeriodSeconds = &gracePeriodSeconds
	return K8S.Clientset.CoreV1().Pods(pod.Namespace).Delete(pod.Name, deleteOptions)
}

//evictPod 驱离POD
func evictPod(pod v1.Pod, policyGroupVersion string) error {
	deleteOptions := &metav1.DeleteOptions{}
	//if o.GracePeriodSeconds >= 0 {
	//	gracePeriodSeconds := int64(o.GracePeriodSeconds)
	//	deleteOptions.GracePeriodSeconds = &gracePeriodSeconds
	//}

	eviction := &policy.Eviction{
		///Users/goodrain/go/src/k8s.io/kubernetes/pkg/apis/policy/types.go
		///Users/goodrain/go/src/k8s.io/apimachinery/pkg/apis/meta/v1/types.go
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyGroupVersion,
			Kind:       EvictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: deleteOptions,
	}
	// Remember to change change the URL manipulation func when Evction's version change
	return K8S.Clientset.Policy().Evictions(eviction.Namespace).Evict(eviction)
}
func evictPods(pods []v1.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*v1.Pod, error)) error {
	doneCh := make(chan bool, len(pods))
	errCh := make(chan error, 1)

	for _, pod := range pods {
		go func(pod v1.Pod, doneCh chan bool, errCh chan error) {
			var err error
			for {
				err = evictPod(pod, policyGroupVersion)
				if err == nil {
					break
				} else if apierrors.IsNotFound(err) {
					doneCh <- true
					return
				} else if apierrors.IsTooManyRequests(err) {
					time.Sleep(5 * time.Second)
				} else {
					errCh <- fmt.Errorf("error when evicting pod %q: %v", pod.Name, err)
					return
				}
			}
			podArray := []v1.Pod{pod}
			_, err = waitForDelete(podArray, time.Second*1, time.Duration(math.MaxInt64), true, getPodFn)
			if err == nil {
				doneCh <- true
			} else {
				errCh <- fmt.Errorf("error when waiting for pod %q terminating: %v", pod.Name, err)
			}
		}(pod, doneCh, errCh)
	}

	doneCount := 0
	// 0 timeout means infinite, we use MaxInt64 to represent it.
	var globalTimeout time.Duration
	globalTimeout = time.Duration(math.MaxInt64)
	//if conf.Config.ReqTimeout == 0 {
	//	//if Timeout == 0 {
	//	globalTimeout = time.Duration(math.MaxInt64)
	//} else {
	//	//globalTimeout = Timeout
	//	globalTimeout = 1000
	//}
	for {
		select {
		case err := <-errCh:
			return err
		case <-doneCh:
			doneCount++
			if doneCount == len(pods) {
				return nil
			}
		case <-time.After(globalTimeout):
			return fmt.Errorf("Drain did not complete within %v", globalTimeout)
		}
	}
}

// SupportEviction uses Discovery API to find out if the server support eviction subresource
// If support, it will return its groupVersion; Otherwise, it will return ""
func SupportEviction(clientset *kubernetes.Clientset) (string, error) {
	//K8S.Clientset.Discovery().ServerGroups()
	discoveryClient := clientset.Discovery()
	groupList, err := discoveryClient.ServerGroups()
	if err != nil {
		return "", err
	}
	foundPolicyGroup := false
	var policyGroupVersion string
	for _, group := range groupList.Groups {
		if group.Name == "policy" {
			foundPolicyGroup = true
			policyGroupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}
	if !foundPolicyGroup {
		return "", nil
	}
	resourceList, err := discoveryClient.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return "", err
	}
	for _, resource := range resourceList.APIResources {
		if resource.Name == EvictionSubresource && resource.Kind == EvictionKind {
			return policyGroupVersion, nil
		}
	}
	return "", nil
}
func mirrorPodFilter(pod v1.Pod) (bool, *warning, *fatal) {

	if _, found := pod.ObjectMeta.Annotations["kubernetes.io/config.mirror"]; found {
		return false, nil, nil
	}
	return true, nil, nil
}
func hasLocalStorage(pod v1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.EmptyDir != nil {
			return true
		}
	}

	return false
}
func localStorageFilter(pod v1.Pod) (bool, *warning, *fatal) {
	if !hasLocalStorage(pod) {
		return true, nil, nil
	}
	if !DeleteLocalData {
		return false, nil, &fatal{kLocalStorageFatal}
	}
	return true, &warning{kLocalStorageWarning}, nil
}

func getController(sr *api.SerializedReference) (interface{}, error) {
	switch sr.Reference.Kind {
	case "ReplicationController":
		return K8S.Clientset.Core().ReplicationControllers(sr.Reference.Namespace).Get(sr.Reference.Name, metav1.GetOptions{})
	case "DaemonSet":
		return K8S.Clientset.Extensions().DaemonSets(sr.Reference.Namespace).Get(sr.Reference.Name, metav1.GetOptions{})
	case "Job":
		return K8S.Clientset.Batch().Jobs(sr.Reference.Namespace).Get(sr.Reference.Name, metav1.GetOptions{})
	case "ReplicaSet":
		return K8S.Clientset.Extensions().ReplicaSets(sr.Reference.Namespace).Get(sr.Reference.Name, metav1.GetOptions{})
	case "StatefulSet":
		return K8S.Clientset.Apps().StatefulSets(sr.Reference.Namespace).Get(sr.Reference.Name, metav1.GetOptions{})
	}
	return nil, fmt.Errorf("Unknown controller kind %q", sr.Reference.Kind)
}

func getPodCreator(pod v1.Pod) (*api.SerializedReference, error) {
	_, found := pod.ObjectMeta.Annotations[api.CreatedByAnnotation]
	if !found {
		return nil, nil
	}
	// Now verify that the specified creator actually exists.
	sr := &api.SerializedReference{}

	//if err := runtime.DecodeInto(util.NewFactory(nil).Decoder(true), []byte(creatorRef), sr); err != nil {
	//	return nil, err
	//}
	// We assume the only reason for an error is because the controller is
	// gone/missing, not for any other cause.  TODO(mml): something more
	// sophisticated than this
	_, err := getController(sr)
	if err != nil {
		return nil, err
	}
	return sr, nil
}

func unreplicatedFilter(pod v1.Pod) (bool, *warning, *fatal) {
	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		return true, nil, nil
	}

	sr, err := getPodCreator(pod)
	if err != nil {
		// if we're forcing, remove orphaned pods with a warning
		if apierrors.IsNotFound(err) && Force {
			return true, &warning{err.Error()}, nil
		}
		return false, nil, &fatal{err.Error()}
	}
	if sr != nil {
		return true, nil, nil
	}
	if !Force {
		return false, nil, &fatal{kUnmanagedFatal}
	}
	return true, &warning{kUnmanagedWarning}, nil
}
func daemonsetFilter(pod v1.Pod) (bool, *warning, *fatal) {
	sr, err := getPodCreator(pod)
	if err != nil {
		// if we're forcing, remove orphaned pods with a warning
		if apierrors.IsNotFound(err) && Force {
			return true, &warning{err.Error()}, nil
		}
		return false, nil, &fatal{err.Error()}
	}
	if sr == nil || sr.Reference.Kind != "DaemonSet" {
		return true, nil, nil
	}
	if _, err := clientSet.Extensions().DaemonSets(sr.Reference.Namespace).Get(sr.Reference.Name, metav1.GetOptions{}); err != nil {
		return false, nil, &fatal{err.Error()}
	}
	if !IgnoreDaemonsets {
		return false, nil, &fatal{kDaemonsetFatal}
	}
	return false, &warning{kDaemonsetWarning}, nil
}

func getPodsToDeletion(nodeName string) (pods []v1.Pod, err error) {
	podList, err := K8S.Clientset.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String()})

	if err != nil {
		return pods, err
	}
	for _, pod := range podList.Items {
		pods = append(pods, pod)
	}
	return pods, nil
}

//GetPodsByNodeName get pods by nodename
func GetPodsByNodeName(nodeName string) (pods []v1.Pod, err error) {
	podList, err := K8S.Clientset.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String(),
	})

	if err != nil {
		return pods, err
	}
	p := make(map[string]v1.Pod)
	for _, pod := range podList.Items {
		p[string(pod.UID)] = pod
	}
	for _, v := range p {
		pods = append(pods, v)
	}
	return pods, nil
}

//GetAllPods get all pods
func GetAllPods() (pods []v1.Pod, err error) {
	podList, err := K8S.Clientset.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return pods, err
	}
	p := make(map[string]v1.Pod)
	for _, pod := range podList.Items {
		p[string(pod.UID)] = pod
	}
	for _, v := range p {
		pods = append(pods, v)
	}
	return pods, nil
}
func getPodsForDeletion(nodeName string) (pods []v1.Pod, err error) {
	podList, err := K8S.Clientset.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String()})

	if err != nil {
		return pods, err
	}

	ws := podStatuses{}
	fs := podStatuses{}

	for _, pod := range podList.Items {
		podOk := true
		for _, filt := range []podFilter{mirrorPodFilter, localStorageFilter, unreplicatedFilter, daemonsetFilter} {
			filterOk, w, f := filt(pod)

			podOk = podOk && filterOk
			if w != nil {
				ws[w.string] = append(ws[w.string], pod.Name)
			}
			if f != nil {
				fs[f.string] = append(fs[f.string], pod.Name)
			}
		}
		if podOk {
			pods = append(pods, pod)
		}
	}

	if len(fs) > 0 {
		return []v1.Pod{}, errors.New(fs.Message())
	}
	if len(ws) > 0 {
		fmt.Fprintf(os.Stdout, "WARNING: %s\n", ws.Message())
	}
	return pods, nil
}
func (ps podStatuses) Message() string {
	msgs := []string{}

	for key, pods := range ps {
		msgs = append(msgs, fmt.Sprintf("%s: %s", key, strings.Join(pods, ", ")))
	}
	return strings.Join(msgs, "; ")
}

//DeleteNode  k8s节点下线
func DeleteNode(nodename string) error {
	_, err := GetNodeByName(nodename)
	if err != nil {
		logrus.Infof("get k8s node %s failed ", nodename)
		return err
	}
	//节点禁止调度
	_, err = CordonOrUnCordon(nodename, true)
	if err != nil {
		logrus.Infof("cordon node %s failed ", nodename)
		return err
	}
	//节点pod驱离
	err = DeleteOrEvictPodsSimple(nodename)
	if err != nil {
		logrus.Infof("delete or evict pods of node  %s failed ", nodename)
		return err
	}
	//删除节点
	err = deleteNodeWithoutPods(nodename)
	if err != nil {
		logrus.Infof("delete node with given name failed  %s failed ", nodename)
		return err
	}
	return nil
}

func deleteNodeWithoutPods(name string) error {
	opt := &metav1.DeleteOptions{}
	err := K8S.Clientset.Nodes().Delete(name, opt)
	if err != nil {
		return err
	}
	return nil
}

//CreatK8sNodeFromRainbonNode 创建k8s node
func CreatK8sNodeFromRainbonNode(rainbondNode *model.HostNode) (*v1.Node, error) {
	capacity := make(v1.ResourceList)
	capacity[v1.ResourceCPU] = *resource.NewQuantity(rainbondNode.AvailableCPU, resource.BinarySI)
	capacity[v1.ResourceMemory] = *resource.NewQuantity(rainbondNode.AvailableMemory, resource.BinarySI)
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   rainbondNode.ID,
			Labels: rainbondNode.Labels,
		},
		Spec: v1.NodeSpec{
			Unschedulable: rainbondNode.Unschedulable,
		},
		Status: v1.NodeStatus{
			Capacity:    capacity,
			Allocatable: capacity,
			Addresses: []v1.NodeAddress{
				v1.NodeAddress{Type: v1.NodeHostName, Address: rainbondNode.HostName},
				v1.NodeAddress{Type: v1.NodeInternalIP, Address: rainbondNode.InternalIP},
				v1.NodeAddress{Type: v1.NodeExternalIP, Address: rainbondNode.ExternalIP},
			},
		},
	}
	savedNode, err := K8S.Clientset.Nodes().Create(node)
	if err != nil {
		return nil, err
	}
	logrus.Info("creating new node success , details: %v ", savedNode)
	return node, nil
}

func LabelMulti(k8sNodeName string, keys []string) error {
	opt := metav1.GetOptions{}
	patchSet, err := K8S.Clientset.Nodes().Get(k8sNodeName, opt)
	if err != nil {
		logrus.Errorf("get node %s fail,details : %s", k8sNodeName, err.Error())
		return err
	}
	labels := make(map[string]string, len(keys))
	patchSet.SetLabels(labels)
	//patchNoLabelJSON, err := json.Marshal(patchSet)
	//if err != nil {
	//	logrus.Errorf("marshal patchset %s fail,details : %s", k8sNodeName, err.Error())
	//	return err
	//}
	//_, err = K8S.Clientset.Nodes().Patch(k8sNodeName, types.MergePatchType, patchNoLabelJSON)

	K8S.Clientset.Nodes().Update(patchSet)
	opt2 := metav1.GetOptions{}
	patchSetFinal, err := K8S.Clientset.Nodes().Get(k8sNodeName, opt2)
	if err != nil {
		logrus.Errorf("get node %s fail,details : %s", k8sNodeName, err.Error())
		return err
	}

	for _, v := range keys {
		labels[v] = "default"
	}
	logrus.Infof("get k8s node labels %v", patchSetFinal.Labels)
	patchSetFinal.Labels = labels
	patchSetInJSON, err := json.Marshal(patchSetFinal)
	if err != nil {
		logrus.Errorf("marshal patchset %s fail,details : %s", k8sNodeName, err.Error())
		return err
	}
	_, err = K8S.Clientset.Nodes().Patch(k8sNodeName, types.StrategicMergePatchType, patchSetInJSON)
	if err != nil {
		logrus.Errorf("patch node %s fail,details : %s", k8sNodeName, err.Error())
		return err
	}
	return nil
}
func MarkLabel(k8sNodeName, key, value string) error {
	opt := metav1.GetOptions{}
	patchSet, err := K8S.Clientset.Nodes().Get(k8sNodeName, opt)
	if err != nil {
		logrus.Errorf("get node %s fail,details : %s", k8sNodeName, err.Error())
		return err
	}
	labels := make(map[string]string, 1)
	labels[key] = value
	patchSet.SetLabels(labels)
	patchSetInJSON, err := json.Marshal(patchSet)
	if err != nil {
		logrus.Errorf("marshal patchset %s fail,details : %s", k8sNodeName, err.Error())
		return err
	}

	_, err = K8S.Clientset.Nodes().Patch(k8sNodeName, types.StrategicMergePatchType, patchSetInJSON)
	if err != nil {
		logrus.Errorf("patch node %s fail,details : %s", k8sNodeName, err.Error())
		return err
	}
	return nil
}

func CreateK8sNode(node *model.HostNode) (*v1.Node, error) {
	cpu, err := resource.ParseQuantity(fmt.Sprintf("%dm", node.AvailableCPU*1000))
	if err != nil {
		return nil, err
	}
	mem, err := resource.ParseQuantity(fmt.Sprintf("%dKi", node.AvailableMemory*1024*1024))
	if err != nil {
		return nil, err
	}
	nameAddress := v1.NodeAddress{
		Type:    v1.NodeHostName,
		Address: node.HostName,
	}
	internalIP := v1.NodeAddress{
		Type:    v1.NodeInternalIP,
		Address: node.InternalIP,
	}
	externalIP := v1.NodeAddress{
		Type:    v1.NodeExternalIP,
		Address: node.ExternalIP,
	}
	k8sNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			UID:    types.UID(node.ID), //todo 不知道create的时候用不用这个
			Name:   node.HostName,
			Labels: node.Labels,
		},
		Spec: v1.NodeSpec{
			Unschedulable: node.Unschedulable,
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{v1.ResourceCPU: cpu, v1.ResourceMemory: mem},
			Addresses:   []v1.NodeAddress{nameAddress, internalIP, externalIP},
		},
	}
	return k8sNode, nil
}

func DeleteSource(key string) error {
	if key == "" {
		return errors.New("key or source can not nil")
	}
	_, err := store.DefalutClient.Delete(key + "/info")
	return err
}
func GetSource(key string) (node *model.HostNode, err error) {
	node = &model.HostNode{}
	if key == "" {
		return node, errors.New("key or source can not nil")
	}
	key = key + "/info"
	resp, err := store.DefalutClient.Get(key, v3.WithPrefix())
	if err != nil {
		logrus.Warnf("get resp from etcd with given key %s failed", err.Error())
		return
	}
	if resp.Count == 0 {
		return nil, errors.New("can't found node with given key " + key)
	} else if resp.Count == 1 {
		err = json.Unmarshal(resp.Kvs[0].Value, node)
		if err != nil {
			logrus.Infof("从etcd获取node unmarshal 失败 : %s", err.Error())
			return &model.HostNode{}, err
		}
		return node, nil
	} else {
		return nil, errors.New("found multi node with given key " + key)
	}

}
func GetSourceList() ([]*model.HostNode, error) {
	key := conf.Config.K8SNode
	if key == "" {
		return nil, errors.New("key  can not nil")
	}

	//resp, err := DefalutClient.Get(conf.Config.Cmd, client.WithPrefix())
	res, err := store.DefalutClient.Get(conf.Config.K8SNode, v3.WithPrefix())
	if err != nil {
		return nil, err
	}
	//if !res.Node.Dir {
	//	return nil, fmt.Errorf("%s is not dir. don't list", key)
	//}
	var list []*model.HostNode
	for _, j := range res.Kvs {
		hostnode := new(model.HostNode)
		if e := json.Unmarshal(j.Value, hostnode); e != nil {
			logrus.Warnf("job[%s] umarshal err: %s", string(j.Key), e.Error())
			continue
		}
		list = append(list, hostnode)
	}
	return list, nil

}

//AddSource 添加资源
func AddSource(key string, object interface{}) error {
	logrus.Infof("updating source who's key is %s", key)
	if key == "" {
		return errors.New("key or source can not nil")
	}
	var data string
	var err error
	switch object.(type) {
	case string:
		data = object.(string)
		break
	case int:
		data = fmt.Sprintf("%d", object)
		break
	case []byte:
		data = string(object.([]byte))
		break
	default:
		dataB, err := json.Marshal(object)
		if err != nil {
			return err
		}
		data = string(dataB)
	}
	key = key + "/info"
	_, err = store.DefalutClient.Put(key, data)
	if err != nil {
		return err
	}
	return nil
}
