// Package testutil 提供用于测试的 HTTP 相关工具函数
package testutil

import (
	"net/http"

	"github.com/creasty/defaults"
	"github.com/labstack/echo/v4"
)

// NewTestEcho 创建一个用于测试的 Echo 实例，配置了自定义的 binder 和 error handler
func NewTestEcho() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = customErrorHandler()
	e.Binder = customBinder()
	return e
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
		if he, ok := err.(*echo.HTTPError); ok {
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
// 使用 creasty/defaults 设置默认值，在结构体中使用 `default` 标签设置
func customBinder() echo.Binder {
	return &defaultsBinder{
		binder: &echo.DefaultBinder{},
	}
}

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
