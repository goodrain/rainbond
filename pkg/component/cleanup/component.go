// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

package cleanup

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
)

// CleanupComponent 本地文件清理组件
type CleanupComponent struct {
	cleanup *LocalFileCleanup
}

var defaultCleanupComponent *CleanupComponent

// New 创建清理组件
func New() *CleanupComponent {
	defaultCleanupComponent = &CleanupComponent{}
	return defaultCleanupComponent
}

// Start 启动清理服务
func (c *CleanupComponent) Start(ctx context.Context) error {
	// 获取数据根目录
	// 从 LogPath 中提取基础路径，或使用环境变量，或使用默认值
	dataPath := "/grdata" // 默认路径

	// 尝试从环境变量获取
	if envPath := os.Getenv("GRDATA_PATH"); envPath != "" {
		dataPath = envPath
	}

	logrus.Infof("Starting local file cleanup service with data path: %s", dataPath)

	// 初始化清理服务
	c.cleanup = NewLocalFileCleanup(dataPath)

	return nil
}

// CloseHandle 关闭清理服务
func (c *CleanupComponent) CloseHandle() {
	if c.cleanup != nil {
		logrus.Info("Closing local file cleanup service...")
		c.cleanup.Close()
	}
}

// Default 获取默认清理组件实例
func Default() *CleanupComponent {
	return defaultCleanupComponent
}
