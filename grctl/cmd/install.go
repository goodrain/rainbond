package cmd

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/monitor/utils"
	"io/ioutil"
	"os"
	"path"

	rainbondv1alpha1 "github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond/api/region"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/cmd/grctl/option"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var pemDirPath = ".rbd/ssl"
var clientPemPath string
var clientKeyPemPath string
var clientCAPemPath string

func init() {
	homePath, _ := sources.Home()
	pemDirPath = path.Join(homePath, pemDirPath)
	clientPemPath = path.Join(pemDirPath, "client.pem")
	clientKeyPemPath = path.Join(pemDirPath, "client.key.pem")
	clientCAPemPath = path.Join(pemDirPath, "ca.pem")
}

// NewCmdInstall -
func NewCmdInstall() cli.Command {
	c := cli.Command{
		Name:   "install",
		Hidden: true,
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:   "gateway-ips",
				Usage:  "all gateway ip of this cluster, use it to access the region api",
				EnvVar: "GatewayIP",
			},
			cli.StringFlag{
				Name:   "namespace,ns",
				Usage:  "rainbond namespace",
				EnvVar: "RBDNamespace",
				Value:  utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace),
			},
		},
		Usage: "grctl install",
		Action: func(c *cli.Context) error {
			fmt.Println("Start install, please waiting!")
			CommonWithoutRegion(c)
			namespace := c.String("namespace")
			apiClientSecrit, err := clients.K8SClient.CoreV1().Secrets(namespace).Get(context.Background(), "rbd-api-client-cert", metav1.GetOptions{})
			if err != nil {
				showError(fmt.Sprintf("get region api tls secret failure %s", err.Error()))
			}
			regionAPIIP := c.StringSlice("gateway-ip")
			if len(regionAPIIP) == 0 {
				var cluster rainbondv1alpha1.RainbondCluster
				err := clients.RainbondKubeClient.Get(context.Background(),
					types.NamespacedName{Namespace: namespace, Name: "rainbondcluster"}, &cluster)
				if err != nil {
					showError(fmt.Sprintf("get rainbond cluster config failure %s", err.Error()))
				}
				gatewayIP := cluster.GatewayIngressIPs()
				if len(gatewayIP) == 0 {
					showError("gateway ip not found")
				}
				regionAPIIP = gatewayIP
			}
			if err := writeCertFile(apiClientSecrit); err != nil {
				showError(fmt.Sprintf("write region api cert file failure %s", err.Error()))
			}
			if err := writeConfig(regionAPIIP); err != nil {
				showError(fmt.Sprintf("write grctl config file failure %s", err.Error()))
			}
			fmt.Println("Install success!")
			return nil
		},
	}
	return c
}

func writeCertFile(apiClientSecrit *v1.Secret) error {
	if _, err := os.Stat(pemDirPath); err != nil {
		os.MkdirAll(pemDirPath, os.ModeDir)
	}
	if err := ioutil.WriteFile(clientPemPath, apiClientSecrit.Data["client.pem"], 0411); err != nil && !os.IsExist(err) {
		return err
	}
	if err := ioutil.WriteFile(clientKeyPemPath, apiClientSecrit.Data["client.key.pem"], 0411); err != nil && !os.IsExist(err) {
		return err
	}
	if err := ioutil.WriteFile(clientCAPemPath, apiClientSecrit.Data["ca.pem"], 0411); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func writeConfig(ips []string) error {
	var endpoints []string
	for _, ip := range ips {
		endpoints = append(endpoints, fmt.Sprintf("https://%s:8443", ip))
	}
	var config = option.Config{
		RegionAPI: region.APIConf{
			Endpoints: endpoints,
			Cacert:    clientCAPemPath,
			Cert:      clientPemPath,
			CertKey:   clientKeyPemPath,
		},
	}
	home, _ := sources.Home()
	configFilePath := path.Join(home, ".rbd", "grctl.yaml")
	os.MkdirAll(path.Dir(configFilePath), os.ModeDir)
	os.Remove(configFilePath)
	configFile, err := os.OpenFile(configFilePath, os.O_CREATE|os.O_RDWR, 0411)
	if err != nil {
		return err
	}
	defer configFile.Close()
	body, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}
	_, err = configFile.Write(body)
	if err != nil {
		return err
	}
	return nil
}
