package k8s

import (
	"time"

	"github.com/Sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/reference"
)

// NewClientset -
func NewClientset(kubecfg string) (kubernetes.Interface, error) {
	c, err := clientcmd.BuildConfigFromFlags("", kubecfg)
	if err != nil {
		logrus.Errorf("error reading kube config file: %s", err.Error())
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		logrus.Error("error creating kube api client", err.Error())
		return nil, err
	}
	return clientset, nil
}

// NewRainbondFilteredSharedInformerFactory -
func NewRainbondFilteredSharedInformerFactory(clientset kubernetes.Interface) informers.SharedInformerFactory {
	return informers.NewFilteredSharedInformerFactory(
		clientset, 30*time.Second, corev1.NamespaceAll, func(options *metav1.ListOptions) {
			options.LabelSelector = "service_id=81f86ea23bb22c37385b8e7edf36f4a9"
		},
	)
}

// ExtractLabels extracts the service information from the labels
func ExtractLabels(labels map[string]string) (string, string, string, string) {
	if labels == nil {
		return "", "", "", ""
	}
	return labels["tenant_id"], labels["service_id"], labels["version"], labels["creater_id"]
}

// ListEventsByPod -
type ListEventsByPod func(kubernetes.Interface, *corev1.Pod) *corev1.EventList

// DefListEventsByPod default implementatoin of ListEventsByPod
func DefListEventsByPod(clientset kubernetes.Interface, pod *corev1.Pod) *corev1.EventList {
	ref, err := reference.GetReference(scheme.Scheme, pod)
	if err != nil {
		logrus.Errorf("Unable to construct reference to '%#v': %v", pod, err)
		return nil
	}
	ref.Kind = ""
	if _, isMirrorPod := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
		ref.UID = types.UID(pod.Annotations[corev1.MirrorPodAnnotationKey])
	}
	events, _ := clientset.CoreV1().Events(pod.GetNamespace()).Search(scheme.Scheme, ref)
	return events
}
