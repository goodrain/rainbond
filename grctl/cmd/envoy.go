// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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
	"context"
	"fmt"
	"strings"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	envoyv2 "github.com/goodrain/rainbond/node/core/envoy/v2"
	"github.com/gosuri/uitable"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

//NewCmdEnvoy envoy cmd
func NewCmdEnvoy() cli.Command {
	c := cli.Command{
		Name:  "envoy",
		Usage: "Envoy management related commands",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "address",
				Usage: "node envoy api address",
				Value: "127.0.0.1:6101",
			},
			cli.StringFlag{
				Name:  "node",
				Usage: "envoy node name",
			},
		},
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "endpoints",
				Usage: "list envoy node endpoints",
				Action: func(c *cli.Context) error {
					return listEnvoyEndpoint(c)
				},
			},
		},
	}
	return c
}

func listEnvoyEndpoint(c *cli.Context) error {
	if c.GlobalString("node") == "" {
		showError("node name can not be empty,please define by --node")
	}
	cli, err := grpc.Dial(c.GlobalString("address"), grpc.WithInsecure())
	if err != nil {
		showError(err.Error())
	}
	endpointDiscover := v2.NewEndpointDiscoveryServiceClient(cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := endpointDiscover.FetchEndpoints(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: c.GlobalString("node"),
			Id:      c.GlobalString("node"),
		},
	})
	if err != nil {
		showError(err.Error())
	}
	if len(res.Resources) == 0 {
		showError("not find endpoints")
	}
	endpoints := envoyv2.ParseLocalityLbEndpointsResource(res.Resources)
	table := uitable.New()
	table.Wrap = true // wrap columns
	for _, end := range endpoints {
		table.AddRow(end.ClusterName, strings.Join(func() []string {
			var re []string
			for _, e := range end.Endpoints {
				for _, a := range e.LbEndpoints {
					if lbe, ok := a.HostIdentifier.(*endpoint.LbEndpoint_Endpoint); ok && lbe != nil {
						if address, ok := lbe.Endpoint.Address.Address.(*core.Address_SocketAddress); ok && address != nil {
							if port, ok := address.SocketAddress.PortSpecifier.(*core.SocketAddress_PortValue); ok && port != nil {
								re = append(re, fmt.Sprintf("%s:%d", address.SocketAddress.Address, port.PortValue))
							}
						}
					}
				}
			}
			return re
		}(), ";"))
	}
	fmt.Println(table)
	return nil
}
