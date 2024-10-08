package configs

import "github.com/spf13/pflag"

type ESConfig struct {
	ElasticSearchURL      string `json:"elastic_search_url"`
	ElasticSearchUsername string `json:"elastic_search_username"`
	ElasticSearchPassword string `json:"elastic_search_password"`
	ElasticEnable         bool   `json:"elastic_enable"`
}

func AddESFlags(fs *pflag.FlagSet, esc *ESConfig) {
	fs.StringVar(&esc.ElasticSearchURL, "es-url", "http://47.92.106.114:9200", "es url")
	fs.StringVar(&esc.ElasticSearchUsername, "es-username", "", "es username")
	fs.StringVar(&esc.ElasticSearchPassword, "es-password", "", "es pwd")
	fs.BoolVar(&esc.ElasticEnable, "es-enable", false, "enable es")
}
