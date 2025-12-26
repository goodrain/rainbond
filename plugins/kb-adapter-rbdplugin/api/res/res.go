// Package res 用于构建响应和定义响应结构体
//
// 在 handler 中应当使用 res 包返回响应或错误
package res

import (
	"net/http"
	"reflect"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/labstack/echo/v4"
)

type ConnectInfoRes struct {
	ConnectInfos []model.ConnectInfo `json:"connect_infos"`
	Port         int                 `json:"port"`
}

type RestoreFromBackupRes struct {
	NewClusterName string `json:"new_service"`
}

// Response 定义用于正确返回的 JSON
type Response struct {
	Bean          any `json:"bean,omitempty"`
	List          any `json:"list,omitempty"`
	ListAllNumber int `json:"number,omitempty"`
	Page          int `json:"page,omitempty"`
}

// ReturnSuccess -
func ReturnSuccess(c echo.Context, data any) error {
	if data == nil {
		return c.JSON(http.StatusOK, Response{Bean: nil})
	}

	// TODO 优化反射
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Slice {
		return c.JSON(http.StatusOK, Response{List: data})
	}

	return c.JSON(http.StatusOK, Response{Bean: data})
}

// ReturnList 返回分页列表
func ReturnList(c echo.Context, total, page int, list any) error {
	return c.JSON(http.StatusOK, Response{
		List:          list,
		ListAllNumber: total,
		Page:          page,
	})
}
