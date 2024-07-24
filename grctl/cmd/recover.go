package cmd

import (
	"github.com/goodrain/rainbond/grctl/clients"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"bytes"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sigs.k8s.io/yaml"
)

// NewCmdRecover -
func NewCmdRecover() cli.Command {
	c := cli.Command{
		Name:  "recover",
		Usage: "this command is used to restore the rainbond platform\n",
		Subcommands: []cli.Command{
			{
				Name: "region",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:     "region_name",
						Value:    "",
						Usage:    "use region_name",
						FilePath: GetTenantNamePath(),
						Required: true,
					},
					cli.StringFlag{
						Name:     "recover_range",
						Value:    "",
						Usage:    "recover range [all、component、resource]",
						FilePath: GetTenantNamePath(),
						Required: true,
					},
					cli.StringFlag{
						Name:     "console_host",
						Value:    "",
						Usage:    "use console svc host",
						FilePath: GetTenantNamePath(),
					},
				},
				Usage: "recover region resource. example<grctl recover region --region_name rainbond --range all>",
				Action: func(c *cli.Context) error {
					Common(c)
					return recoverRegion(c)
				},
			},
			{
				Name:  "pvc",
				Usage: "recover region pvc. example<grctl recover pvc>",
				Action: func(c *cli.Context) error {
					Common(c)
					return recoverPvc()
				},
			},
		},
	}
	return c
}

func recoverPvc() error {
	cmd := exec.Command("tar", "-zxvf", "pvc_pv_export.tgz")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	pvcdir, _ := os.ReadDir("pvc")
	for _, dirName := range pvcdir {
		pvc, _ := os.ReadDir("pvc/" + dirName.Name())
		for i := range pvc {
			file, _ := os.ReadFile("pvc/" + dirName.Name() + "/" + pvc[i].Name())
			PersistentVolumeClaim := corev1.PersistentVolumeClaim{}
			err = yaml.Unmarshal(file, &PersistentVolumeClaim)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			_, err := clients.K8SClient.CoreV1().PersistentVolumeClaims(PersistentVolumeClaim.Namespace).Create(context.Background(), &PersistentVolumeClaim, metav1.CreateOptions{})
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
		}
	}

	pvdir, _ := os.ReadDir("pv")
	for _, dirName := range pvdir {
		pv, _ := os.ReadDir("pv/" + dirName.Name())
		for i := range pv {
			file, _ := os.ReadFile("pv/" + dirName.Name() + "/" + pv[i].Name())
			PersistentVolume := corev1.PersistentVolume{}
			err = yaml.Unmarshal(file, &PersistentVolume)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			_, err := clients.K8SClient.CoreV1().PersistentVolumes().Create(context.Background(), &PersistentVolume, metav1.CreateOptions{})
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
		}
	}
	return nil
}

func recoverRegion(ctx *cli.Context) error {
	regionName := ctx.String("region_name")
	fmt.Println(regionName)
	recoverRange := ctx.String("recover_range")
	fmt.Println(recoverRange)
	consoleHost := ctx.String("console_host")
	fmt.Println(consoleHost)
	recoverUrl := fmt.Sprintf("%v/console/regions_recover", consoleHost)

	requestBody, err := json.Marshal(map[string]string{
		"region_name":   regionName,
		"recover_range": recoverRange,
	})
	if err != nil {
		showError(fmt.Sprintf("failed to marshal request body: %v", err))
	}
	resp, err := http.Post(recoverUrl, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		showError(fmt.Sprintf("failed to make request: %v", err))
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		showError(fmt.Sprintf("failed to read response body: %v", err))
	}
	fmt.Printf("Response Body: %s\n", body)
	return nil
}

type Bean struct {
	ResourceCount  int `json:"resource_count"`
	ComponentCount int `json:"component_count"`
}
