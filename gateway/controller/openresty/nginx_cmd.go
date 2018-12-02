package openresty

import (
	"os"
	"os/exec"
	"path"

	"github.com/goodrain/rainbond/gateway/controller/openresty/template"
)

var (
	nginxBinary      = "nginx"
	defaultNginxConf = "/run/nginx/conf/nginx.conf"
)

func init() {
	nginxBinary = path.Join(os.Getenv("OPENRESTY_HOME"), "/nginx/sbin/nginx")
	ngx := os.Getenv("NGINX_BINARY")
	if ngx != "" {
		nginxBinary = ngx
	}
	customConfig := os.Getenv("NGINX_CUSTOM_CONFIG")
	if customConfig != "" {
		template.CustomConfigPath = customConfig
	}
}
func nginxExecCommand(args ...string) *exec.Cmd {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "-c", defaultNginxConf)
	cmdArgs = append(cmdArgs, args...)
	return exec.Command(nginxBinary, cmdArgs...)
}
