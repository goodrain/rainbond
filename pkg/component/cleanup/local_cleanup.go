// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

package cleanup

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// LocalFileCleanup 本地文件清理服务
type LocalFileCleanup struct {
	basePath      string
	stopCh        chan struct{}
	done          chan struct{}
	shutdownOnce  sync.Once
	cleanupConfig CleanupConfig
}

// CleanupConfig 清理配置
type CleanupConfig struct {
	// 是否启用清理
	Enabled bool
	// 清理规则
	Rules []CleanupRule
	// 清理时间（小时，0-23）
	CleanupHour int
}

// CleanupRule 清理规则
type CleanupRule struct {
	// 规则名称
	Name string
	// 相对于 basePath 的路径
	Path string
	// 保留天数
	RetentionDays int
	// 文件匹配模式（glob）
	Pattern string
	// 是否递归清理子目录
	Recursive bool
}

// NewLocalFileCleanup 创建本地文件清理服务
func NewLocalFileCleanup(basePath string) *LocalFileCleanup {
	cleanup := &LocalFileCleanup{
		basePath:      basePath,
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		cleanupConfig: getDefaultConfig(),
	}

	// 从环境变量读取配置
	cleanup.loadConfigFromEnv()

	if cleanup.cleanupConfig.Enabled {
		go cleanup.start()
		logrus.Info("Local file cleanup service started")
	} else {
		logrus.Info("Local file cleanup service disabled")
		close(cleanup.done)
	}

	return cleanup
}

// getDefaultConfig 获取默认清理配置
func getDefaultConfig() CleanupConfig {
	return CleanupConfig{
		Enabled:     true,
		CleanupHour: 2, // 凌晨 2点
		Rules: []CleanupRule{
			{
				Name:          "cleanup-temp-chunks",
				Path:          "package_build/temp/chunks",
				RetentionDays: 1,
				Pattern:       "*",
				Recursive:     true,
			},
			{
				Name:          "cleanup-temp-events",
				Path:          "package_build/temp/events",
				RetentionDays: 1,
				Pattern:       "*",
				Recursive:     true,
			},
			{
				Name:          "cleanup-build-slugs",
				Path:          "build/tenant",
				RetentionDays: 7,
				Pattern:       "*.tgz",
				Recursive:     true,
			},
			{
				Name:          "cleanup-app-import",
				Path:          "app/import",
				RetentionDays: 7,
				Pattern:       "*",
				Recursive:     true,
			},
			{
				Name:          "cleanup-app-export",
				Path:          "app",
				RetentionDays: 7,
				Pattern:       "*.zip",
				Recursive:     false,
			},
			{
				Name:          "cleanup-restore",
				Path:          "restore",
				RetentionDays: 1,
				Pattern:       "*",
				Recursive:     true,
			},
		},
	}
}

// loadConfigFromEnv 从环境变量加载配置
func (c *LocalFileCleanup) loadConfigFromEnv() {
	// 是否启用
	if enabled := os.Getenv("LOCAL_CLEANUP_ENABLED"); enabled == "false" {
		c.cleanupConfig.Enabled = false
		return
	}

	// 清理时间
	if hourStr := os.Getenv("LOCAL_CLEANUP_HOUR"); hourStr != "" {
		if hour, err := strconv.Atoi(hourStr); err == nil && hour >= 0 && hour < 24 {
			c.cleanupConfig.CleanupHour = hour
		}
	}

	// 可以通过环境变量调整各个规则的保留天数
	// 格式：CLEANUP_RETENTION_{RULE_NAME}=7
	for i := range c.cleanupConfig.Rules {
		rule := &c.cleanupConfig.Rules[i]
		envKey := "CLEANUP_RETENTION_" + strings.ToUpper(strings.ReplaceAll(rule.Name, "-", "_"))
		if daysStr := os.Getenv(envKey); daysStr != "" {
			if days, err := strconv.Atoi(daysStr); err == nil && days > 0 {
				logrus.Infof("Override retention for %s: %d days", rule.Name, days)
				rule.RetentionDays = days
			}
		}
	}
}

// start 启动清理任务
func (c *LocalFileCleanup) start() {
	defer close(c.done)

	logrus.Infof("Local file cleanup scheduled at %02d:00 daily", c.cleanupConfig.CleanupHour)

	// 立即执行一次清理
	c.runCleanup()

	// 计算到下一次清理时间的延迟
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), c.cleanupConfig.CleanupHour, 0, 0, 0, now.Location())
	if now.Hour() >= c.cleanupConfig.CleanupHour {
		// 如果已经过了今天的清理时间，等到明天
		next = next.AddDate(0, 0, 1)
	}
	firstDelay := next.Sub(now)

	logrus.Infof("First cleanup will run in %v at %s", firstDelay.Round(time.Minute), next.Format("2006-01-02 15:04:05"))

	firstTimer := time.NewTimer(firstDelay)
	defer firstTimer.Stop()

	// 等待首次清理时间
	select {
	case <-firstTimer.C:
		c.runCleanup()
	case <-c.stopCh:
		logrus.Info("Local file cleanup stopped before first run")
		return
	}

	// 之后每 24 小时执行一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.runCleanup()
		case <-c.stopCh:
			logrus.Info("Local file cleanup stopped")
			return
		}
	}
}

// runCleanup 执行清理
func (c *LocalFileCleanup) runCleanup() {
	logrus.Info("====== Starting local file cleanup ======")
	startTime := time.Now()

	totalDeleted := 0
	totalSize := int64(0)
	totalErrors := 0

	for _, rule := range c.cleanupConfig.Rules {
		deleted, size, errors := c.cleanupByRule(rule)
		totalDeleted += deleted
		totalSize += size
		totalErrors += errors
	}

	duration := time.Since(startTime)
	logrus.Infof("====== Local file cleanup completed in %v ======", duration.Round(time.Second))
	logrus.Infof("Summary: deleted %d files (%.2f MB), %d errors",
		totalDeleted,
		float64(totalSize)/(1024*1024),
		totalErrors)
}

// cleanupByRule 按规则清理
func (c *LocalFileCleanup) cleanupByRule(rule CleanupRule) (deleted int, size int64, errors int) {
	dirPath := filepath.Join(c.basePath, rule.Path)

	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logrus.Debugf("[%s] Directory does not exist: %s", rule.Name, dirPath)
		return 0, 0, 0
	}

	logrus.Infof("[%s] Cleaning files older than %d days in %s (pattern: %s)",
		rule.Name, rule.RetentionDays, dirPath, rule.Pattern)

	cutoffTime := time.Now().AddDate(0, 0, -rule.RetentionDays)

	if rule.Recursive {
		// 递归清理
		deleted, size, errors = c.cleanupRecursive(dirPath, rule.Pattern, cutoffTime, rule.Name)
	} else {
		// 只清理当前目录
		deleted, size, errors = c.cleanupDirectory(dirPath, rule.Pattern, cutoffTime, rule.Name)
	}

	if deleted > 0 || errors > 0 {
		logrus.Infof("[%s] Cleaned %d files (%.2f MB), %d errors",
			rule.Name, deleted, float64(size)/(1024*1024), errors)
	} else {
		logrus.Debugf("[%s] No old files to clean", rule.Name)
	}

	return deleted, size, errors
}

// cleanupRecursive 递归清理目录
func (c *LocalFileCleanup) cleanupRecursive(dirPath, pattern string, cutoffTime time.Time, ruleName string) (deleted int, size int64, errors int) {
	err := filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Warnf("[%s] Error accessing %s: %v", ruleName, filePath, err)
			errors++
			return nil // 继续处理其他文件
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 检查文件名是否匹配模式
		matched, err := filepath.Match(pattern, filepath.Base(filePath))
		if err != nil {
			logrus.Warnf("[%s] Invalid pattern %s: %v", ruleName, pattern, err)
			return nil
		}
		if !matched {
			return nil
		}

		// 检查修改时间
		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(filePath); err != nil {
				logrus.Errorf("[%s] Failed to delete %s: %v", ruleName, filePath, err)
				errors++
			} else {
				deleted++
				size += info.Size()
				logrus.Debugf("[%s] Deleted: %s (age: %v, size: %d bytes)",
					ruleName,
					filePath,
					time.Since(info.ModTime()).Round(time.Hour),
					info.Size())
			}
		}

		return nil
	})

	if err != nil {
		logrus.Errorf("[%s] Walk error: %v", ruleName, err)
		errors++
	}

	return deleted, size, errors
}

// cleanupDirectory 清理单个目录（不递归）
func (c *LocalFileCleanup) cleanupDirectory(dirPath, pattern string, cutoffTime time.Time, ruleName string) (deleted int, size int64, errors int) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		logrus.Errorf("[%s] Failed to read directory %s: %v", ruleName, dirPath, err)
		return 0, 0, 1
	}

	for _, entry := range entries {
		// 跳过目录
		if entry.IsDir() {
			continue
		}

		// 检查文件名是否匹配模式
		matched, err := filepath.Match(pattern, entry.Name())
		if err != nil {
			logrus.Warnf("[%s] Invalid pattern %s: %v", ruleName, pattern, err)
			continue
		}
		if !matched {
			continue
		}

		filePath := path.Join(dirPath, entry.Name())

		// 获取文件信息
		fileInfo, err := entry.Info()
		if err != nil {
			logrus.Warnf("[%s] Failed to get file info for %s: %v", ruleName, filePath, err)
			errors++
			continue
		}

		// 检查修改时间
		if fileInfo.ModTime().Before(cutoffTime) {
			if err := os.Remove(filePath); err != nil {
				logrus.Errorf("[%s] Failed to delete %s: %v", ruleName, filePath, err)
				errors++
			} else {
				deleted++
				size += fileInfo.Size()
				logrus.Debugf("[%s] Deleted: %s (age: %v, size: %d bytes)",
					ruleName,
					entry.Name(),
					time.Since(fileInfo.ModTime()).Round(time.Hour),
					fileInfo.Size())
			}
		}
	}

	return deleted, size, errors
}

// Close 关闭清理服务
func (c *LocalFileCleanup) Close() error {
	c.shutdownOnce.Do(func() {
		if c.cleanupConfig.Enabled {
			logrus.Info("Stopping local file cleanup service...")
			close(c.stopCh)
			<-c.done
			logrus.Info("Local file cleanup service stopped")
		}
	})
	return nil
}
