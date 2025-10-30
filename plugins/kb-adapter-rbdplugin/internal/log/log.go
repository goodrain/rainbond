// Package log 提供日志记录相关功能、配置
package log

import (
	"os"
	"sync"
	"time"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/config"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

type Field = zap.Field

// InitLogger 初始化全局 logger 实例
// 自动检测环境：默认生产模式，只有在 ENV=dev 时才使用开发模式
// 同时输出到控制台和 logs/bm.log 文件中
func InitLogger() {
	once.Do(func() {
		development := config.InDevelopment()

		// 创建 logs 目录
		if err := os.MkdirAll("logs", 0755); err != nil {
			createConsoleOnlyLogger(development)
			return
		}

		consoleEncoder := createConsoleEncoder(development)

		fileEncoder := createFileEncoder(development)

		consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), getLogLevel(development))

		fileCore, err := createFileCore(fileEncoder, development)
		if err != nil {
			createConsoleOnlyLogger(development)
			return
		}

		core := zapcore.NewTee(consoleCore, fileCore)

		logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

		globalLogger = logger

		// 为 controller-runtime 设置 logger
		ctrl.SetLogger(zapr.NewLogger(logger))
	})
}

// getLogLevel 根据环境获取日志级别
func getLogLevel(development bool) zapcore.Level {
	if development {
		return zapcore.DebugLevel
	}
	return zapcore.InfoLevel
}

// createConsoleEncoder 创建控制台编码器
func createConsoleEncoder(development bool) zapcore.Encoder {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	if development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
	}
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// createFileEncoder 创建文件编码器
func createFileEncoder(development bool) zapcore.Encoder {
	if development {
		// 开发环境，去除颜色
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		return zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		// 生产环境：使用 JSON 格式
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		return zapcore.NewJSONEncoder(encoderConfig)
	}
}

// createFileCore 创建文件输出核心
func createFileCore(encoder zapcore.Encoder, development bool) (zapcore.Core, error) {
	file, err := os.OpenFile("logs/bm.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return zapcore.NewCore(encoder, zapcore.AddSync(file), getLogLevel(development)), nil
}

// createConsoleOnlyLogger 创建仅控制台输出的 logger
func createConsoleOnlyLogger(development bool) {
	encoder := createConsoleEncoder(development)
	core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), getLogLevel(development))
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	globalLogger = logger
	ctrl.SetLogger(zapr.NewLogger(logger))
}

// getLogger 获取全局 logger 实例
func getLogger() *zap.Logger {
	if globalLogger == nil {
		InitLogger()
	}
	return globalLogger
}

// 日志记录

// getLoggerWithCallerSkip 获取带有调用位置跳过的 logger，默认跳过 1 层。
//
// 确保正确记录 log 位置
func getLoggerWithCallerSkip() *zap.Logger {
	return getLoggerWithCustomCallerSkip(1)
}

// getLoggerWithCustomCallerSkip 返回一个带有自定义调用位置跳过层数的 logger,
// skip 参数指定需要跳过的调用栈层数
func getLoggerWithCustomCallerSkip(skip int) *zap.Logger {
	return getLogger().WithOptions(zap.AddCallerSkip(skip))
}

func Info(msg string, fields ...Field) {
	getLoggerWithCallerSkip().Info(msg, fields...)
}

func Error(msg string, fields ...Field) {
	getLoggerWithCallerSkip().Error(msg, fields...)
}

func Debug(msg string, fields ...Field) {
	getLoggerWithCallerSkip().Debug(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	getLoggerWithCallerSkip().Warn(msg, fields...)
}

func Fatal(msg string, fields ...Field) {
	getLoggerWithCallerSkip().Fatal(msg, fields...)
}

func With(fields ...Field) *zap.Logger {
	return getLogger().With(fields...)
}

func Sync() error {
	return getLogger().Sync()
}

// 常用字段函数，避免额外导入 zap

func String(key, val string) Field {
	return zap.String(key, val)
}

func Int(key string, val int) Field {
	return zap.Int(key, val)
}

func Int32(key string, val int32) Field {
	return zap.Int32(key, val)
}

func Int64(key string, val int64) Field {
	return zap.Int64(key, val)
}

func Float64(key string, val float64) Field {
	return zap.Float64(key, val)
}

func Bool(key string, val bool) Field {
	return zap.Bool(key, val)
}

// Err 创建错误 Field
//
// 用于兼容命名冲突
func Err(err error) Field {
	return zap.Error(err)
}

func Duration(key string, val time.Duration) Field {
	return zap.Duration(key, val)
}

func Time(key string, val time.Time) Field {
	return zap.Time(key, val)
}

func Any(key string, val any) Field {
	return zap.Any(key, val)
}
