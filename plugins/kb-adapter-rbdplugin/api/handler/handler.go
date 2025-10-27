package handler

import (
	"fmt"

	"github.com/furutachiKurea/block-mechanica/api/req"
	"github.com/furutachiKurea/block-mechanica/api/res"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service"

	"github.com/labstack/echo/v4"
)

// Handler 处理 API 请求，集合了 Block Mechanica 的各个服务
type Handler struct {
	svc service.Services
}

// NewHandler -
func NewHandler(svc service.Services) *Handler {
	return &Handler{svc: svc}
}

// GetAddons 获取 Block Mechanica 支持的数据库类型
//
//	GET /v1/addons
func (h *Handler) GetAddons(c echo.Context) error {
	ctx := c.Request().Context()

	addons, err := h.svc.GetAddons(ctx)
	if err != nil {
		return res.InternalError(err)
	}
	return res.ReturnSuccess(c, addons)
}

// GetStorageClasses 集群中的 StorageClass
//
// GET /v1/storageclasses
func (h *Handler) GetStorageClasses(c echo.Context) error {
	ctx := c.Request().Context()
	storageClasses, err := h.svc.GetStorageClasses(ctx)
	if err != nil {
		return res.InternalError(fmt.Errorf("get storage classes: %w", err))
	}
	return res.ReturnSuccess(c, storageClasses)
}

// GetBackupRepos 集群中设置的 BackupRepo
//
// GET /v1/backuprepos
func (h *Handler) GetBackupRepos(c echo.Context) error {
	ctx := c.Request().Context()
	repos, err := h.svc.ListAvailableBackupRepos(ctx)
	if err != nil {
		return res.InternalError(fmt.Errorf("list available backup repos: %w", err))
	}
	return res.ReturnSuccess(c, repos)
}

// CreateCluster 创建 KubeBlocks 数据库集群
//
// POST /v1/clusters
//
// 完成之后不保证 Cluster 与 KubeBlocks Component 就绪
func (h *Handler) CreateCluster(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.ClusterRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	log.Info("CreateCluster", log.Any("request", request))

	modelReq := model.ClusterInput{
		ClusterInfo: model.ClusterInfo{
			Name:              request.Name,
			Namespace:         request.Namespace,
			Type:              request.Type,
			Version:           request.Version,
			StorageClass:      request.StorageClass,
			TerminationPolicy: request.TerminationPolicy,
		},
		ClusterResource: model.ClusterResource{
			CPU:      request.CPU,
			Memory:   request.Memory,
			Storage:  request.Storage,
			Replicas: request.Replicas,
		},
		ClusterBackup: model.ClusterBackup{
			BackupRepo: request.BackupRepo,
			Schedule: model.BackupSchedule{
				Frequency: request.Schedule.Frequency,
				DayOfWeek: request.Schedule.DayOfWeek,
				Hour:      request.Schedule.Hour,
				Minute:    request.Schedule.Minute,
			},
			RetentionPeriod: request.RetentionPeriod,
		},
		RBDService: model.RBDService{
			ServiceID: request.RBDService.ServiceID,
		},
	}

	cluster, err := h.svc.CreateCluster(ctx, modelReq)
	if err != nil {
		return res.InternalError(fmt.Errorf("create cluster: %w", err))
	}

	return res.ReturnSuccess(c, cluster)
}

// CancelClusterCreate 取消集群创建
//
// DELETE /v1/clusters/:service-id
func (h *Handler) CancelClusterCreate(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.ClusterRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	rbdService := model.RBDService{
		ServiceID: request.RBDService.ServiceID,
	}

	if err := h.svc.CancelClusterCreate(ctx, rbdService); err != nil {
		return res.InternalError(fmt.Errorf("cancel cluster create: %w", err))
	}

	return res.ReturnSuccess(c, "Cancled")
}

// DeleteCluster 删除 KubeBlocks 数据库集群
//
// DELETE /v1/clusters
func (h *Handler) DeleteCluster(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.DeleteClustersRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	if err := h.svc.DeleteClusters(ctx, request.ServiceIDs); err != nil {
		return res.InternalError(fmt.Errorf("delete clusters: %w", err))
	}

	return res.ReturnSuccess(c, "Deleted")
}

// GetConnectInfo 获取 KubeBlocks 数据库集群的连接信息
//
// GET /v1/clusters/connect-info
func (h *Handler) GetConnectInfo(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.ClusterRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	connectInfos, err := h.svc.GetConnectInfo(ctx, request.RBDService)
	if err != nil {
		return res.InternalError(fmt.Errorf("get connect info: %w", err))
	}

	response := &res.ConnectInfoRes{
		ConnectInfos: connectInfos,
		Port:         h.svc.GetClusterPort(ctx, request.RBDService.ServiceID),
	}

	return res.ReturnSuccess(c, response)
}

// GetClusterDetail 获取 KubeBlocks 数据库集群的详细信息
//
// GET /v1/clusters/:service-id
func (h *Handler) GetClusterDetail(c echo.Context) error {
	ctx := c.Request().Context()

	var rbdService model.RBDService
	if err := c.Bind(&rbdService); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	detail, err := h.svc.GetClusterDetail(ctx, rbdService)
	if err != nil {
		return res.InternalError(fmt.Errorf("get cluster detail: %w", err))
	}

	return res.ReturnSuccess(c, detail)
}

// ExpansionCluster 对 KubeBlocks 数据库集群进行伸缩操作
//
// PUT /v1/clusters/:service-id/expansions
func (h *Handler) ExpansionCluster(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.ClusterRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	modelReq := model.ExpansionInput{
		RBDService: request.RBDService,
		ClusterResource: model.ClusterResource{
			CPU:      request.CPU,
			Memory:   request.Memory,
			Storage:  request.Storage,
			Replicas: request.Replicas,
		},
	}

	if err := h.svc.ExpansionCluster(ctx, modelReq); err != nil {
		return res.InternalError(fmt.Errorf("expansion cluster: %w", err))
	}

	return res.ReturnSuccess(c, "Done")
}

// ReScheduleBackup 重新调度 KubeBlocks 数据库集群的备份配置
//
// PUT /v1/clusters/:service-id/backup-schedules
func (h *Handler) ReScheduleBackup(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.BackupRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	schedule := model.BackupScheduleInput{
		RBDService: request.RBDService,
		ClusterBackup: model.ClusterBackup{
			BackupRepo:      request.BackupRepo,
			Schedule:        request.Schedule,
			RetentionPeriod: request.RetentionPeriod,
		},
	}

	if err := h.svc.ReScheduleBackup(ctx, schedule); err != nil {
		return res.InternalError(fmt.Errorf("reschedule backupe: %w", err))
	}

	return res.ReturnSuccess(c, "Done")
}

// GetBackups 获取 KubeBlocks 数据库集群的备份列表
//
// GET /v1/clusters/:service-id/backups
func (h *Handler) GetBackups(c echo.Context) error {
	ctx := c.Request().Context()

	var query model.BackupListQuery
	if err := c.Bind(&query); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	result, err := h.svc.ListBackups(ctx, query)
	if err != nil {
		return res.InternalError(fmt.Errorf("get backup list: %w", err))
	}

	return res.ReturnList(c, result.Total, query.Page, result.Items)
}

// CreateBackup 创建 KubeBlocks 数据库集群的备份
//
// POST /v1/clusters/:service-id/backups
func (h *Handler) CreateBackup(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.BackupRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	backupInput := model.BackupInput{
		RBDService: request.RBDService,
	}

	if err := h.svc.BackupCluster(ctx, backupInput); err != nil {
		return res.InternalError(fmt.Errorf("create backup: %w", err))
	}

	return res.ReturnSuccess(c, "Done")
}

// DeleteBackups 批量删除 KubeBlocks 数据库集群的指定备份
//
// DELETE /v1/clusters/:service-id/backups
func (h *Handler) DeleteBackups(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.DeleteBackupsRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	log.Info("Wanted to delete backups", log.Any("backups", request.Backups))

	deleted, err := h.svc.DeleteBackups(ctx, request.RBDService, request.Backups)
	if err != nil {
		return res.InternalError(fmt.Errorf("delete backups: %w", err))
	}

	log.Info("Deleted backups", log.Any("deleted", deleted))

	return res.ReturnSuccess(c, deleted)
}

func (h *Handler) ManageCluster(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.ManageClusterLifecycleRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	result := h.svc.ManageClustersLifecycle(ctx, request.ManageClusterType(), request.ServiceIDs)
	if len(result.Succeeded) == 0 {
		return res.InternalError(res.NewBatchOperationError("manage clusters", result.Failed))
	}

	return res.ReturnSuccess(c, result)
}

func (h *Handler) GetPodDetail(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.GetPodDetailRequest
	log.Debug("GetPodDetail", log.Any("request", request))
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	podDetail, err := h.svc.GetPodDetail(ctx, request.ServiceID, request.PodName)
	if err != nil {
		return res.InternalError(fmt.Errorf("get pod detail: %w", err))
	}

	return res.ReturnSuccess(c, podDetail)
}

func (h *Handler) GetClusterEvents(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.GetClusterEventsRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	result, err := h.svc.GetClusterEvents(ctx, request.ServiceID, request.Pagination)
	if err != nil {
		return res.InternalError(fmt.Errorf("get cluster events: %w", err))
	}

	return res.ReturnList(c, result.Total, request.Page, result.Items)
}

// GetClusterParameters 返回 service-id 对应的 KubeBlocks Cluster 的参数设置,
// 返回的数据结构还包括参数的约束。
// ErrTargetNotFound 错误表示该数据库不支持参数设置，不应作为业务错误处理，只返回空列表。
//
// GET /v1/clusters/:service-id/parameters
func (h *Handler) GetClusterParameters(c echo.Context) error {
	ctx := c.Request().Context()

	var query model.ClusterParametersQuery
	if err := c.Bind(&query); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	result, err := h.svc.GetClusterParameter(ctx, query)
	if err != nil {
		return res.InternalError(fmt.Errorf("get cluster parameters: %w", err))
	}

	return res.ReturnList(c, result.Total, query.Page, result.Items)
}

// ChangeClusterParameter 变更 KubeBlocks 数据库集群的参数配置
// 无论是否有参数变更，都应返回 200
//
// POST /v1/clusters/:service-id/parameters
func (h *Handler) ChangeClusterParameter(c echo.Context) error {
	ctx := c.Request().Context()

	var change model.ClusterParametersChange
	if err := c.Bind(&change); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	log.Debug("ChangeClusterParameter", log.Any("change", change))

	result, err := h.svc.ChangeClusterParameter(ctx, change)
	if err != nil {
		return res.InternalError(fmt.Errorf("change cluster parameters: %w", err))
	}

	log.Debug("parameter change response",
		log.String("serviceID", change.ServiceID),
		log.Any("appliedCount", result.Applied),
		log.Any("invalidCount", result.Invalids),
	)

	return res.ReturnSuccess(c, result)
}

func (h *Handler) RestoreClusterFromBackup(c echo.Context) error {
	ctx := c.Request().Context()

	var request req.RestoreFromBackupRequest
	if err := c.Bind(&request); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	restoredCluster, err := h.svc.RestoreFromBackup(ctx, request.ServiceID, request.NewServiceID, request.BackupName)
	if err != nil {
		return res.InternalError(fmt.Errorf("restore cluster from backup: %w", err))
	}

	response := &res.RestoreFromBackupRes{
		NewClusterName: restoredCluster,
	}

	return res.ReturnSuccess(c, response)
}
