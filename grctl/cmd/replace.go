// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RegionInfo -
type RegionInfo struct {
	RegionName string `json:"region_name"`
	SslCaCert  string `json:"ssl_ca_cert"`
	KeyFile    string `json:"key_file"`
	CertFile   string `json:"cert_file"`
	URL        string `json:"url"`
	WsURL      string `json:"ws_url"`
	HTTPDomain string `json:"http_domain"`
	TCPDomain  string `json:"tcp_domain"`
}

// Response -
type Response struct {
	Code    int32  `json:"code"`
	Msg     string `json:"msg"`
	MsgShow string `json:"msg_show"`
}

const (
	successCode = 200
	namespace   = "rbd-system"
)

//NewCmdReplace replace cmd
func NewCmdReplace() cli.Command {
	var (
		ip     string
		domain string
		token  string
		name   string
		suffix string
	)
	c := cli.Command{
		Name:  "replace",
		Usage: "replace rainbond cluster info",
		Subcommands: []cli.Command{
			{
				Name:  "ip",
				Usage: "The new IP address takes effect",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:        "ip",
						Usage:       "new ip address",
						Destination: &ip,
					},
					cli.StringFlag{
						Name:        "domain,d",
						Usage:       "console domain You must start with HTTP or HTTPS",
						Destination: &domain,
					},
					cli.StringFlag{
						Name:        "token,t",
						Usage:       "console token",
						Destination: &token,
					},
					cli.StringFlag{
						Name:        "name,n",
						Usage:       "region name",
						Destination: &name,
					},
					cli.StringFlag{
						Name:        "suffix,s",
						Usage:       "region name",
						Destination: &suffix,
						Value:       "false",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					if ip == "" {
						logrus.Errorf("need args")
						return nil
					}
					fmt.Println("new ip:", ip)
					fmt.Println("console domain:", domain)
					fmt.Println("console token:", token)
					fmt.Println("cluster name:", name)

					var rc v1alpha1.RainbondCluster
					// get rainbondcluster info
					if err := clients.RainbondKubeClient.Get(context.Background(),
						types.NamespacedName{Namespace: "rbd-system", Name: "rainbondcluster"}, &rc); err != nil {
						return errors.Wrap(err, "get rainbondcluster info")
					}
					//update ip
					rc.Spec.GatewayIngressIPs = []string{ip}
					if err := clients.RainbondKubeClient.Update(context.Background(), &rc); err != nil {
						return errors.Wrap(err, "update rainbond cluster")
					}
					// delete rbd-api-client-cert
					if err := clients.K8SClient.CoreV1().Secrets("rbd-system").Delete(context.Background(),
						"rbd-api-client-cert", metav1.DeleteOptions{}); err != nil {
						return errors.Wrap(err, "delete rbd-api-client-cert")
					}
					// delete rbd-api-server-cert
					if err := clients.K8SClient.CoreV1().Secrets("rbd-system").Delete(context.Background(),
						"rbd-api-server-cert", metav1.DeleteOptions{}); err != nil {
						return errors.Wrap(err, "delete rbd-api-server-cert error")
					}
					// get pod list
					pods, err := clients.K8SClient.CoreV1().Pods("rbd-system").List(context.Background(),
						metav1.ListOptions{})
					if err != nil {
						return errors.Wrap(err, "get rainbond pod list error")
					}
					var (
						operatorPodName string
						apiPodName      string
					)
					for _, pod := range pods.Items {
						if find := strings.Contains(pod.Name, "operator"); find {
							operatorPodName = pod.Name
						}
						if find := strings.Contains(pod.Name, "rbd-api"); find {
							apiPodName = pod.Name
						}
						// TODO Processing of multiple API Pods
					}
					fmt.Println("Please wait while the cluster configuration is updated............")
					// delete pod rainbond-operator
					if err := clients.K8SClient.CoreV1().Pods(namespace).Delete(context.Background(),
						operatorPodName, metav1.DeleteOptions{}); err != nil {
						return errors.Wrap(err, "delete rainbond-operator error")
					}
					time.Sleep(time.Second * 3)

					var operatorNewName string
					if pods, err = clients.K8SClient.CoreV1().Pods(namespace).List(context.Background(),
						metav1.ListOptions{}); err != nil {
						return errors.Wrap(err, "get rainbond pod list error")
					}
					for _, pod := range pods.Items {
						if find := strings.Contains(pod.Name, "rainbond-operator"); find {
							operatorNewName = pod.Name
							break
						}
					}
					var newOperatorPod *corev1.Pod
					// wait operator running
					for {
						if newOperatorPod, err = clients.K8SClient.CoreV1().Pods(namespace).Get(context.Background(),
							operatorNewName, metav1.GetOptions{}); err != nil {
							return errors.Wrap(err, "get new operator pod error")
						}
						if newOperatorPod != nil && newOperatorPod.Status.Phase == "Running" {
							break
						}
					}

					// delete pod rainbond-api
					if err := clients.K8SClient.CoreV1().Pods(namespace).Delete(context.Background(),
						apiPodName, metav1.DeleteOptions{}); err != nil {
						return errors.Wrap(err, "delete rainbond-api error")
					}

					// get new secret
					var secret *corev1.Secret
					for {
						if secret, err = clients.K8SClient.CoreV1().Secrets(namespace).Get(context.Background(),
							"rbd-api-client-cert", metav1.GetOptions{}); err != nil {
							if strings.Contains(err.Error(), "not found") {
								continue
							}
						}
						if secret != nil {
							break
						}
						time.Sleep(time.Second * 1)
					}
					// get configmap
					var configMap *corev1.ConfigMap
					if configMap, err = clients.K8SClient.CoreV1().ConfigMaps(namespace).Get(context.Background(),
						"region-config", metav1.GetOptions{}); err != nil {
						return errors.Wrap(err, "get configMap error")
					}
					var regionInfo RegionInfo
					regionInfo.RegionName = name
					regionInfo.CertFile = string(secret.Data["client.pem"])
					regionInfo.KeyFile = string(secret.Data["client.key.pem"])
					regionInfo.SslCaCert = string(secret.Data["ca.pem"])
					regionInfo.URL = configMap.Data["apiAddress"]
					regionInfo.HTTPDomain = configMap.Data["defaultDomainSuffix"]
					regionInfo.TCPDomain = configMap.Data["defaultTCPHost"]
					regionInfo.WsURL = configMap.Data["websocketAddress"]
					// A new domain name suffix is generated
					if suffix == "true" {
						suffix, err := genSuffixHTTPHost(ip)
						if err != nil {
							fmt.Println("get suffix error")
							return err
						}
						regionInfo.HTTPDomain = suffix
					}
					// TODO old domain handle if suffix false
					// send ip config to console
					if err := SendtoConsole(domain, token, &regionInfo); err != nil {
						fmt.Println("SendtoConsole error:", err)
					}
					return nil
				},
			},
		},
	}
	return c
}

// SendtoConsole -
func SendtoConsole(domain, token string, regionInfo *RegionInfo) (err error) {
	reqParam, err := json.Marshal(&regionInfo)
	if err != nil {
		logrus.Error("Marshal RequestParam fail", err)
		return err
	}
	client := &http.Client{}
	consoleDomain := fmt.Sprintf("%s%s", strings.TrimSuffix(domain, "/"), "/openapi/v1/grctl/ip")
	request, err := http.NewRequest("POST", consoleDomain,
		strings.NewReader(string(reqParam)))
	if err != nil {
		logrus.Error("request reader fail", err)
		return err
	}
	request.Header.Add("Authorization", token)
	request.Header.Add("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		logrus.Error("Request console openapi interface failed", err)
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logrus.Error("ReadAll error", err)
		return err
	}
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		logrus.Error("response json unmarshal error", err)
		return err
	}
	fmt.Printf("Rainbond Cluster config update %v", resp.Msg)
	return nil
}

// genSuffixHTTPHost -
func genSuffixHTTPHost(ip string) (domain string, err error) {
	id, auth, err := getOrCreateUUIDAndAuth()
	if err != nil {
		return "", err
	}
	domain, err = GenerateDomain(ip, id, auth)
	if err != nil {
		return "", err
	}
	return domain, nil
}

// getOrCreateUUIDAndAuth -
func getOrCreateUUIDAndAuth() (id, auth string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cm := &corev1.ConfigMap{}
	cm = GenerateSuffixConfigMap("rbd-suffix-host", namespace)
	if _, err = clients.K8SClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return "", "", err
	}
	return cm.Data["uuid"], cm.Data["auth"], nil
}

// GenerateSuffixConfigMap -
func GenerateSuffixConfigMap(name, namespace string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"uuid": string(uuid.NewUUID()),
			"auth": string(uuid.NewUUID()),
		},
	}
	return cm
}

// GenerateDomain generate suffix domain
func GenerateDomain(iip, id, secretKey string) (string, error) {
	body := make(url.Values)
	body["ip"] = []string{iip}
	body["uuid"] = []string{id}
	body["type"] = []string{"False"}
	body["auth"] = []string{secretKey}

	resp, err := http.PostForm("http://domain.grapps.cn/domain/new", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
