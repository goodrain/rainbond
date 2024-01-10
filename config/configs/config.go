package configs

import "github.com/goodrain/rainbond/cmd/api/option"

// Env -
type Env string

// Config -
type Config struct {
	AppName   string
	Version   string
	Env       Env
	Debug     bool
	APIConfig option.Config
}

var defaultConfig *Config

// Default -
func Default() *Config {
	return defaultConfig
}

// SetDefault -
func SetDefault(cfg *Config) {
	defaultConfig = cfg
}
