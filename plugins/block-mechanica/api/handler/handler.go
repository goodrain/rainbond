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

// NewHandler
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
		return res.InternalError(fmt.Errorf("list available backuper repos: %w", err))
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

	var req req.ClusterRequest
	if err := c.Bind(&req); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	log.Info("CreateCluster", log.Any("req", req))

	modelReq := model.ClusterInput{
		ClusterInfo: model.ClusterInfo{
			Name:              req.Name,
			Namespace:         req.Namespace,
			Type:              req.Type,
			Version:           req.Version,
			StorageClass:      req.StorageClass,
			TerminationPolicy: req.TerminationPolicy,
		},
		ClusterResource: model.ClusterResource{
			CPU:      req.CPU,
			Memory:   req.Memory,
			Storage:  req.Storage,
			Replicas: req.Replicas,
		},
		ClusterBackup: model.ClusterBackup{
			BackupRepo: req.BackupRepo,
			Schedule: model.BackupSchedule{
				Frequency: model.BackupFrequency(req.Schedule.Frequency),
				DayOfWeek: req.Schedule.DayOfWeek,
				Hour:      req.Schedule.Hour,
				Minute:    req.Schedule.Minute,
			},
			RetentionPeriod: req.RetentionPeriod,
		},
		RBDService: model.RBDService{
			ServiceID: req.RBDService.ServiceID,
		},
	}

	if err := h.svc.CreateCluster(ctx, modelReq); err != nil {
		return res.InternalError(fmt.Errorf("create cluster: %w", err))
	}

	return res.ReturnSuccess(c, "Done")
}

// CancelClusterCreate 取消集群创建
//
// DELETE /v1/clusters/:service-id
func (h *Handler) CancelClusterCreate(c echo.Context) error {
	ctx := c.Request().Context()

	var req req.ClusterRequest
	if err := c.Bind(&req); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	rbdService := model.RBDService{
		ServiceID: req.RBDService.ServiceID,
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

	var req req.DeleteClustersRequest
	if err := c.Bind(&req); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	if err := h.svc.DeleteCluster(ctx, req.ServiceIDs); err != nil {
		return res.InternalError(fmt.Errorf("delete clusters: %w", err))
	}

	return res.ReturnSuccess(c, "Deleted")
}

// GetConnectInfo 获取 KubeBlocks 数据库集群的连接信息
//
// GET /v1/clusters/connect-info
func (h *Handler) GetConnectInfo(c echo.Context) error {
	ctx := c.Request().Context()

	var req req.ClusterRequest
	if err := c.Bind(&req); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	connectInfos, err := h.svc.GetConnectInfo(ctx, req.RBDService)
	if err != nil {
		return res.InternalError(fmt.Errorf("get connect info: %w", err))
	}

	return res.ReturnSuccess(c, connectInfos)
}

// CheckKubeBlocksComponent 通过给定的 service-id 判断该 Rainbond 组件是否为 KubeBlocks Component 并返回相关信息
//
// GET /v1/kubeblocks-component/:service-id
func (h *Handler) CheckKubeBlocksComponent(c echo.Context) error {
	ctx := c.Request().Context()

	log.Debug("CheckKubeBlocksComponent")

	var rbdService model.RBDService
	if err := c.Bind(&rbdService); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	log.Debug("bind request", log.Any("rbdService", rbdService))

	componentInfo, err := h.svc.CheckKubeBlocksComponent(ctx, rbdService)
	if err != nil {
		return res.InternalError(fmt.Errorf("check KubeBlocks component info: %w", err))
	}

	return res.ReturnSuccess(c, componentInfo)
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

	var req req.ClusterRequest
	if err := c.Bind(&req); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	modelReq := model.ExpansionInput{
		RBDService: req.RBDService,
		ClusterResource: model.ClusterResource{
			CPU:      req.CPU,
			Memory:   req.Memory,
			Storage:  req.Storage,
			Replicas: req.Replicas,
		},
	}

	if err := h.svc.ExpansionCluster(ctx, modelReq); err != nil {
		return res.InternalError(fmt.Errorf("expansion cluster: %w", err))
	}

	return res.ReturnSuccess(c, "Done")
}

// ReScheduleBackup 重新调度 KubeBlocks 数据库集群的备份配置
//
// PUT /v1/clusters/:service-id/backuper-schedules
func (h *Handler) ReScheduleBackup(c echo.Context) error {
	ctx := c.Request().Context()

	var req req.BackupRequest
	if err := c.Bind(&req); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	schedule := model.BackupScheduleInput{
		RBDService: req.RBDService,
		ClusterBackup: model.ClusterBackup{
			BackupRepo:      req.BackupRepo,
			Schedule:        req.Schedule,
			RetentionPeriod: req.RetentionPeriod,
		},
	}

	if err := h.svc.ReScheduleBackup(ctx, schedule); err != nil {
		return res.InternalError(fmt.Errorf("reschedule backuper: %w", err))
	}

	return res.ReturnSuccess(c, "Done")
}

// GetBackupList 获取 KubeBlocks 数据库集群的备份列表
//
// GET /v1/clusters/:service-id/backups
func (h *Handler) GetBackups(c echo.Context) error {
	ctx := c.Request().Context()

	var rbdService model.RBDService
	if err := c.Bind(&rbdService); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	backupListReq := model.BackupListQuerry{
		RBDService: rbdService,
	}

	backups, err := h.svc.ListBackups(ctx, backupListReq)
	if err != nil {
		return res.InternalError(fmt.Errorf("get backuper list: %w", err))
	}

	return res.ReturnSuccess(c, backups)
}

// CreateBackup 创建 KubeBlocks 数据库集群的备份
//
// POST /v1/clusters/:service-id/backups
func (h *Handler) CreateBackup(c echo.Context) error {
	ctx := c.Request().Context()

	var req req.BackupRequest
	if err := c.Bind(&req); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	backupInput := model.BackupInput{
		RBDService: req.RBDService,
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

	var req req.DeleteBackupsRequest
	if err := c.Bind(&req); err != nil {
		return res.BadRequest(fmt.Errorf("bind request: %w", err))
	}

	rbdService := model.RBDService{
		ServiceID: req.ServiceID,
	}

	deleted, err := h.svc.DeleteBackups(ctx, rbdService, req.Backups)
	if err != nil {
		return res.InternalError(fmt.Errorf("delete backups: %w", err))
	}

	return res.ReturnSuccess(c, deleted)
}
