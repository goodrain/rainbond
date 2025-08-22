// Package service 提供 Block Mechanica 的核心服务
//
// - Cluster: 提供 KubeBlocks 的 Cluster 相关操作
//
// - Resource: 提供 k8s 资源的相关操作
//
// - Backup: 提供 KubeBlocks 的 Backup 相关操作
//
// - Rainbond: 提供 Rainbond 相关资源与 BlockMechanica 的关联判定等操作
package service

import (
	"context"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/adapter"
	"github.com/furutachiKurea/block-mechanica/service/backuper"
	"github.com/furutachiKurea/block-mechanica/service/builder"
	"github.com/furutachiKurea/block-mechanica/service/coordinator"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ServiceIDLabel = index.ServiceIDLabel
	ServiceIDField = index.ServiceIDField
)

// _clusterRegistry 在这里注册 Block Mechanica 支持的数据库集群，需要实现 adapter.ClusterAdapter 中的 Builder
var _clusterRegistry = map[string]adapter.ClusterAdapter{
	"postgresql": _postgresql,
	"mysql":      _mysql,
	// ... new types here
}

var (
	_postgresql = adapter.ClusterAdapter{
		Builder:     &builder.PostgreBuilder{},
		Coordinator: &coordinator.PostgreSQLCoordinator{},
		Backup:      &backuper.PostgreSQLBackuper{},
	}

	_mysql = adapter.ClusterAdapter{
		Builder:     &builder.MySQLBuilder{},
		Coordinator: &coordinator.MySQLCoordinator{},
		Backup:      &backuper.MySQLBackuper{},
	}
)

var _ Services = (*DefaultServices)(nil)

// Services 聚合接口：供上层（handler/controller）使用
type Services interface {
	Resource
	Backup
	Cluster
	Rainbond
}

// Resource 提供集群资源相关操作
type Resource interface {
	// GetAddons 获取所有可用的 Addon（数据库类型与版本）
	GetAddons(ctx context.Context) ([]*Addon, error)

	// GetStorageClasses 返回集群中所有的 StorageClass 的名称
	GetStorageClasses(ctx context.Context) (StorageClasses, error)
}

// Backup 提供 KubeBlocks 的 Backup 相关操作
type Backup interface {
	// ListAvailableBackupRepos 返回所有 Available 的 BackupRepo
	ListAvailableBackupRepos(ctx context.Context) ([]*BackupRepo, error)

	// ReScheduleBackup 重新调度 Cluster 的备份配置
	//
	// 通过 Patch cluster 中的备份字段来实现 back schedule 的更新
	ReScheduleBackup(ctx context.Context, schedule model.BackupScheduleInput) error

	// BackupCluster 执行集群备份操作
	BackupCluster(ctx context.Context, backup model.BackupInput) error

	// ListBackups 返回给定的 Cluster 的备份列表
	ListBackups(ctx context.Context, backupList model.BackupListQuerry) ([]*BackupItem, error)

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
	CreateCluster(ctx context.Context, cluster model.ClusterInput) error

	// DeleteCluster 删除 KubeBlocks 数据库集群
	//
	// 批量删除指定 serviceIDs 对应的 Cluster，忽略找不到的 service_id
	DeleteCluster(ctx context.Context, serviceIDs []string) error

	// CancelClusterCreate 取消集群创建
	//
	// 在删除前将 TerminationPolicy 调整为 WipeOut，确保 PVC/PV 等存储资源一并清理
	// https://kubeblocks.io/docs/preview/user_docs/references/api-reference/cluster#apps.kubeblocks.io/v1.TerminationPolicyType
	CancelClusterCreate(ctx context.Context, rbd model.RBDService) error

	// GetConnectInfo 获取指定 Cluster 的连接账户信息,
	// 从 Kubernetes Secret 中获取 root 账户的用户名和密码
	//
	// Secret 命名规则: {clustername}-{clustertype}-account-root
	GetConnectInfo(ctx context.Context, rbd model.RBDService) ([]model.ConnectInfo, error)

	// GetClusterDetail 通过 RBDService.ID 获取 Cluster 的详细信息
	GetClusterDetail(ctx context.Context, rbd model.RBDService) (*model.ClusterDetail, error)

	// ExpansionCluster 对 Cluster 进行伸缩操作
	//
	// 使用 opsrequest 将 Cluster 的资源规格进行伸缩，使其变为 req 的期望状态
	ExpansionCluster(ctx context.Context, expansion model.ExpansionInput) error

	// StartCluster 启动 Cluster
	StartCluster(ctx context.Context, cluster *kbappsv1.Cluster) error

	// StopCluster 停止 Cluster
	StopCluster(ctx context.Context, cluster *kbappsv1.Cluster) error
}

// Rainbond 提供 Rainbond 相关资源与 BlockMechanica 的关联判定
type Rainbond interface {
	// CheckKubeBlocksComponent 依据 RBDService 判定该 Rainbond 组件是否为 KubeBlocks Component，如果是，则还返回 KubeBlocks Component 对应的 Cluster 的数据库类型
	//
	// 如果给定的 req.RBDService.ID 能够匹配到一个 KubeBlocks Cluster，则说明该 Rainbond 组件为 KubeBlocks Component
	CheckKubeBlocksComponent(ctx context.Context, rbd model.RBDService) (*KubeBlocksComponentInfo, error)

	// GetClusterByServiceID 通过 service_id 获取对应的 KubeBlocks Cluster
	//
	// 封装 GetClusterByServiceID 方法
	GetClusterByServiceID(ctx context.Context, serviceID string) (*kbappsv1.Cluster, error)

	// GetKubeBlocksComponentByServiceID 通过 service_id 获取对应的 KubeBlocks Component（Rainbond 侧的 Deployment）
	//
	// 封装 getComponentByServiceID 方法
	GetKubeBlocksComponentByServiceID(ctx context.Context, serviceID string) (*appsv1.Deployment, error)

	// GetTargetPort 返回指定数据库类型在 KubeBlocks Service 中的目标端口
	GetTargetPort(dbType string) int

	// IsLegalType 判断数据库类型是否受支持（方法版）
	IsLegalType(dbType string) bool
}

// DefaultServices 为聚合接口的默认实现，委托到具体子服务
type DefaultServices struct {
	Backup   Backup
	Cluster  Cluster
	Resource Resource
	Rainbond Rainbond
}

// NewServices 构建聚合服务实例
func NewServices(backup *BackupService, cluster *ClusterService, resource *ResourceService, rainbond *RainbondService) Services {
	return &DefaultServices{
		Backup:   backup,
		Cluster:  cluster,
		Resource: resource,
		Rainbond: rainbond,
	}
}

// New 构造 Services
func New(c client.Client) Services {
	return NewServices(
		NewBackupService(c),
		NewClusterService(c),
		NewResourceService(c),
		NewRainbondService(c),
	)
}

// Resource

func (s *DefaultServices) GetAddons(ctx context.Context) ([]*Addon, error) {
	return s.Resource.GetAddons(ctx)
}
func (s *DefaultServices) GetStorageClasses(ctx context.Context) (StorageClasses, error) {
	return s.Resource.GetStorageClasses(ctx)
}

// Backup

func (s *DefaultServices) ListAvailableBackupRepos(ctx context.Context) ([]*BackupRepo, error) {
	return s.Backup.ListAvailableBackupRepos(ctx)
}
func (s *DefaultServices) ReScheduleBackup(ctx context.Context, schedule model.BackupScheduleInput) error {
	return s.Backup.ReScheduleBackup(ctx, schedule)
}
func (s *DefaultServices) BackupCluster(ctx context.Context, backup model.BackupInput) error {
	return s.Backup.BackupCluster(ctx, backup)
}
func (s *DefaultServices) ListBackups(ctx context.Context, backupList model.BackupListQuerry) ([]*BackupItem, error) {
	return s.Backup.ListBackups(ctx, backupList)
}
func (s *DefaultServices) DeleteBackups(ctx context.Context, rbd model.RBDService, backups []string) ([]string, error) {
	return s.Backup.DeleteBackups(ctx, rbd, backups)
}

// Cluster

func (s *DefaultServices) CreateCluster(ctx context.Context, cluster model.ClusterInput) error {
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
func (s *DefaultServices) StartCluster(ctx context.Context, cluster *kbappsv1.Cluster) error {
	return s.Cluster.StartCluster(ctx, cluster)
}
func (s *DefaultServices) StopCluster(ctx context.Context, cluster *kbappsv1.Cluster) error {
	return s.Cluster.StopCluster(ctx, cluster)
}
func (s *DefaultServices) DeleteCluster(ctx context.Context, serviceIDs []string) error {
	return s.Cluster.DeleteCluster(ctx, serviceIDs)
}
func (s *DefaultServices) CancelClusterCreate(ctx context.Context, rbd model.RBDService) error {
	return s.Cluster.CancelClusterCreate(ctx, rbd)
}

// Rainbond

func (s *DefaultServices) CheckKubeBlocksComponent(ctx context.Context, rbd model.RBDService) (*KubeBlocksComponentInfo, error) {
	return s.Rainbond.CheckKubeBlocksComponent(ctx, rbd)
}

func (s *DefaultServices) GetTargetPort(dbType string) int {
	return s.Rainbond.GetTargetPort(dbType)
}

func (s *DefaultServices) IsLegalType(dbType string) bool {
	return s.Rainbond.IsLegalType(dbType)
}

func (s *DefaultServices) GetClusterByServiceID(ctx context.Context, serviceID string) (*kbappsv1.Cluster, error) {
	return s.Rainbond.GetClusterByServiceID(ctx, serviceID)
}

func (s *DefaultServices) GetKubeBlocksComponentByServiceID(ctx context.Context, serviceID string) (*appsv1.Deployment, error) {
	return s.Rainbond.GetKubeBlocksComponentByServiceID(ctx, serviceID)
}

// init 函数进行注册表验证
func init() {
	validateClusterRegistry()
}

// validateClusterRegistry 验证集群注册表的完整性
func validateClusterRegistry() {
	for dbType, adapter := range _clusterRegistry {
		if err := adapter.Validate(); err != nil {
			log.Fatal("Critical validation error", log.String("DB Type", dbType), log.Err(err))
		}
		log.Info("Database validation passed", log.String("DB Type", dbType))
	}
	log.Info("All database validation passed")
}
