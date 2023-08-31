package registry

import (
	"bytes"
	"context"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sort"
)

// CleanRepo clean rbd-hub index
func (registry *Registry) CleanRepo(repository string, keep uint) error {
	tags, err := registry.Tags(repository)
	if err != nil {
		return err
	}
	sort.Strings(tags)
	logrus.Info("scan rbd-hub repository: ", repository)
	if uint(len(tags)) > keep {
		result := tags[:uint(len(tags))-keep]
		for _, tag := range result {
			registry.CleanRepoByTag(repository, tag)
		}
	}
	return nil
}

// CleanRepoByTag CleanRepoByTag
func (registry *Registry) CleanRepoByTag(repository string, tag string) error {
	dig, err := registry.ManifestDigestV2(repository, tag)
	if err != nil {
		logrus.Error("delete rbd-hub fail: ", repository)
		return err
	}
	if err := registry.DeleteManifest(repository, dig); err != nil {
		logrus.Error(err, "delete rbd-hub fail: ", repository, "; please set env REGISTRY_STORAGE_DELETE_ENABLED=true; see: https://t.goodrain.com/d/21-rbd-hub")
		return err
	}
	logrus.Info("delete rbd-hub tag: ", tag)
	return nil
}

// PodExecCmd registry garbage-collect
func (registry *Registry) PodExecCmd(config *rest.Config, clientset *kubernetes.Clientset, podName string, cmd []string) (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"name": podName}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	pods, err := clientset.CoreV1().Pods("rbd-system").List(context.TODO(), listOptions)
	if err != nil {
		return stdout, stderr, err
	}

	for _, pod := range pods.Items {
		req := clientset.CoreV1().RESTClient().Post().
			Namespace("rbd-system").
			Resource("pods").
			Name(pod.Name).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Command: cmd,
				Stdin:   false,
				Stdout:  true,
				Stderr:  true,
				TTY:     false,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
		if err != nil {
			return stdout, stderr, err
		}
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    false,
		})
		if err != nil {
			return stdout, stderr, err
		}
		return stdout, stderr, nil
	}
	return stdout, stderr, nil
}
