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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"
)

//Info license 信息
type Info struct {
	Code       string   `json:"code"`
	Company    string   `json:"company"`
	Node       int64    `json:"node"`
	CPU        int64    `json:"cpu"`
	Memory     int64    `json:"memory"`
	Tenant     int64    `json:"tenant"`
	EndTime    string   `json:"end_time"`
	StartTime  string   `json:"start_time"`
	DataCenter int64    `json:"data_center"`
	ModuleList []string `json:"module_list"`
}

var key = []byte("qa123zxswe3532crfvtg123bnhymjuki")

//decrypt 解密算法
func decrypt(key []byte, encrypted string) ([]byte, error) {
	return []byte{}, nil
}

//ReadLicenseFromFile 从文件获取license
func ReadLicenseFromFile(licenseFile string) (Info, error) {

	info := Info{}
	//step1 read license file
	_, err := os.Stat(licenseFile)
	if err != nil {
		return info, err
	}
	infoBody, err := ioutil.ReadFile(licenseFile)
	if err != nil {
		return info, errors.New("LICENSE文件不可读")
	}

	//step2 decryption info
	infoData, err := decrypt(key, string(infoBody))
	if err != nil {
		return info, errors.New("LICENSE解密发生错误。")
	}
	err = json.Unmarshal(infoData, &info)
	if err != nil {
		return info, errors.New("解码LICENSE文件发生错误")
	}
	return info, nil
}

//ReadLicenseFromConsole 从控制台api获取license
func ReadLicenseFromConsole(token string, defaultLicense string) (Info, error) {
	var info Info
	req, err := http.NewRequest("GET", "http://console.goodrain.me/api/license", nil)
	if err != nil {
		return info, err
	}
	req.Header.Add("Content-Type", "application/json")
	if token != "" {
		req.Header.Add("Authorization", "Token "+token)
	}
	http.DefaultClient.Timeout = time.Second * 5
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		glog.Info("控制台读取授权失败，使用默认授权。")
		return ReadLicenseFromFile(defaultLicense)
	}
	if res != nil {
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return info, err
		}
		apiInfo := struct {
			Body struct {
				Bean string `json:"bean"`
			}
		}{}

		err = json.Unmarshal(body, &apiInfo)
		if err != nil {
			return info, err
		}
		//step2 decryption info
		infoData, err := decrypt(key, apiInfo.Body.Bean)
		if err != nil {
			return info, errors.New("LICENSE解密发生错误。")
		}
		err = json.Unmarshal(infoData, &info)
		if err != nil {
			return info, errors.New("解码LICENSE文件发生错误")
		}
		return info, nil
	}
	return info, errors.New("res body is nil")
}

//BasePack base pack
func BasePack(text []byte) (string, error) {
	token := ""
	encodeStr := base64.StdEncoding.EncodeToString(text)
	begin := 0
	if len([]byte(encodeStr)) > 40 {
		begin = randInt(0, (len([]byte(encodeStr)) - 40))
	} else {
		return token, fmt.Errorf("error license")
	}
	token = string([]byte(encodeStr)[begin:(begin + 40)])
	return token, nil
}

func randInt(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}
