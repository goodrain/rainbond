package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/webcli/app"
	restclient "k8s.io/client-go/rest"
)

func main() {
	option := app.DefaultOptions
	option.K8SConfPath = "/tmp/config"
	ap, err := app.New(&option)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Info(ap.GetDefaultContainerName("rbd-system", "rainbond-operator-0"))
	config, err := k8sutil.NewRestConfig(option.K8SConfPath)
	if err != nil {
		logrus.Error(err)
	}
	config.UserAgent = "rainbond/webcli"
	app.SetConfigDefaults(config)
	restClient, err := restclient.RESTClientFor(config)
	if err != nil {
		logrus.Error(err)
	}
	commands := []string{"sh"}
	req := restClient.Post().
		Resource("pods").
		Name("rainbond-operator-0").
		Namespace("rbd-system").
		SubResource("exec").
		Param("container", "operator").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("tty", "true")
	for _, c := range commands {
		req.Param("command", c)
	}
	out := &app.Out{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	t := out.SetTTY()
	fn := func() error {
		exec := app.NewExecContextByStd(&app.ClientContext{}, out.Stdin, out.Stdout, out.Stderr, req, config)
		if err := exec.Run(); err != nil {
			logrus.Error(err)
			return err
		}
		return nil
	}
	if err := t.Safe(fn); err != nil {
		logrus.Error(err)
	}
}
