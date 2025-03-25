package configs

import (
	"fmt"
	"github.com/goodrain/rainbond/config/configs/rbdcomponent"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"os"
	"runtime"
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

	return c
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
