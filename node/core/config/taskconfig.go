// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package config

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/sirupsen/logrus"
)

//GroupContext 组任务会话
type GroupContext struct {
	ctx     context.Context
	groupID string
}

//NewGroupContext 创建组配置会话
func NewGroupContext(groupID string) *GroupContext {
	return &GroupContext{
		ctx:     context.Background(),
		groupID: groupID,
	}
}

//Add 添加配置项
func (g *GroupContext) Add(k, v interface{}) {
	g.ctx = context.WithValue(g.ctx, k, v)
	store.DefalutClient.Put(fmt.Sprintf("%s/group/%s/%s", option.Config.ConfigStoragePath, g.groupID, k), v.(string))
}

//Get get
func (g *GroupContext) Get(k interface{}) interface{} {
	if v := g.ctx.Value(k); v != nil {
		return v
	}
	res, _ := store.DefalutClient.Get(fmt.Sprintf("%s/group/%s/%s", option.Config.ConfigStoragePath, g.groupID, k))
	if res.Count > 0 {
		return string(res.Kvs[0].Value)
	}
	return ""
}

//GetString get
func (g *GroupContext) GetString(k interface{}) string {
	if v := g.ctx.Value(k); v != nil {
		return v.(string)
	}
	res, _ := store.DefalutClient.Get(fmt.Sprintf("%s/group/%s/%s", option.Config.ConfigStoragePath, g.groupID, k))
	if res.Count > 0 {
		return string(res.Kvs[0].Value)
	}
	return ""
}

var reg = regexp.MustCompile(`(?U)\$\{.*\}`)

//GetConfigKey 获取配置key
func GetConfigKey(rk string) string {
	if len(rk) < 4 {
		return ""
	}
	left := strings.Index(rk, "{")
	right := strings.Index(rk, "}")
	return rk[left+1 : right]
}

//ResettingArray 根据实际配置解析数组字符串
func ResettingArray(groupCtx *GroupContext, source []string) ([]string, error) {
	sourcecopy := make([]string, len(source))
	// 使用copy
	for i, s := range source {
		sourcecopy[i] = s
	}
	for i, s := range sourcecopy {
		resultKey := reg.FindAllString(s, -1)
		for _, rk := range resultKey {
			key := strings.ToUpper(GetConfigKey(rk))
			// if len(key) < 1 {
			// 	return nil, fmt.Errorf("%s Parameter configuration error.please make sure `${XXX}`", s)
			// }
			value := GetConfig(groupCtx, key)
			sourcecopy[i] = strings.Replace(s, rk, value, -1)
		}
	}
	return sourcecopy, nil
}

//GetConfig 获取配置信息
func GetConfig(groupCtx *GroupContext, key string) string {
	if groupCtx != nil {
		value := groupCtx.Get(key)
		if value != nil {
			switch value.(type) {
			case string:
				if value.(string) != "" {
					return value.(string)
				}
			case int:
				if value.(int) != 0 {
					return strconv.Itoa(value.(int))
				}
			case []string:
				if value.([]string) != nil {
					result := strings.Join(value.([]string), ",")
					if strings.HasSuffix(result, ",") {
						return result
					}
					return result + ","
				}
			case []interface{}:
				if value.([]interface{}) != nil && len(value.([]interface{})) > 0 {
					result := ""
					for _, v := range value.([]interface{}) {
						switch v.(type) {
						case string:
							result += v.(string) + ","
						case int:
							result += strconv.Itoa(v.(int)) + ","
						}
					}
					return result
				}
			}
		}
	}
	if dataCenterConfig == nil {
		return ""
	}
	cn := dataCenterConfig.GetConfig(key)
	if cn != nil && cn.Value != nil {
		if cn.ValueType == "string" || cn.ValueType == "" {
			return cn.Value.(string)
		}
		if cn.ValueType == "array" {
			switch cn.Value.(type) {
			case []string:
				return strings.Join(cn.Value.([]string), ",")
			case []interface{}:
				vas := cn.Value.([]interface{})
				result := ""
				for _, va := range vas {
					switch va.(type) {
					case string:
						result += va.(string) + ","
					case int:
						result += strconv.Itoa(va.(int)) + ","
					}
				}
				return result
			}
		}
		if cn.ValueType == "int" {
			return strconv.Itoa(cn.Value.(int))
		}
	}
	logrus.Warnf("can not find config for key %s", key)
	return ""
}

//ResettingString 根据实际配置解析字符串
func ResettingString(groupCtx *GroupContext, source string) (string, error) {
	resultKey := reg.FindAllString(source, -1)
	for _, rk := range resultKey {
		key := strings.ToUpper(GetConfigKey(rk))
		// if len(key) < 1 {
		// 	return nil, fmt.Errorf("%s Parameter configuration error.please make sure `${XXX}`", s)
		// }
		value := GetConfig(groupCtx, key)
		source = strings.Replace(source, rk, value, -1)
	}
	return source, nil
}

//ResettingMap 根据实际配置解析Map字符串
func ResettingMap(groupCtx *GroupContext, source map[string]string) (map[string]string, error) {
	sourcecopy := make(map[string]string, len(source))
	for k, v := range source {
		sourcecopy[k] = v
	}
	for k, s := range sourcecopy {
		resultKey := reg.FindAllString(s, -1)
		for _, rk := range resultKey {
			key := strings.ToUpper(GetConfigKey(rk))
			// if len(key) < 1 {
			// 	return nil, fmt.Errorf("%s Parameter configuration error.please make sure `${XXX}`", s)
			// }
			value := GetConfig(groupCtx, key)
			sourcecopy[k] = strings.Replace(s, rk, value, -1)
		}
	}
	return sourcecopy, nil
}
