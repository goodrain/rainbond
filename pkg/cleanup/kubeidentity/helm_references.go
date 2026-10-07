package kubeidentity

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
)

// ReadHelmReferenceInventory lists metadata first and fetches only Helm release
// payloads. Every retained revision and hook is protective, including records
// whose owner label was removed. No Secret value or chart config is returned.
func ReadHelmReferenceInventory(ctx context.Context, client kubernetes.Interface, metaClient metadata.Interface) (guard.RegionReferenceInventory, error) {
	denied := guard.RegionReferenceInventory{}
	if client == nil || metaClient == nil {
		return denied, fmt.Errorf("helm clients unavailable: %w", ErrBinding)
	}
	result := guard.RegionReferenceInventory{Complete: true, Images: []string{}}
	images := map[string]bool{}
	records, encodedBytes := 0, 0
	for _, resource := range []string{"secrets", "configmaps"} {
		cursor, version := "", ""
		cursors := map[string]bool{}
		seen := map[string]bool{}
		for pages := 0; ; pages++ {
			if pages >= 128 || ctx.Err() != nil {
				return denied, fmt.Errorf("%s metadata pagination stopped: %w", resource, ErrBinding)
			}
			list, err := metaClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: resource}).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 128, Continue: cursor})
			if err != nil {
				return denied, fmt.Errorf("%s metadata list failed: %w", resource, err)
			}
			if list == nil || list.ResourceVersion == "" {
				return denied, fmt.Errorf("%s metadata snapshot missing: %w", resource, ErrBinding)
			}
			if version != "" && version != list.ResourceVersion {
				return denied, fmt.Errorf("%s metadata snapshot changed: %w", resource, ErrBinding)
			}
			version = list.ResourceVersion
			for _, item := range list.Items {
				if item.Name == "" || item.Namespace == "" || item.UID == "" || item.ResourceVersion == "" || seen[string(item.UID)] || len(seen) >= 16384 {
					return denied, fmt.Errorf("%s metadata identity invalid: %w", resource, ErrBinding)
				}
				seen[string(item.UID)] = true
				const prefix = "sh.helm.release.v1."
				if !strings.HasPrefix(item.Name, prefix) {
					if item.Labels["owner"] == "helm" {
						return denied, fmt.Errorf("%s Helm owner name invalid: %w", resource, ErrBinding)
					}
					continue
				}
				end := strings.LastIndex(item.Name, ".v")
				if end <= len(prefix) {
					return denied, fmt.Errorf("%s Helm release name invalid: %w", resource, ErrBinding)
				}
				name := item.Name[len(prefix):end]
				revision, err := strconv.Atoi(item.Name[end+2:])
				if err != nil || revision < 1 || strconv.Itoa(revision) != item.Name[end+2:] {
					return denied, fmt.Errorf("%s Helm release revision invalid: %w", resource, ErrBinding)
				}
				records++
				if records > 1024 || ctx.Err() != nil {
					return denied, fmt.Errorf("Helm release inventory limit exceeded: %w", ErrBinding)
				}
				var encoded string
				if resource == "secrets" {
					secret, err := client.CoreV1().Secrets(item.Namespace).Get(ctx, item.Name, metav1.GetOptions{})
					if err != nil {
						return denied, fmt.Errorf("Helm secret read failed: %w", err)
					}
					if secret == nil || secret.UID != item.UID || secret.ResourceVersion != item.ResourceVersion {
						return denied, fmt.Errorf("Helm secret identity changed: %w", ErrBinding)
					}
					encoded = string(secret.Data["release"])
				} else {
					config, err := client.CoreV1().ConfigMaps(item.Namespace).Get(ctx, item.Name, metav1.GetOptions{})
					if err != nil {
						return denied, fmt.Errorf("Helm configmap read failed: %w", err)
					}
					if config == nil || config.UID != item.UID || config.ResourceVersion != item.ResourceVersion {
						return denied, fmt.Errorf("Helm configmap identity changed: %w", ErrBinding)
					}
					encoded = config.Data["release"]
				}
				encodedBytes += len(encoded)
				if encodedBytes > 32<<20 {
					return denied, fmt.Errorf("Helm release payload limit exceeded: %w", ErrBinding)
				}
				inventory := guard.InspectHelmReleaseReferences(encoded, name, item.Namespace, revision)
				result.Complete = result.Complete && inventory.Complete
				result.HelmReleases = append(result.HelmReleases, inventory.HelmReleases...)
				for _, image := range inventory.Images {
					if len(images) >= 20000 && !images[image] {
						return denied, ErrBinding
					}
					images[image] = true
				}
			}
			if list.Continue == "" {
				break
			}
			if cursors[list.Continue] {
				return denied, fmt.Errorf("%s metadata cursor repeated: %w", resource, ErrBinding)
			}
			cursors[list.Continue] = true
			cursor = list.Continue
		}
	}
	for image := range images {
		result.Images = append(result.Images, image)
	}
	sort.Strings(result.Images)
	return result, nil
}
