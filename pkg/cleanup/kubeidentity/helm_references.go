package kubeidentity

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type helmReleaseMetadata struct {
	item     metav1.PartialObjectMetadata
	name     string
	revision int
}

type helmReleasePayload struct {
	encoded   string
	inventory guard.RegionReferenceInventory
	err       error
}

func helmReleaseListOptions() metav1.ListOptions {
	return metav1.ListOptions{LabelSelector: "owner=helm"}
}

func readHelmReleasePayloads(ctx context.Context, client kubernetes.Interface, resource string, releases []helmReleaseMetadata) ([]helmReleasePayload, error) {
	type labeledPayload struct {
		uid, resourceVersion, encoded string
	}
	labeled := map[string]labeledPayload{}
	key := func(namespace, name string) string { return namespace + "\x00" + name }
	if resource == "secrets" {
		list, err := client.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, helmReleaseListOptions())
		if err != nil {
			return nil, fmt.Errorf("labeled Helm secret list failed: %w", err)
		}
		if list == nil || list.Continue != "" || len(list.Items) > 1024 {
			return nil, fmt.Errorf("labeled Helm secret inventory limit exceeded: %w", ErrBinding)
		}
		for index := range list.Items {
			item := &list.Items[index]
			labeled[key(item.Namespace, item.Name)] = labeledPayload{uid: string(item.UID), resourceVersion: item.ResourceVersion, encoded: string(item.Data["release"])}
		}
	} else {
		list, err := client.CoreV1().ConfigMaps(metav1.NamespaceAll).List(ctx, helmReleaseListOptions())
		if err != nil {
			return nil, fmt.Errorf("labeled Helm configmap list failed: %w", err)
		}
		if list == nil || list.Continue != "" || len(list.Items) > 1024 {
			return nil, fmt.Errorf("labeled Helm configmap inventory limit exceeded: %w", ErrBinding)
		}
		for index := range list.Items {
			item := &list.Items[index]
			labeled[key(item.Namespace, item.Name)] = labeledPayload{uid: string(item.UID), resourceVersion: item.ResourceVersion, encoded: item.Data["release"]}
		}
	}
	expectedLabeled := map[string]bool{}
	for _, release := range releases {
		if release.item.Labels["owner"] == "helm" {
			expectedLabeled[key(release.item.Namespace, release.item.Name)] = true
		}
	}
	if len(labeled) != len(expectedLabeled) {
		return nil, fmt.Errorf("labeled Helm inventory changed: %w", ErrBinding)
	}
	for identity := range labeled {
		if !expectedLabeled[identity] {
			return nil, fmt.Errorf("labeled Helm inventory changed: %w", ErrBinding)
		}
	}
	results := make([]helmReleasePayload, len(releases))
	parallel := make(chan struct{}, 8)
	var wait sync.WaitGroup
	for index := range releases {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			select {
			case parallel <- struct{}{}:
				defer func() { <-parallel }()
			case <-ctx.Done():
				results[index].err = fmt.Errorf("Helm release read canceled: %w", ctx.Err())
				return
			}
			release := releases[index]
			if release.item.Labels["owner"] == "helm" {
				payload, ok := labeled[key(release.item.Namespace, release.item.Name)]
				if !ok || payload.uid != string(release.item.UID) || payload.resourceVersion != release.item.ResourceVersion {
					results[index].err = fmt.Errorf("labeled Helm release identity changed: %w", ErrBinding)
					return
				}
				results[index].encoded = payload.encoded
				results[index].inventory = guard.InspectHelmReleaseReferences(payload.encoded, release.name, release.item.Namespace, release.revision)
				return
			}
			if resource == "secrets" {
				secret, err := client.CoreV1().Secrets(release.item.Namespace).Get(ctx, release.item.Name, metav1.GetOptions{})
				if err != nil {
					results[index].err = fmt.Errorf("Helm secret read failed: %w", err)
					return
				}
				if secret == nil || secret.UID != release.item.UID || secret.ResourceVersion != release.item.ResourceVersion {
					results[index].err = fmt.Errorf("Helm secret identity changed: %w", ErrBinding)
					return
				}
				results[index].encoded = string(secret.Data["release"])
			} else {
				config, err := client.CoreV1().ConfigMaps(release.item.Namespace).Get(ctx, release.item.Name, metav1.GetOptions{})
				if err != nil {
					results[index].err = fmt.Errorf("Helm configmap read failed: %w", err)
					return
				}
				if config == nil || config.UID != release.item.UID || config.ResourceVersion != release.item.ResourceVersion {
					results[index].err = fmt.Errorf("Helm configmap identity changed: %w", ErrBinding)
					return
				}
				results[index].encoded = config.Data["release"]
			}
			results[index].inventory = guard.InspectHelmReleaseReferences(results[index].encoded, release.name, release.item.Namespace, release.revision)
		}(index)
	}
	wait.Wait()
	for index, result := range results {
		if result.err != nil {
			return nil, result.err
		}
		if !result.inventory.Complete {
			release := releases[index]
			logrus.Warnf("cleanup Helm release reference incomplete: resource=%s namespace=%s release=%s revision=%d payload_bytes=%d",
				resource, release.item.Namespace, release.name, release.revision, len(result.encoded))
		}
	}
	return results, nil
}

// ReadHelmReferenceInventory fetches only standard owner=helm release payloads.
// Every retained labeled revision and hook is protective. Unrelated Secret
// values and chart configuration are never read.
func ReadHelmReferenceInventory(ctx context.Context, client kubernetes.Interface) (guard.RegionReferenceInventory, error) {
	denied := guard.RegionReferenceInventory{}
	if client == nil {
		return denied, fmt.Errorf("helm clients unavailable: %w", ErrBinding)
	}
	result := guard.RegionReferenceInventory{Complete: true, Images: []string{}}
	images := map[string]bool{}
	records, encodedBytes := 0, 0
	for _, resource := range []string{"secrets", "configmaps"} {
		seen := map[string]bool{}
		resourceReleases := []helmReleaseMetadata{}
		list := &metav1.PartialObjectMetadataList{}
		if resource == "secrets" {
			items, err := client.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, helmReleaseListOptions())
			if err != nil {
				return denied, fmt.Errorf("%s metadata list failed: %w", resource, err)
			}
			list.ListMeta = items.ListMeta
			for index := range items.Items {
				list.Items = append(list.Items, metav1.PartialObjectMetadata{ObjectMeta: items.Items[index].ObjectMeta})
			}
		} else {
			items, err := client.CoreV1().ConfigMaps(metav1.NamespaceAll).List(ctx, helmReleaseListOptions())
			if err != nil {
				return denied, fmt.Errorf("%s metadata list failed: %w", resource, err)
			}
			list.ListMeta = items.ListMeta
			for index := range items.Items {
				list.Items = append(list.Items, metav1.PartialObjectMetadata{ObjectMeta: items.Items[index].ObjectMeta})
			}
		}
		if list == nil || list.ResourceVersion == "" {
			return denied, fmt.Errorf("%s metadata snapshot missing: %w", resource, ErrBinding)
		}
		if list.Continue != "" || len(list.Items) > 1024 {
			return denied, fmt.Errorf("%s metadata snapshot limit exceeded: %w", resource, ErrBinding)
		}
		for _, item := range list.Items {
			if item.Name == "" || item.Namespace == "" || item.UID == "" || item.ResourceVersion == "" || seen[string(item.UID)] {
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
			resourceReleases = append(resourceReleases, helmReleaseMetadata{item: item, name: name, revision: revision})
		}
		payloads, err := readHelmReleasePayloads(ctx, client, resource, resourceReleases)
		if err != nil {
			return denied, err
		}
		for _, payload := range payloads {
			encodedBytes += len(payload.encoded)
			if encodedBytes > 32<<20 {
				return denied, fmt.Errorf("Helm release payload limit exceeded: %w", ErrBinding)
			}
			inventory := payload.inventory
			result.Complete = result.Complete && inventory.Complete
			result.HelmReleases = append(result.HelmReleases, inventory.HelmReleases...)
			for _, image := range inventory.Images {
				if len(images) >= 20000 && !images[image] {
					return denied, ErrBinding
				}
				images[image] = true
			}
		}
	}
	for image := range images {
		result.Images = append(result.Images, image)
	}
	sort.Strings(result.Images)
	return result, nil
}
