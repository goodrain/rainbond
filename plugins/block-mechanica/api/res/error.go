package res

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// newError 创建一个 echo.HTTPError
func newError(code int, message string) error {
	return echo.NewHTTPError(code, message)
}

func InternalError(err error) error {
	message := err.Error()
	return newError(http.StatusInternalServerError, message)
}

func BadRequest(err error) error {
	message := err.Error()
	return newError(http.StatusBadRequest, message)
}

func NotFound(err error) error {
	message := err.Error()
	return newError(http.StatusNotFound, message)
}

func Unauthorized(err error) error {
	message := err.Error()
	return newError(http.StatusUnauthorized, message)
}

func Forbidden(err error) error {
	message := err.Error()
	return newError(http.StatusForbidden, message)
}
