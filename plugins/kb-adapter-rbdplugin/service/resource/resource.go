// Package resource 提供集群资源相关操作
package resource

import (
	"context"
	"fmt"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/mono"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/kbkit"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/registry"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Service 提供集群资源相关操作
type Service struct {
	client client.Client
}

func NewService(c client.Client) *Service {
	return &Service{
		client: c,
	}
}

// GetStorageClasses 返回集群中所有的 StorageClass 的名称
func (s *Service) GetStorageClasses(ctx context.Context) (model.StorageClasses, error) {
	var scList storagev1.StorageClassList
	if err := s.client.List(ctx, &scList); err != nil {
		return nil, fmt.Errorf("list StorageClass: %w", err)
	}
	names := make([]string, 0, len(scList.Items))
	for _, sc := range scList.Items {
		names = append(names, sc.Name)
	}

	return mono.Sorted(names), nil
}

// GetAddons 获取所有可用的 Addon（数据库类型与版本）
func (s *Service) GetAddons(ctx context.Context) ([]*model.Addon, error) {
	var cmpvList kbappsv1.ComponentVersionList
	if err := s.client.List(ctx, &cmpvList); err != nil {
		return nil, fmt.Errorf("get component version list: %w", err)
	}

	addons := make([]*model.Addon, 0, len(cmpvList.Items))
	for _, item := range cmpvList.Items {
		releases := make([]string, 0, len(item.Spec.Releases))
		for _, release := range item.Spec.Releases {
			releases = append(releases, release.ServiceVersion)
		}

		addon := &model.Addon{
			Type:            item.Name,
			Version:         mono.Sorted(releases),
			IsSupportBackup: kbkit.IsSupportBackup(item.Name),
		}
		addons = append(addons, addon)
	}

	return mono.FilterThenSort(addons, filterSupportedAddons, func(a, b *model.Addon) bool {
		return a.Type < b.Type
	}), nil
}

// GetClusterPort 返回指定数据库在 KubeBlocks service 中的目标端口
func (s *Service) GetClusterPort(ctx context.Context, serviceID string) int {
	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, serviceID)
	if err != nil {
		return -1
	}
	adapter, ok := registry.Cluster[cluster.Spec.ClusterDef]
	if !ok {
		return -1
	}
	return adapter.Coordinator.TargetPort()
}

// filterSupportedAddons mono.Filter 的过滤函数
// 仅返回在 _clusterRegistry 中声明过的数据库类型，确保返回值与系统实际可创建的类型一致。
// 判定是否受 Block Mechanica 支持时, 不同 toplogy 的 addon 视为同一类型
func filterSupportedAddons(addon *model.Addon) bool {
	t := addon.Type
	// TODO
	/* if i := strings.LastIndex(t, "-"); i > 0 {
	    t = t[:i]
	} */
	_, ok := registry.Cluster[t]
	return ok
}
