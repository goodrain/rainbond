// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

package cleanup

import (
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	once            sync.Once
	cleanupInstance *LocalFileCleanup
)

// Init 初始化清理服务
// basePath: 数据根目录，通常是 /grdata
func Init(basePath string) {
	once.Do(func() {
		cleanupInstance = NewLocalFileCleanup(basePath)
		logrus.Infof("Local file cleanup manager initialized with base path: %s", basePath)
	})
}

// GetCleanup 获取清理服务实例
func GetCleanup() *LocalFileCleanup {
	if cleanupInstance == nil {
		logrus.Warn("Local file cleanup not initialized, call Init() first")
	}
	return cleanupInstance
}

// Close 关闭清理服务
func Close() error {
	if cleanupInstance != nil {
		return cleanupInstance.Close()
	}
	return nil
}
