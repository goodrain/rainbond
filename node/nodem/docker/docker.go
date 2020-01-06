package docker

import (
	"io"
	"os"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

var defaultSecretName = "rbd-docker-secret"
var defaultNamespace = "rbd-system"
var defaultFileName = "server.crt"
var defaultFilePath = "/etc/docker/certs.d/goodrain.me"

// SyncDockerCertFromSecret sync docker cert from secret
func SyncDockerCertFromSecret(clientset kubernetes.Interface, namespace, secretName string) error {
	namespace = strings.TrimSpace(namespace)
	secretName = strings.TrimSpace(secretName)
	if namespace == "" {
		namespace = defaultNamespace
	}
	if secretName == "" {
		secretName = defaultSecretName
	}
	secretInfo, err := clientset.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if certInfo, ok := secretInfo.Data["cert"]; ok { // TODO fanyangyang key name
		if err := saveORUpdateFile(certInfo); err != nil {
			return err
		}

	} else {
		logrus.Warnf("docker secret:%s do not contain cert info", defaultSecretName)
	}
	return nil
}

// sync as file saved int /etc/docker/goodrain.me/server.crt
func saveORUpdateFile(content []byte) error {
	// If path is already a directory, MkdirAll does nothing and returns nil
	if err := os.MkdirAll(defaultFilePath, 0666); err != nil {
		logrus.Errorf("mkdir path(%s) error: %s", defaultFilePath, err.Error())
		return err
	}
	logrus.Debugf("mkdir path(%s) successfully", defaultFilePath)
	dest := path.Join(defaultFilePath, defaultFileName)
	// Create creates the named file with mode 0666 (before umask), truncating it if it already exists
	file, err := os.Create(dest)
	if err != nil {
		return err
	}

	_, err = io.Copy(file, strings.NewReader(string(content)))
	return err
}
