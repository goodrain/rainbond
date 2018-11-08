package openresty

import (
	"os"
	"os/exec"
)

const (
	defBinary = "/usr/local/opt/openresty/nginx/sbin/nginx"
	cfgPath   = "/export/servers/nginx/conf/nginx.conf"
)

func nginxExecCommand(args ...string) *exec.Cmd {
	ngx := os.Getenv("NGINX_BINARY")
	if ngx == "" {
		ngx = defBinary
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs,"-c", cfgPath)
	cmdArgs = append(cmdArgs, args...)

	return exec.Command(ngx, cmdArgs...)
}