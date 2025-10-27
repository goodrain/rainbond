// Package service 提供 Block Mechanica 的核心服务
//
// - Cluster: 提供 KubeBlocks 的 Cluster 相关操作
//
// - Resource: 提供 k8s 资源的相关操作
//
// - Backup: 提供 KubeBlocks 的 Backup 相关操作
package service

import (
	"context"

	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/backup"
	"github.com/furutachiKurea/block-mechanica/service/cluster"
	"github.com/furutachiKurea/block-mechanica/service/resource"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Services = (*DefaultServices)(nil)

// Services 聚合接口：供上层（handler/controller）使用
type Services interface {
	Resource
	Backup
	Cluster
}

// Backup 提供 KubeBlocks 的 Backup 相关操作
type Backup interface {
	// ListAvailableBackupRepos 返回所有 Available 的 BackupRepo
	ListAvailableBackupRepos(ctx context.Context) ([]*model.BackupRepo, error)

	// ReScheduleBackup 重新调度 Cluster 的备份配置
	//
	// 通过 Patch cluster 中的备份字段来实现 back schedule 的更新
	ReScheduleBackup(ctx context.Context, schedule model.BackupScheduleInput) error

	// BackupCluster 执行集群备份操作
	BackupCluster(ctx context.Context, backup model.BackupInput) error

	// ListBackups 返回给定的 Cluster 的备份列表
	ListBackups(ctx context.Context, query model.BackupListQuery) (*model.PaginatedResult[model.BackupItem], error)

	// DeleteBackups 批量删除指定备份
	//
	// 根据 service_id 查找对应的 Cluster，然后删除请求中指定名称的备份
	// 返回成功删除的备份名称列表
	DeleteBackups(ctx context.Context, rbd model.RBDService, backups []string) ([]string, error)
}

// Cluster 提供 KubeBlocks Cluster 相关操作
type Cluster interface {
	// CreateCluster 依据 req 创建 KubeBlocks Cluster
	//
	// 在创建 Cluster 时，会更新由 Rainbond 创建的 KubeBlocks Component (Deployment) 的 args
	// 以确保将来自其他 Rainbond 组件的连接请求转发至该 Cluster 的 Service 上;
	// 同时，通过将 service_id 添加至 Cluster 的 labels 中以关联 KubeBlocks Component 与 Cluster,
	// Rainbond 也通过这层关系来判断 Rainbond 组件是否为 KubeBlocks Component
	//
	// 返回成功创建的 KubeBlocks Cluster 实例
	CreateCluster(ctx context.Context, cluster model.ClusterInput) (*kbappsv1.Cluster, error)

	// DeleteClusters 删除 KubeBlocks 数据库集群
	//
	// 批量删除指定 serviceIDs 对应的 Cluster，忽略找不到的 service_id
	DeleteClusters(ctx context.Context, serviceIDs []string) error

	// CancelClusterCreate 取消集群创建
	//
	// 在删除前将 TerminationPolicy 调整为 WipeOut，确保 PVC/PV 等存储资源一并清理
	// https://kubeblocks.io/docs/preview/user_docs/references/api-reference/cluster#apps.kubeblocks.io/v1.TerminationPolicyType
	CancelClusterCreate(ctx context.Context, rbd model.RBDService) error

	// GetConnectInfo 获取指定 Cluster 的连接账户信息,
	// 从 Kubernetes Secret 中获取数据库账户的用户名和密码
	//
	// Secret 名称由对应数据库类型的 Coordinator 适配器生成
	GetConnectInfo(ctx context.Context, rbd model.RBDService) ([]model.ConnectInfo, error)

	// GetClusterDetail 通过 RBDService.ID 获取 Cluster 的详细信息
	GetClusterDetail(ctx context.Context, rbd model.RBDService) (*model.ClusterDetail, error)

	// ExpansionCluster 对 Cluster 进行伸缩操作
	//
	// 使用 opsrequest 将 Cluster 的资源规格进行伸缩，使其变为 req 的期望状态
	ExpansionCluster(ctx context.Context, expansion model.ExpansionInput) error

	// ManageClustersLifecycle 通过创建 OpsRequest 批量管理多个 Cluster 的生命周期
	//
	// 支持 operation: Start, Stop, Restart
	ManageClustersLifecycle(ctx context.Context, operation opsv1alpha1.OpsType, serviceIDs []string) *model.BatchOperationResult

	// GetPodDetail 获取指定 Cluster 的 Pod detail
	// 获取指定 service_id 的 Cluster 管理的指定 Pod 的详细信息
	GetPodDetail(ctx context.Context, serviceID string, podName string) (*model.PodDetail, error)

	// GetClusterEvents 获取指定 KubeBlocks Cluster 的 events
	//
	// 从 Cluster 拥有的 OpsRequest 中获取，按创建时间降序排序
	GetClusterEvents(ctx context.Context, serviceID string, pagination model.Pagination) (*model.PaginatedResult[model.EventItem], error)

	// RestoreFromBackup 从用户通过 backupName 指定的备份中 restore cluster，
	// 返回 restored cluster 的名称 + clusterDef, 用于 Rainbond 更新 KubeBlocks Component 信息
	//
	// 该方法将为恢复的 cluster 通过 newServiceID 绑定到一个新的 KubeBlocks Component 中
	RestoreFromBackup(ctx context.Context, oldServiceID, newServiceID, backupName string) (string, error)

	// GetClusterParameter 获取指定 KubeBlocks Cluster 的参数
	GetClusterParameter(ctx context.Context, query model.ClusterParametersQuery) (*model.PaginatedResult[model.Parameter], error)

	// ChangeClusterParameter 变更指定 KubeBlocks Cluster 的参数
	ChangeClusterParameter(ctx context.Context, req model.ClusterParametersChange) (*model.ParameterChangeResult, error)
}

// Resource 提供集群资源发现和 Rainbond 集成操作
type Resource interface {
	// GetAddons 获取所有可用的 Addon（数据库类型与版本）
	GetAddons(ctx context.Context) ([]*model.Addon, error)

	// GetStorageClasses 返回集群中所有的 StorageClass 的名称
	GetStorageClasses(ctx context.Context) (model.StorageClasses, error)

	// GetClusterPort 返回指定数据库在 KubeBlocks service 中的目标端口
	GetClusterPort(ctx context.Context, serviceID string) int
}

// DefaultServices 为聚合接口的默认实现，委托到具体子服务
type DefaultServices struct {
	Backup   Backup
	Cluster  Cluster
	Resource Resource
}

// NewServices 构建聚合服务实例
func NewServices(backup *backup.Service, cluster *cluster.Service, resource *resource.Service) Services {
	return &DefaultServices{
		Backup:   backup,
		Cluster:  cluster,
		Resource: resource,
	}
}

// New 构造 Services
func New(c client.Client) Services {
	return NewServices(
		backup.NewService(c),
		cluster.NewService(c),
		resource.NewService(c),
	)
}

// Backup

func (s *DefaultServices) ListAvailableBackupRepos(ctx context.Context) ([]*model.BackupRepo, error) {
	return s.Backup.ListAvailableBackupRepos(ctx)
}

func (s *DefaultServices) ReScheduleBackup(ctx context.Context, schedule model.BackupScheduleInput) error {
	return s.Backup.ReScheduleBackup(ctx, schedule)
}

func (s *DefaultServices) BackupCluster(ctx context.Context, backup model.BackupInput) error {
	return s.Backup.BackupCluster(ctx, backup)
}

func (s *DefaultServices) ListBackups(ctx context.Context, query model.BackupListQuery) (*model.PaginatedResult[model.BackupItem], error) {
	return s.Backup.ListBackups(ctx, query)
}

func (s *DefaultServices) DeleteBackups(ctx context.Context, rbd model.RBDService, backups []string) ([]string, error) {
	return s.Backup.DeleteBackups(ctx, rbd, backups)
}

// Cluster

func (s *DefaultServices) CreateCluster(ctx context.Context, cluster model.ClusterInput) (*kbappsv1.Cluster, error) {
	return s.Cluster.CreateCluster(ctx, cluster)
}

func (s *DefaultServices) GetConnectInfo(ctx context.Context, rbd model.RBDService) ([]model.ConnectInfo, error) {
	return s.Cluster.GetConnectInfo(ctx, rbd)
}

func (s *DefaultServices) GetClusterDetail(ctx context.Context, rbd model.RBDService) (*model.ClusterDetail, error) {
	return s.Cluster.GetClusterDetail(ctx, rbd)
}

func (s *DefaultServices) ExpansionCluster(ctx context.Context, expansion model.ExpansionInput) error {
	return s.Cluster.ExpansionCluster(ctx, expansion)
}

func (s *DefaultServices) DeleteClusters(ctx context.Context, serviceIDs []string) error {
	return s.Cluster.DeleteClusters(ctx, serviceIDs)
}

func (s *DefaultServices) CancelClusterCreate(ctx context.Context, rbd model.RBDService) error {
	return s.Cluster.CancelClusterCreate(ctx, rbd)
}

func (s *DefaultServices) ManageClustersLifecycle(ctx context.Context, operation opsv1alpha1.OpsType, serviceIDs []string) *model.BatchOperationResult {
	return s.Cluster.ManageClustersLifecycle(ctx, operation, serviceIDs)
}

func (s *DefaultServices) GetPodDetail(ctx context.Context, serviceID string, podName string) (*model.PodDetail, error) {
	return s.Cluster.GetPodDetail(ctx, serviceID, podName)
}

func (s *DefaultServices) GetClusterEvents(ctx context.Context, serviceID string, pagination model.Pagination) (*model.PaginatedResult[model.EventItem], error) {
	return s.Cluster.GetClusterEvents(ctx, serviceID, pagination)
}

func (s *DefaultServices) RestoreFromBackup(ctx context.Context, oldServiceID, newServiceID, backupName string) (string, error) {
	return s.Cluster.RestoreFromBackup(ctx, oldServiceID, newServiceID, backupName)
}

// Resource

func (s *DefaultServices) GetAddons(ctx context.Context) ([]*model.Addon, error) {
	return s.Resource.GetAddons(ctx)
}
func (s *DefaultServices) GetStorageClasses(ctx context.Context) (model.StorageClasses, error) {
	return s.Resource.GetStorageClasses(ctx)
}

func (s *DefaultServices) GetClusterPort(ctx context.Context, serviceID string) int {
	return s.Resource.GetClusterPort(ctx, serviceID)
}

func (s *DefaultServices) GetClusterParameter(ctx context.Context, query model.ClusterParametersQuery) (*model.PaginatedResult[model.Parameter], error) {
	return s.Cluster.GetClusterParameter(ctx, query)
}

func (s *DefaultServices) ChangeClusterParameter(ctx context.Context, req model.ClusterParametersChange) (*model.ParameterChangeResult, error) {
	return s.Cluster.ChangeClusterParameter(ctx, req)
}
