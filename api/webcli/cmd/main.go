package main

import (
	"fmt"
	"github.com/goodrain/rainbond-operator/util/constants"
	app2 "github.com/goodrain/rainbond/api/webcli/app"
	utils "github.com/goodrain/rainbond/util"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/sirupsen/logrus"
	restclient "k8s.io/client-go/rest"
)

func main() {
	option := app2.DefaultOptions
	option.K8SConfPath = "/root/.kube/config"
	config, err := k8sutil.NewRestConfig(option.K8SConfPath)
	if err != nil {
		logrus.Error(err)
	}
	config.UserAgent = "rainbond/webcli"
	app2.SetConfigDefaults(config)
	restClient, err := restclient.RESTClientFor(config)
	if err != nil {
		logrus.Error(err)
	}
	namespace := utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)
	commands := []string{"sh"}
	req := restClient.Post().
		Resource("pods").
		Name("rainbond-operator-0").
		Namespace(namespace).
		SubResource("exec").
		Param("container", "operator").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("tty", "true")
	for _, c := range commands {
		req.Param("command", c)
	}

	slave, err := app2.NewExecContext(req, config)
	if err != nil {
		logrus.Error(err)
		return
	}
	slave.ResizeTerminal(100, 60)
	defer slave.Close()
	for {
		buffer := make([]byte, 1024)
		n, err := slave.Read(buffer)
		if err != nil {
			logrus.Error(err)
			return
		}
		fmt.Print(string(buffer[:n]))
	}
}
