package log

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// EchoZap 使用 zap 的 echo RequestLogger 中间件
//
// https://echo.labstack.com/docs/middleware/logger#new-requestlogger-middleware
func EchoZap() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogError:     true,
		LogRemoteIP:  true,
		LogUserAgent: true,
		LogLatency:   true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			getLoggerWithCallerSkip().Info("request",
				String("method", v.Method),
				String("URI", v.URI),
				Int("status", v.Status),
				Err(v.Error),
				String("remote_ip", v.RemoteIP),
				String("user_agent", v.UserAgent),
				Duration("latency", v.Latency),
			)
			return nil
		},
	})
}
