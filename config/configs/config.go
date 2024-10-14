package configs

import (
	"github.com/goodrain/rainbond/config/configs/rbdcomponent"
	"github.com/spf13/pflag"
)

// Env -
type Env string

func init() {
	defaultConfig = &Config{
		fs:               pflag.CommandLine,
		APIConfig:        &rbdcomponent.APIConfig{},
		EventLogConfig:   &rbdcomponent.EventLogConfig{},
		StorageConfig:    &StorageConfig{},
		DBConfig:         &DBConfig{},
		ESConfig:         &ESConfig{},
		LogConfig:        &LogConfig{},
		WebSocketConfig:  &WebSocketConfig{},
		MQConfig:         &rbdcomponent.MQConfig{},
		K8SConfig:        &K8SConfig{},
		PrometheusConfig: &PrometheusConfig{},
		ServerConfig:     &ServerConfig{},
		WorkerConfig:     &rbdcomponent.WorkerConfig{},
		PublicConfig:     &PublicConfig{},
		ChaosConfig:      &rbdcomponent.ChaosConfig{},
	}
}

// Config -
type Config struct {
	AppName          string
	Version          string
	Env              Env
	Debug            bool
	APIConfig        *rbdcomponent.APIConfig
	EventLogConfig   *rbdcomponent.EventLogConfig
	StorageConfig    *StorageConfig
	DBConfig         *DBConfig
	ESConfig         *ESConfig
	LogConfig        *LogConfig
	WebSocketConfig  *WebSocketConfig
	MQConfig         *rbdcomponent.MQConfig
	K8SConfig        *K8SConfig
	PrometheusConfig *PrometheusConfig
	ServerConfig     *ServerConfig
	WorkerConfig     *rbdcomponent.WorkerConfig
	PublicConfig     *PublicConfig
	ChaosConfig      *rbdcomponent.ChaosConfig
	fs               *pflag.FlagSet
}

var defaultConfig *Config

// Default -
func Default() *Config {
	return defaultConfig
}
