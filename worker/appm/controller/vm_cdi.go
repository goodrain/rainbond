package controller

import (
	"context"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var cdiConfigGVK = schema.GroupVersionKind{
	Group:   "cdi.kubevirt.io",
	Version: "v1beta1",
	Kind:    "CDIConfig",
}

var cdiGVK = schema.GroupVersionKind{
	Group:   "cdi.kubevirt.io",
	Version: "v1beta1",
	Kind:    "CDI",
}

func ensureVMRegistryImportInsecureRegistries(ctx context.Context, runtimeClient client.Client, templates []kubevirtv1.DataVolumeTemplateSpec) error {
	registries := vmRegistryImportInsecureRegistries(templates)
	if len(registries) == 0 || runtimeClient == nil {
		return nil
	}
	err := ensureCDIResourceInsecureRegistries(ctx, runtimeClient, registries)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	return ensureCDIConfigInsecureRegistries(ctx, runtimeClient, registries)
}

func ensureCDIResourceInsecureRegistries(ctx context.Context, runtimeClient client.Client, registries []string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cdi := newCDIObject()
		err := runtimeClient.Get(ctx, types.NamespacedName{Name: cdi.GetName()}, cdi)
		if err != nil {
			return err
		}
		existing, found, err := unstructured.NestedStringSlice(cdi.Object, "spec", "config", "insecureRegistries")
		if err != nil {
			return err
		}
		if !found {
			existing = nil
		}
		merged, changed := mergeVMRegistryList(existing, registries)
		if !changed {
			return nil
		}
		if err := unstructured.SetNestedStringSlice(cdi.Object, merged, "spec", "config", "insecureRegistries"); err != nil {
			return err
		}
		logrus.Infof("ensure CDI insecure registries for VM import: %v", registries)
		return runtimeClient.Update(ctx, cdi)
	})
}

func ensureCDIConfigInsecureRegistries(ctx context.Context, runtimeClient client.Client, registries []string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		config := newCDIConfigObject()
		err := runtimeClient.Get(ctx, types.NamespacedName{Name: config.GetName()}, config)
		if apierrors.IsNotFound(err) {
			config = newCDIConfigObject()
			if err := unstructured.SetNestedStringSlice(config.Object, registries, "spec", "insecureRegistries"); err != nil {
				return err
			}
			return runtimeClient.Create(ctx, config)
		}
		if err != nil {
			return err
		}
		existing, found, err := unstructured.NestedStringSlice(config.Object, "spec", "insecureRegistries")
		if err != nil {
			return err
		}
		if !found {
			existing = nil
		}
		merged, changed := mergeVMRegistryList(existing, registries)
		if !changed {
			return nil
		}
		if err := unstructured.SetNestedStringSlice(config.Object, merged, "spec", "insecureRegistries"); err != nil {
			return err
		}
		logrus.Infof("ensure CDIConfig insecure registries for VM import: %v", registries)
		return runtimeClient.Update(ctx, config)
	})
}

func newCDIObject() *unstructured.Unstructured {
	cdi := &unstructured.Unstructured{}
	cdi.SetGroupVersionKind(cdiGVK)
	cdi.SetName("cdi")
	return cdi
}

func newCDIConfigObject() *unstructured.Unstructured {
	config := &unstructured.Unstructured{}
	config.SetGroupVersionKind(cdiConfigGVK)
	config.SetName("config")
	return config
}

func vmRegistryImportInsecureRegistries(templates []kubevirtv1.DataVolumeTemplateSpec) []string {
	seen := map[string]struct{}{}
	registries := make([]string, 0)
	for _, template := range templates {
		if template.Spec.Source == nil || template.Spec.Source.Registry == nil || template.Spec.Source.Registry.URL == nil {
			continue
		}
		registry, ok := vmRegistryURLHost(*template.Spec.Source.Registry.URL)
		if !ok || !isInternalRBDHubRegistry(registry) {
			continue
		}
		if _, exists := seen[registry]; exists {
			continue
		}
		seen[registry] = struct{}{}
		registries = append(registries, registry)
	}
	return registries
}

func vmRegistryURLHost(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	return parsed.Host, true
}

func isInternalRBDHubRegistry(registry string) bool {
	host, _ := splitVMRegistryHostPort(strings.TrimSpace(registry))
	return host == "rbd-hub" || strings.HasPrefix(host, "rbd-hub.")
}

func splitVMRegistryHostPort(host string) (string, string) {
	index := strings.LastIndex(host, ":")
	if index <= 0 || strings.Contains(host[:index], ":") {
		return host, ""
	}
	return host[:index], host[index:]
}

func mergeVMRegistryList(existing, required []string) ([]string, bool) {
	seen := make(map[string]struct{}, len(existing)+len(required))
	merged := make([]string, 0, len(existing)+len(required))
	for _, registry := range existing {
		registry = strings.TrimSpace(registry)
		if registry == "" {
			continue
		}
		if _, ok := seen[registry]; ok {
			continue
		}
		seen[registry] = struct{}{}
		merged = append(merged, registry)
	}
	changed := len(merged) != len(existing)
	for _, registry := range required {
		registry = strings.TrimSpace(registry)
		if registry == "" {
			continue
		}
		if _, ok := seen[registry]; ok {
			continue
		}
		seen[registry] = struct{}{}
		merged = append(merged, registry)
		changed = true
	}
	return merged, changed
}
