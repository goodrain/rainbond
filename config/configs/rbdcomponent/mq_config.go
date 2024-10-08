package rbdcomponent

import "github.com/spf13/pflag"

type MQConfig struct {
	KeyPrefix string
	RunMode   string //http grpc
	HostName  string
	APIPort   int
}

func AddMQFlags(fs *pflag.FlagSet, mqc *MQConfig) {
	fs.StringVar(&mqc.KeyPrefix, "key-prefix", "/mq", "key prefix ")
	fs.StringVar(&mqc.RunMode, "mode", "grpc", "the api server run mode grpc or http")
	fs.StringVar(&mqc.HostName, "hostName", "", "Current node host name")
	fs.IntVar(&mqc.APIPort, "api-port", 6300, "the api server listen port")
}
