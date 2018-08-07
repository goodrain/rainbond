// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package main

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"net"
	"os"
	"sort"

	"github.com/Sirupsen/logrus"
	version "github.com/goodrain/rainbond/cmd"
	"github.com/urfave/cli"
)

//Config Config
type Config struct {
	CrtName, KeyName  string
	Address           []string
	IsCa              bool
	CAName, CAKeyName string
	Domains           []string
}

func main() {
	App := cli.NewApp()
	App.Version = version.GetVersion()
	App.Commands = []cli.Command{
		cli.Command{
			Name: "create",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "crt-name",
					Value: "",
					Usage: "creat crt file name",
				},
				cli.StringFlag{
					Name:  "crt-key-name",
					Value: "",
					Usage: "creat crt key file name",
				},
				cli.StringSliceFlag{
					Name:  "address",
					Value: &cli.StringSlice{"127.0.0.1"},
					Usage: "address list",
				},
				cli.StringSliceFlag{
					Name:  "domains",
					Value: &cli.StringSlice{""},
					Usage: "domain list",
				},
				cli.StringFlag{
					Name:  "ca-name",
					Value: "./ca.pem",
					Usage: "creat or read ca file name",
				},
				cli.StringFlag{
					Name:  "ca-key-name",
					Value: "./ca.key.pem",
					Usage: "creat or read ca key file name",
				},
				cli.BoolFlag{
					Name:   "is-ca",
					Hidden: false,
					Usage:  "is create ca",
				},
			},
			Action: create,
		},
	}
	sort.Sort(cli.FlagsByName(App.Flags))
	sort.Sort(cli.CommandsByName(App.Commands))
	App.Run(os.Args)
}
func parseConfig(ctx *cli.Context) Config {
	var c Config
	c.Address = ctx.StringSlice("address")
	c.CAKeyName = ctx.String("ca-key-name")
	c.CAName = ctx.String("ca-name")
	c.CrtName = ctx.String("crt-name")
	c.KeyName = ctx.String("crt-key-name")
	c.Domains = ctx.StringSlice("domains")
	c.IsCa = ctx.Bool("is-ca")
	return c
}
func create(ctx *cli.Context) error {
	c := parseConfig(ctx)
	info := c.CreateCertInformation()
	if c.IsCa {
		err := CreateCRT(nil, nil, info)
		if err != nil {
			logrus.Fatal("Create crt error,Error info:", err)
		}
	} else {
		info.Names = []pkix.AttributeTypeAndValue{{asn1.ObjectIdentifier{2, 1, 3}, "MAC_ADDR"}}
		crt, pri, err := Parse(c.CAName, c.CAKeyName)
		if err != nil {
			logrus.Fatal("Parse crt error,Error info:", err)
		}
		err = CreateCRT(crt, pri, info)
		if err != nil {
			logrus.Fatal("Create crt error,Error info:", err)
		}
	}
	fmt.Println("create success")
	return nil
}

//CreateCertInformation CreateCertInformation
func (c *Config) CreateCertInformation() CertInformation {
	baseinfo := CertInformation{
		Country:            []string{"CN"},
		Organization:       []string{"Goodrain"},
		IsCA:               c.IsCa,
		OrganizationalUnit: []string{"goodrain rainbond"},
		EmailAddress:       []string{"zengqg@goodrain.com"},
		Locality:           []string{"BeiJing"},
		Province:           []string{"BeiJing"},
		CommonName:         "rainbond",
		CrtName:            c.CrtName,
		KeyName:            c.KeyName,
		Domains:            c.Domains,
	}
	if c.IsCa {
		baseinfo.CrtName = c.CAName
		baseinfo.KeyName = c.CAKeyName
	}
	var addres []net.IP
	for _, a := range c.Address {
		addres = append(addres, net.ParseIP(a))
	}
	baseinfo.IPAddresses = addres
	return baseinfo
}
