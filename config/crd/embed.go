package crd

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

//go:embed *.yaml
var crdFS embed.FS

// EnsureCRDs applies all embedded CRD YAML files to the cluster.
// It creates new CRDs or updates existing ones to match the latest schema.
func EnsureCRDs(ctx context.Context, config *rest.Config) error {
	client, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create apiextensions client: %w", err)
	}

	entries, err := crdFS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read embedded CRD dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		data, err := crdFS.ReadFile(entry.Name())
		if err != nil {
			logrus.Warningf("read embedded CRD %s: %v", entry.Name(), err)
			continue
		}
		if err := applyCRD(ctx, client, data); err != nil {
			logrus.Warningf("apply CRD %s: %v", entry.Name(), err)
		}
	}
	return nil
}

func applyCRD(ctx context.Context, client *apiextensionsclient.Clientset, data []byte) error {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	crdClient := client.ApiextensionsV1().CustomResourceDefinitions()
	existing, err := crdClient.Get(ctx, crd.Name, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("get CRD %s: %w", crd.Name, err)
		}
		// Create
		if _, err := crdClient.Create(ctx, &crd, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create CRD %s: %w", crd.Name, err)
		}
		logrus.Infof("CRD %s created", crd.Name)
		return nil
	}
	// Update: preserve resourceVersion
	crd.ResourceVersion = existing.ResourceVersion
	if _, err := crdClient.Update(ctx, &crd, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update CRD %s: %w", crd.Name, err)
	}
	logrus.Infof("CRD %s updated", crd.Name)
	return nil
}
