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

package cmd
import (
	"github.com/urfave/cli"
	"fmt"
	"bytes"
	"os/exec"
)

func NewCmdDomain() cli.Command {
	c:=cli.Command{
		Name: "domain",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "ip",
				Usage: "ip address",
			},
			cli.StringFlag{
				Name:  "domain",
				Usage: "domain",
			},
		},
		Usage: "",
		Action: func(c *cli.Context) error {
			ip:=c.String("ip")
			if len(ip)==0 {
				fmt.Println("ip must not null")
				return nil
			}
			domain:=c.String("domain")
			cmd := exec.Command("bash", "-c","set "+ip+" "+domain+";"," /usr/share/gr-rainbond-node/gaops/jobs/install/manage/tasks/ex_domain.sh")
			//cmd := exec.Command("bash", "-c","set "+ip+" "+domain+";"," /usr/share/gr-rainbond-node/gaops/jobs/install/manage/tasks/ex_domain.sh")
			buf:=bytes.NewBuffer(nil)
			outbuf:=bytes.NewBuffer(nil)
			cmd.Stderr=buf
			cmd.Stdout=outbuf
			cmd.Run()
			out:=buf.String()
			out_:=outbuf.String()
			fmt.Println(out)
			fmt.Println(out_)
			return nil
		},
	}
	return c
}




