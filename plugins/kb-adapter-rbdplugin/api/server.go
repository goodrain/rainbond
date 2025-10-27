// Package api Block Mechanica 提供的 API 服务
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/creasty/defaults"
	"github.com/furutachiKurea/block-mechanica/api/handler"
	"github.com/furutachiKurea/block-mechanica/internal/config"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/service"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Server 将 API 服务封装为 controller-runtime 的 Runnable
type Server struct {
	echo    *echo.Echo
	handler *handler.Handler
	config  *config.ServerConfig
}

func NewAPIServer(h *handler.Handler) *Server {
	e := echo.New()
	cfg := config.MustLoad()
	return &Server{echo: e, handler: h, config: cfg}
}

// Start 实现 manager.Runnable
func (r *Server) Start(ctx context.Context) error {
	if err := StartServerWithConfig(ctx, r.echo, r.handler, r.config); err != nil {
		return err
	}
	return nil
}

// NeedLeaderElection 实现 manager.LeaderElectionRunnable
func (r *Server) NeedLeaderElection() bool {
	return false
}

// RegisterServer 创建 Server 并注册至 manager
func RegisterServer(ctx context.Context, mgr ctrl.Manager, svcs service.Services) error {
	h := handler.NewHandler(svcs)
	apiServer := NewAPIServer(h)
	return mgr.Add(apiServer)
}

// StartServerWithConfig 使用配置启动服务器
func StartServerWithConfig(ctx context.Context, e *echo.Echo, handler *handler.Handler, cfg *config.ServerConfig) error {
	// custom echo
	e.HTTPErrorHandler = customErrorHandler()
	e.Binder = customBinder()

	// Middleware
	e.Use(middleware.Recover())
	e.Use(log.EchoZap()) // 使用 zap 日志中间件

	// 健康检查路由
	setupHealthRoutes(e, cfg)

	// 设置路由
	v1 := e.Group("/v1")
	setupRouter(v1, handler)

	// 启动服务器 - 直接启动，让 controller-runtime 管理生命周期
	serverAddr := cfg.Host + ":" + cfg.Port
	return e.Start(serverAddr)
}

// customErrorHandler 自定义 echo.HTTPErrorHandler
//
// 将错误处理返回的 JSON 格式设置为
//
//	{
//		"code": code,
//		"msg":  msg,
//	}
func customErrorHandler() echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		msg := "Internal Server Error"
		var he *echo.HTTPError
		if errors.As(err, &he) {
			code = he.Code
			if m, ok := he.Message.(string); ok {
				msg = m
			} else if m, ok := he.Message.(error); ok {
				msg = m.Error()
			}
		}
		_ = c.JSON(code, echo.Map{
			"code": code,
			"msg":  msg,
		})
	}
}

// customBinder 自定义 echo.Binder
//
// 使用 creasty/defaults 设置默认值，在结构体中通过 `default` 标签设置
func customBinder() echo.Binder {
	return &defaultsBinder{
		binder: &echo.DefaultBinder{},
	}
}

// defaultsBinder 自定义 echo.Binder, 允许使用 creasty/defaults 设置默认值
type defaultsBinder struct {
	binder *echo.DefaultBinder
}

func (b *defaultsBinder) Bind(i any, c echo.Context) error {
	// 标准绑定：处理 JSON、查询参数、路径参数等
	if err := b.binder.Bind(i, c); err != nil {
		return err
	}

	// 默认值设置：为未提供的字段设置默认值
	if err := defaults.Set(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to set defaults: "+err.Error())
	}

	return nil
}
