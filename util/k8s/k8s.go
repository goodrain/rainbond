package k8s

import (
	"github.com/Sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/reference"

	corev1 "k8s.io/api/core/v1"
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

func ListEventsByPod(clientset kubernetes.Interface, pod *corev1.Pod) (*corev1.EventList, error) {
	ref, err := reference.GetReference(scheme.Scheme, pod)
	if err != nil {
		logrus.Errorf("Unable to construct reference to '%#v': %v", pod, err)
		return nil, err
	}
	ref.Kind = ""
	if _, isMirrorPod := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
		ref.UID = types.UID(pod.Annotations[corev1.MirrorPodAnnotationKey])
	}
	return clientset.CoreV1().Events(pod.GetNamespace()).Search(scheme.Scheme, ref)
}
