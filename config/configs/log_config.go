package configs

import "github.com/spf13/pflag"

type LogConfig struct {
	LogLevel   string `json:"log_level"`
	LogPath    string `json:"log_path"`
	LoggerFile string `json:"logger_file"`
	LogOutType string `json:"log_out_type"`
}

func AddLogFlags(fs *pflag.FlagSet, lc *LogConfig) {
	fs.StringVar(&lc.LogLevel, "log-level", "info", "the api log level")
	fs.StringVar(&lc.LogPath, "log-path", "/grdata/logs", "Where Docker log files and event log files are stored.")
	fs.StringVar(&lc.LoggerFile, "logger-file", "/logs/request.log", "request log file path")
	fs.StringVar(&lc.LogOutType, "log.type", "stdout", "app log output type. stdout or file ")
}
