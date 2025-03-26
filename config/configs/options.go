package configs

import (
	"fmt"
	"github.com/goodrain/rainbond/config/configs/rbdcomponent"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"net"
	"os"
	"runtime"
	"strings"
)

func (c *Config) SetAppName(name string) *Config {
	c.AppName = name
	return c
}

func (c *Config) SetAPIFlags() *Config {
	rbdcomponent.AddAPIFlags(c.fs, c.APIConfig)
	rbdcomponent.AddEventLogFlags(c.fs, c.EventLogConfig)
	return c
}

func (c *Config) SetMQFlags() *Config {
	rbdcomponent.AddMQFlags(c.fs, c.MQConfig)
	return c
}

func (c *Config) SetWorkerFlags() *Config {
	rbdcomponent.AddWorkerFlags(c.fs, c.WorkerConfig)
	return c
}

func (c *Config) SetChaosFlags() *Config {
	rbdcomponent.AddChaosFlags(c.fs, c.ChaosConfig)
	return c
}

func (c *Config) SetPublicFlags() *Config {
	AddDBFlags(c.fs, c.DBConfig)
	AddESFlags(c.fs, c.ESConfig)
	AddLogFlags(c.fs, c.LogConfig)
	AddStorageFlags(c.fs, c.StorageConfig)
	AddFilePersistenceFlags(c.fs, c.FilePersistenceConfig)
	AddWebSocketFlags(c.fs, c.WebSocketConfig)
	AddK8SFlags(c.fs, c.K8SConfig)
	AddPrometheusFlags(c.fs, c.PrometheusConfig)
	AddServerFlags(c.fs, c.ServerConfig)
	rbdcomponent.AddMQFlags(c.fs, c.MQConfig)
	AddPublicFlags(c.fs, c.PublicConfig)
	return c
}

func (c *Config) Parse() *Config {
	pflag.Parse()
	return c
}

// SetLog 设置log
func (c *Config) SetLog() *Config {
	level, err := logrus.ParseLevel(c.LogConfig.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return c
	}
	logrus.SetLevel(level)

	// 获取本机 IP 地址
	ip := "unknown"
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ip = ipnet.IP.String()
					break
				}
			}
		}
	}

	// 设置自定义日志格式
	logrus.SetFormatter(&customFormatter{
		textFormatter: logrus.TextFormatter{
			TimestampFormat: "2006/01/02 15:04:05",
			FullTimestamp:   true,
			DisableColors:   false,
			ForceColors:     true,
			DisableQuote:    true,
		},
		ip:      ip,
		appName: c.AppName,
	})

	return c
}

// customFormatter 自定义日志格式化器
type customFormatter struct {
	textFormatter logrus.TextFormatter
	ip            string
	appName       string
}

// Format 实现 Formatter 接口
func (f *customFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// 获取时间戳
	timestamp := entry.Time.Format(f.textFormatter.TimestampFormat)

	// 获取日志级别
	level := strings.ToUpper(entry.Level.String())

	// 构建日志前缀
	prefix := fmt.Sprintf("%s[%s] ", level, timestamp)

	// 构建元数据部分
	metadata := fmt.Sprintf("ip=%s module=%s ", f.ip, f.appName)

	// 构建完整日志
	log := fmt.Sprintf("%s%s%s\n", prefix, metadata, entry.Message)

	return []byte(log), nil
}

// logHook 自定义日志钩子
type logHook struct {
	ip      string
	appName string
}

// Levels 实现 Hook 接口
func (h *logHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire 实现 Hook 接口
func (h *logHook) Fire(entry *logrus.Entry) error {
	if entry.Data == nil {
		entry.Data = make(logrus.Fields)
	}
	entry.Data["ip"] = h.ip
	entry.Data["module"] = h.appName
	return nil
}

// CheckEnv 检测环境变量
func (c *Config) CheckEnv() error {
	if err := os.Setenv("GRDATA_PVC_NAME", c.PublicConfig.GrdataPVCName); err != nil {
		return fmt.Errorf("set env 'GRDATA_PVC_NAME': %v", err)
	}
	return nil
}

// CheckConfig 测配置
func (c *Config) CheckConfig() error {
	if c.ChaosConfig.Topic != client.BuilderTopic && c.ChaosConfig.Topic != client.WindowsBuilderTopic {
		return fmt.Errorf("Topic is only suppory `%s` and `%s`", client.BuilderTopic, client.WindowsBuilderTopic)
	}
	if runtime.GOOS == "windows" {
		c.ChaosConfig.Topic = "windows_builder"
	}
	return nil
}
