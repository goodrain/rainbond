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

package handler

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	api_db "github.com/goodrain/rainbond/api/db"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/cmd/api/option"

	"github.com/sirupsen/logrus"
)

func TestEmessage(t *testing.T) {
	fmt.Printf("begin.\n")
	conf := option.Config{
		DBType:           "mysql",
		DBConnectionInfo: "admin:admin@tcp(127.0.0.1:3306)/region",
	}
	//创建db manager
	if err := api_db.CreateDBManager(conf); err != nil {
		fmt.Printf("create db manager error, %v", err)
	}
	getLevelLog("dd09a25eb9744afa9b3ad5f5541013e7", "info")
	fmt.Printf("end.\n")
}

func getLevelLog(eventID string, level string) (*api_model.DataLog, error) {
	//messages, err := db.GetManager().EventLogDao().GetEventLogMessages(eventID)
	//if err != nil {
	//	return nil, err
	//}
	////var d []api_model.MessageData
	//for _, v := range messages {
	//	log, err := uncompress(v.Message)
	//	if err != nil {
	//		return nil, err
	//	}
	//	logrus.Debugf("log is %v", log)
	//	fmt.Printf("log is %v", string(log))
	//
	//	var mLogs []msgStruct
	//	if err := ffjson.Unmarshal(log, &mLogs); err != nil {
	//		return nil, err
	//	}
	//	fmt.Printf("jlog %v", mLogs)
	//	break
	//}
	return nil, nil
}

type msgStruct struct {
	EventID string `json:"event_id"`
	Step    string `json:"step"`
	Message string `json:"message"`
	Level   string `json:"level"`
	Time    string `json:"time"`
}

func TestLines(t *testing.T) {
	filePath := "/Users/pujielan/Downloads/log"
	logrus.Debugf("file path is %s", filePath)
	n := 1000
	f, err := exec.Command("tail", "-n", fmt.Sprintf("%d", n), filePath).Output()
	if err != nil {
		fmt.Printf("err if %v", err)
	}
	fmt.Printf("f is %v", string(f))
}

func TestTimes(t *testing.T) {
	//toBeCharge := "2015-01-01 00:00:00"                             //待转化为时间戳的字符串 注意 这里的小时和分钟还要秒必须写 因为是跟着模板走的 修改模板的话也可以不写
	toBeCharge := "2017-09-29T10:02:44+08:00" //待转化为时间戳的字符串 注意 这里的小时和分钟还要秒必须写 因为是跟着模板走的 修改模板的话也可以不写
	timeLayout := "2006-01-02T15:04:05"       //转化所需模板
	loc, _ := time.LoadLocation("Local")      //重要：获取时区
	//toBeCharge = strings.Split(toBeCharge, ".")[0]
	fmt.Println(toBeCharge)
	theTime, err := time.ParseInLocation(timeLayout, toBeCharge, loc) //使用模板在对应时区转化为time.time类型
	fmt.Println(err)
	sr := theTime.Unix() //转化为时间戳 类型是int64
	fmt.Println(theTime) //打印输出theTime 2015-01-01 15:15:00 +0800 CST
	fmt.Println(sr)
}

func TestSort(t *testing.T) {
	arr := [...]int{3, 41, 24, 76, 11, 45, 3, 3, 64, 21, 69, 19, 36}
	fmt.Println(arr)
	num := len(arr)

	//循环排序
	for i := 0; i < num; i++ {
		for j := i + 1; j < num; j++ {
			if arr[i] > arr[j] {
				temp := arr[i]
				arr[i] = arr[j]
				arr[j] = temp
			}
		}
	}
	fmt.Println(arr)
}

func quickSort(array []int, left int, right int) {
	if left < right {
		key := array[left]
		low := left
		high := right
		for low < high {
			for low < high && array[high] > key {
				high--
			}
			array[low] = array[high]
			for low < high && array[low] < key {
				low++
			}
			array[high] = array[low]
		}
		array[low] = key
		quickSort(array, left, low-1)
		quickSort(array, low+1, right)
	}
}

func qsort(array []int, low, high int) {
	if low < high {
		m := partition(array, low, high)
		// fmt.Println(m)
		qsort(array, low, m-1)
		qsort(array, m+1, high)
	}
}

func partition(array []int, low, high int) int {
	key := array[low]
	tmpLow := low
	tmpHigh := high
	for {
		//查找小于等于key的元素，该元素的位置一定是tmpLow到high之间，因为array[tmpLow]及左边元素小于等于key，不会越界
		for array[tmpHigh] > key {
			tmpHigh--
		}
		//找到大于key的元素，该元素的位置一定是low到tmpHigh+1之间。因为array[tmpHigh+1]必定大于key
		for array[tmpLow] <= key && tmpLow < tmpHigh {
			tmpLow++
		}

		if tmpLow >= tmpHigh {
			break
		}
		// swap(array[tmpLow], array[tmpHigh])
		array[tmpLow], array[tmpHigh] = array[tmpHigh], array[tmpLow]
		fmt.Println(array)
	}
	array[tmpLow], array[low] = array[low], array[tmpLow]
	return tmpLow
}

func TestFastSort(t *testing.T) {
	var sortArray = []int{3, 41, 24, 76, 11, 45, 3, 3, 64, 21, 69, 19, 36}
	fmt.Println(sortArray)
	qsort(sortArray, 0, len(sortArray)-1)
	fmt.Println(sortArray)
}
