package cmd

import (
	"github.com/urfave/cli"
	"github.com/goodrain/rainbond/grctl/clients"
	"fmt"
	"github.com/apcera/termtables"
	"time"
	"strconv"
)

//NewCmdNode NewCmdNode
func NewCmdNotificationEvent() cli.Command {
	c := cli.Command{
		Name:  "notification",
		Usage: "应用异常通知事件。grctl notification",
		Subcommands: []cli.Command{
			{
				Name:  "get",
				Usage: "get notification",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "StartTime,st",
						Value: "",
						Usage: "StartTime timestamp",
					},
					cli.StringFlag{
						Name:  "EndTime,et",
						Value: "",
						Usage: "EndTime timestamp",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					startTime := c.String("StartTime")
					EndTme := c.String("EndTime")
					if startTime == "" && EndTme == ""{
						NowTime := time.Now()
						startTimeTimestamp := NowTime.AddDate(0,0,-3).Unix()
						startTime = strconv.FormatInt(startTimeTimestamp, 10)
						EndTme = strconv.FormatInt(NowTime.Unix(), 10)
					} else if EndTme == "" && startTime != "" {
						NowTime := time.Now().Unix()
						EndTme = strconv.FormatInt(NowTime, 10)
					}
					val, err := clients.RegionClient.Notification().GetNotification(startTime, EndTme)
					handleErr(err)
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("ServiceName(应用别名)", "TenantName(租户别名)", "Message(异常信息)", "Reason(异常原因)", "Count(出现次数)", "LastTime(最后一次异常时间)", "FirstTime(第一次异常时间)")
					for _, v := range val {
						if v.KindID == "" || v.ServiceName == "" || v.TenantName == ""{
							continue
						}
						serviceTable.AddRow(v.ServiceName, v.TenantName, v.Message, v.Reason, v.Count, v.LastTime, v.FirstTime)
					}
					fmt.Println(serviceTable.Render())
					return nil
				},
			},
		},
	}
	return c
}
