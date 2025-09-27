package res

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type BatchOperationError struct {
	msg    string
	errors map[string]error
}

func NewBatchOperationError(msg string, errors map[string]error) *BatchOperationError {
	return &BatchOperationError{
		msg:    msg,
		errors: errors,
	}
}

func (e *BatchOperationError) Error() string {
	var errMsgs []string
	for serviceID, err := range e.errors {
		errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", serviceID, err))
	}
	return fmt.Sprintf("%s: %s", e.msg, strings.Join(errMsgs, "; "))
}

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
