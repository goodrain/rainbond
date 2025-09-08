package service

// resource.go 提供集群资源的相关操作

import (
	"context"
	"fmt"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/mono"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Addon 表示 KubeBlocks 支持的数据库类型及其版本
type Addon struct {
	Type    string   `json:"type"`
	Version []string `json:"version"`
}

type StorageClasses []string

// ResourceService 提供集群资源相关操作
type ResourceService struct {
	client client.Client
}

func NewResourceService(c client.Client) *ResourceService {
	return &ResourceService{
		client: c,
	}
}

// GetStorageClasses 返回集群中所有的 StorageClass 的名称
func (s *ResourceService) GetStorageClasses(ctx context.Context) (StorageClasses, error) {
	var scList storagev1.StorageClassList
	if err := s.client.List(ctx, &scList); err != nil {
		return nil, fmt.Errorf("list StorageClass: %w", err)
	}
	names := make([]string, 0, len(scList.Items))
	for _, sc := range scList.Items {
		names = append(names, sc.Name)
	}

	return StorageClasses(mono.Sorted(names)), nil
}

// GetAddons 获取所有可用的 Addon（数据库类型与版本）
func (s *ResourceService) GetAddons(ctx context.Context) ([]*Addon, error) {
	var cmpvList kbappsv1.ComponentVersionList
	if err := s.client.List(ctx, &cmpvList); err != nil {
		return nil, fmt.Errorf("get component version list: %w", err)
	}

	addons := make([]*Addon, 0, len(cmpvList.Items))
	for _, item := range cmpvList.Items {
		releases := make([]string, 0, len(item.Spec.Releases))
		for _, release := range item.Spec.Releases {
			releases = append(releases, release.ServiceVersion)
		}

		addon := &Addon{
			Type:    item.Name,
			Version: mono.Sorted(releases),
		}
		addons = append(addons, addon)
	}

	return mono.FilterThenSort(addons, filterSupportedAddons, func(a, b *Addon) bool {
		return a.Type < b.Type
	}), nil
}

// filterSupportedAddons mono.Filter 的过滤函数
//
// 仅返回在 _clusterRegistry 中声明过的数据库类型，确保返回值与系统实际可创建的类型一致。
// 判定是否受 Block Mechanica 支持时, 不同 toplogy 的 addon 视为同一类型
func filterSupportedAddons(addon *Addon) bool {
	t := addon.Type
	/*     if i := strings.LastIndex(t, "-"); i > 0 {
	       t = t[:i]
	   } */
	_, ok := _clusterRegistry[t]
	return ok
}
