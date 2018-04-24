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

package core

import (
	"github.com/goodrain/rainbond/cmd/node/option"

	"net/http"
	"time"
	"os/exec"
	"strings"
	"bytes"
	"github.com/Sirupsen/logrus"
)
//t==0 url:http     t==1 cmd:service_status
func StartGoruntine(todo string,t int, stopCheck chan int) {
	failedCount := 0
	interval:=option.Config.CheckIntervalSec
	for {
		select {

		case <-stopCheck:
			return 

		default:
			time.Sleep(time.Duration(interval)*time.Second)
			if check(todo,t) {
				continue
			}else {
				if failedCount<option.Config.FailTime {
					failedCount=failedCount+1
				}else{
					//检测失败逻辑，notify
					checkFailed(todo,t)
				}
			}

		}
	}
}
func checkFailed(url string,ty int){

}
func check(todo string, ty int) bool {
	if ty == 0 {
		return checkHttp(todo)
	}else if ty==1 {
		return checkCommand(todo)
	}
	return false;
}
func checkCommand(cmd string) bool {
	toRun:=strings.Split(cmd," ")
	c:=exec.Command(toRun[0],toRun[1:]...)
	var b bytes.Buffer
	c.Stdout=&b

	err:=c.Run()
	if err != nil {
		logrus.Warnf("check %s 's status failed,details %s",cmd,err.Error())
		return false
	}
	logrus.Infof("check %s 's result is %s",cmd,b.String())
	return true
}
func checkHttp(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}

	defer resp.Body.Close()
	if resp.StatusCode/100!=2{
		return false
	}
	return true
}
