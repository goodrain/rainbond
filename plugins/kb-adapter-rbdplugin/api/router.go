package api

import (
	"net/http"
	"time"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/config"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/api/handler"

	"github.com/labstack/echo/v4"
)

// setupRouter 设置路由
func setupRouter(v1 *echo.Group, h *handler.Handler) {
	v1.GET("/addons", h.GetAddons)
	v1.GET("/storageclasses", h.GetStorageClasses)
	v1.GET("/backuprepos", h.GetBackupRepos)

	cluster := v1.Group("/clusters")
	{
		cluster.POST("", h.CreateCluster)
		cluster.DELETE("", h.DeleteCluster)
		cluster.GET("/connect-infos", h.GetConnectInfo)
		cluster.GET("/:service-id", h.GetClusterDetail)
		cluster.PUT("/:service-id", h.ExpansionCluster)
		cluster.PUT("/:service-id/backup-schedules", h.ReScheduleBackup)
		cluster.GET("/:service-id/backups", h.GetBackups)
		cluster.POST("/:service-id/backups", h.CreateBackup)
		cluster.DELETE("/:service-id/backups", h.DeleteBackups)
		cluster.POST("/actions", h.ManageCluster)
		cluster.GET("/:service-id/pods/:pod-name/details", h.GetPodDetail)
		cluster.GET("/:service-id/events", h.GetClusterEvents)
		cluster.GET("/:service-id/parameters", h.GetClusterParameters)
		cluster.POST("/:service-id/parameters", h.ChangeClusterParameter)
		cluster.POST("/:service-id/restores", h.RestoreClusterFromBackup)
	}
}

// setupHealthRoutes 健康检查路由
func setupHealthRoutes(e *echo.Echo, cfg *config.ServerConfig) {
	// ready
	e.GET(cfg.ReadinessPath, func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{
			"status":    "ready",
			"timestamp": time.Now().Unix(),
		})
	})

	// live
	e.GET(cfg.LivenessPath, func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{
			"status":    "alive",
			"timestamp": time.Now().Unix(),
		})
	})
}
