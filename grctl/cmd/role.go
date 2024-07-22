package cmd

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewCmdRole() cli.Command {
	c := cli.Command{
		Name:  "role",
		Usage: "this command is switch cluster role\n",
		Subcommands: []cli.Command{
			{
				Name:  "master",
				Usage: "set worker env CLUSTER_STATUS=master. example<grctl role master>",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:     "ns",
						Value:    "rbd-system",
						Usage:    "指定worker所在的命名空间",
						Required: false,
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					return switchRole("master", c)
				},
			},
			{
				Name:  "backup",
				Usage: "set worker env CLUSTER_STATUS=backup. example<grctl role backup>",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:     "ns",
						Value:    "rbd-system",
						Usage:    "指定worker所在的命名空间",
						Required: false,
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					return switchRole("backup", c)
				},
			},
		},
	}
	return c
}

func switchRole(role string, ctx *cli.Context) error {
	ns := ctx.String("ns")
	var cpt v1alpha1.RbdComponent
	err := clients.RainbondKubeClient.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: "rbd-worker"}, &cpt)
	if err != nil {
		fmt.Println(err)
		return err
	}
	for i := range cpt.Spec.Env {
		if cpt.Spec.Env[i].Name == "CLUSTER_STATUS" {
			cpt.Spec.Env[i].Value = role
			err = clients.RainbondKubeClient.Update(context.Background(), &cpt)
			if err != nil {
				fmt.Println(err)
				return err
			}
			return nil
		}
	}
	cpt.Spec.Env = append(cpt.Spec.Env, corev1.EnvVar{Name: "CLUSTER_STATUS", Value: role})

	err = clients.RainbondKubeClient.Update(context.Background(), &cpt)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
