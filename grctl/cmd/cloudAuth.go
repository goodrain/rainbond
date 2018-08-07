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

// //CloudAuth CloudAuth
// type CloudAuth struct {
// 	c *cli.Context
// }

// type respBean struct {
// 	Bean *dbmodel.RegionUserInfo `json:"bean"`
// }

// //NewCmdCloudAuth 云市授权相关操作
// func NewCmdCloudAuth() cli.Command {
// 	c := cli.Command{
// 		Name:  "auth",
// 		Usage: "自定义云市授权相关操作。grctl auth [create/update/get] -e [EID] -t [VALIDITY DATE] ",
// 		Subcommands: []cli.Command{
// 			{
// 				Name:  "create",
// 				Usage: "创建授权信息。 grctl auth create -e EID -t VALIDITY_DATE",
// 				Action: func(c *cli.Context) error {
// 					return authAction(c, "create")
// 				},
// 				Flags: []cli.Flag{
// 					cli.StringFlag{
// 						Name:  "eid, e",
// 						Usage: "企业id，-e EID",
// 					},
// 					cli.StringFlag{
// 						Name:  "ttl, t",
// 						Usage: "有效期，-t VALIDITY_DATE",
// 					},
// 				},
// 			},
// 			{
// 				Name:  "update",
// 				Usage: "更新授权有效期。 grctl auth update -e EID -t VALIDITY_DATE",
// 				Action: func(c *cli.Context) error {
// 					return authAction(c, "update")
// 				},
// 				Flags: []cli.Flag{
// 					cli.StringFlag{
// 						Name:  "eid, e",
// 						Usage: "企业id，-e EID",
// 					},
// 					cli.StringFlag{
// 						Name:  "ttl, t",
// 						Usage: "有效期，-t VALIDITY_DATE",
// 					},
// 				},
// 			},
// 			{
// 				Name:  "get",
// 				Usage: "获取授权信息。 grctl auth get -e EID",
// 				Action: func(c *cli.Context) error {
// 					return authAction(c, "get")
// 				},
// 				Flags: []cli.Flag{
// 					cli.StringFlag{
// 						Name:  "eid, e",
// 						Usage: "企业id，-e EID",
// 					},
// 				},
// 			},
// 		},
// 	}
// 	return c
// }

// func authAction(c *cli.Context, action string) error {
// 	Common(c)
// 	ca := CloudAuth{
// 		c: c,
// 	}
// 	switch action {
// 	case "create", "-c":
// 		return ca.createToken()
// 	case "update", "-u":
// 		return ca.updateTokenTime()
// 	case "get", "-g":
// 		return ca.getToken()
// 	}
// 	return fmt.Errorf("Commands wrong, first args must in [create/update/get] or their simplified format")
// }

// func (ca *CloudAuth) createToken() error {
// 	eid, err := checkoutKV(ca.c, "eid")
// 	if err != nil {
// 		return err
// 	}
// 	ttl, err := checkoutKV(ca.c, "ttl")
// 	if err != nil {
// 		return err
// 	}
// 	var gt api_model.GetUserToken
// 	tt, _ := strconv.Atoi(ttl)
// 	gt.Body.EID = eid
// 	gt.Body.ValidityPeriod = tt
// 	resp, err := clients.RegionClient.Tenants().DefineCloudAuth(&gt).PostToken()
// 	if err != nil {
// 		fmt.Printf("create auth %s failure\n", gt.Body.EID)
// 		return err
// 	}
// 	var rb respBean
// 	if err := ffjson.Unmarshal(resp, &rb); err != nil {
// 		return err
// 	}
// 	table := uitable.New()
// 	table.Wrap = true // wrap columns
// 	fmt.Printf("-------------------------------------------------\n")
// 	table.AddRow("EID:", rb.Bean.EID)
// 	table.AddRow("TOKEN:", rb.Bean.Token)
// 	table.AddRow("VALIDITY_DATE:", rb.Bean.ValidityPeriod)
// 	table.AddRow("CA:", rb.Bean.CA)
// 	fmt.Println(table)
// 	return nil
// }

// func (ca *CloudAuth) updateTokenTime() error {
// 	eid, err := checkoutKV(ca.c, "eid")
// 	if err != nil {
// 		return err
// 	}
// 	ttl, err := checkoutKV(ca.c, "ttl")
// 	if err != nil {
// 		return err
// 	}
// 	var gt api_model.GetUserToken
// 	tt, _ := strconv.Atoi(ttl)
// 	gt.Body.EID = eid
// 	gt.Body.ValidityPeriod = tt
// 	if err := clients.RegionClient.Tenants().DefineCloudAuth(&gt).PutToken(); err != nil {
// 		fmt.Printf("update auth %s failure\n", gt.Body.EID)
// 		return err
// 	}
// 	fmt.Printf("update auth %s success\n", gt.Body.EID)
// 	return nil
// }

// func (ca *CloudAuth) getToken() error {
// 	eid, err := checkoutKV(ca.c, "eid")
// 	if err != nil {
// 		return err
// 	}
// 	var gt api_model.GetUserToken
// 	gt.Body.EID = eid
// 	resp, err := clients.RegionClient.Tenants().DefineCloudAuth(&gt).GetToken()
// 	if err != nil {
// 		fmt.Printf("get auth %s failure\n", gt.Body.EID)
// 		return err
// 	}
// 	var rb respBean
// 	if err := ffjson.Unmarshal(resp, &rb); err != nil {
// 		return err
// 	}
// 	table := uitable.New()
// 	table.Wrap = true // wrap columns
// 	fmt.Printf("-------------------------------------------------\n")
// 	table.AddRow("EID:", rb.Bean.EID)
// 	table.AddRow("TOKEN:", rb.Bean.Token)
// 	table.AddRow("VALIDITY_DATE:", rb.Bean.ValidityPeriod)
// 	table.AddRow("CA:", rb.Bean.CA)
// 	fmt.Println(table)
// 	return nil
// }
