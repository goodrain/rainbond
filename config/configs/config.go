package configs

import "github.com/goodrain/rainbond/cmd/api/option"

type Env string

type Config struct {
	AppName   string
	Version   string
	Env       Env
	Debug     bool
	APIConfig option.Config
}
