// RAINBOND, Application Management Platform
// Copyright (C) 2021-2021 Goodrain Co., Ltd.

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

package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/sirupsen/logrus"
)

// LicenseInfo license data
type LicenseInfo struct {
	Code          string    `json:"code"`
	Company       string    `json:"company"`
	Contact       string    `json:"contact"`
	ExpectCluster int64     `json:"expect_cluster"`
	ExpectNode    int64     `json:"expect_node"`
	ExpectMemory  int64     `json:"expect_memory"`
	EndTime       string    `json:"end_time"`
	StartTime     string    `json:"start_time"`
	Features      []Feature `json:"features"`
}

func (l *LicenseInfo) SetResp(actualCluster, actualNode, actualMemory int64) *LicenseResp {
	return &LicenseResp{
		Code:          l.Code,
		Company:       l.Company,
		Contact:       l.Contact,
		ExpectCluster: l.ExpectCluster,
		ActualCluster: actualCluster,
		ExpectNode:    l.ExpectNode,
		ActualNode:    actualNode,
		ExpectMemory:  l.ExpectMemory,
		ActualMemory:  actualMemory,
		EndTime:       l.EndTime,
		StartTime:     l.StartTime,
		Features:      l.Features,
	}
}

// LicenseResp license resp data
type LicenseResp struct {
	Code          string    `json:"code" description:"code"`
	Company       string    `json:"company" description:"公司名"`
	Contact       string    `json:"contact" description:"联系信息"`
	ExpectCluster int64     `json:"expect_cluster" description:"授权集群数量"`
	ActualCluster int64     `json:"actual_cluster" description:"实际集群数量"`
	ExpectNode    int64     `json:"expect_node" description:"授权节点数量"`
	ActualNode    int64     `json:"actual_node" description:"实际节点数量"`
	ExpectMemory  int64     `json:"expect_memory" description:"授权内存"`
	ActualMemory  int64     `json:"actual_memory" description:"实际内存"`
	EndTime       string    `json:"end_time" description:"结束时间"`
	StartTime     string    `json:"start_time" description:"开始时间"`
	Features      []Feature `json:"features" description:"特性列表"`
}

func (l *LicenseInfo) HaveFeature(code string) bool {
	for _, f := range l.Features {
		if f.Code == strings.ToUpper(code) {
			return true
		}
	}
	return false
}

type Feature struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// ReadLicense -
func ReadLicense(enterpriseID, infoBody string) *LicenseInfo {
	if enterpriseID == "" {
		logrus.Errorf("license id is nil")
		return nil
	}
	salt := []byte(md5String(enterpriseID + string(defaultKey)))
	key := md5String(enterpriseID)
	infoData, err := Decrypt(getKey(key, salt), infoBody)
	if err != nil {
		logrus.Error("decrypt LICENSE failure " + err.Error())
		return nil
	}
	info := LicenseInfo{}
	err = json.Unmarshal(infoData, &info)
	if err != nil {
		logrus.Error("decrypt LICENSE json failure " + err.Error())
		return nil
	}
	return &info
}

// Decrypt -
func Decrypt(key []byte, encrypted string) ([]byte, error) {
	ciphertext, err := base64.RawURLEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(ciphertext, ciphertext)
	return ciphertext, nil
}

// getKey -
func getKey(source string, salt []byte) []byte {
	if len(source) > 32 {
		return []byte(source[:32])
	}
	return append(salt[len(source):], []byte(source)...)
}

// md5String -
func md5String(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

var defaultKey = []byte{113, 119, 101, 114, 116, 121, 117, 105, 111, 112, 97, 115, 100, 102, 103, 104, 106, 107, 108, 122, 120, 99, 118, 98, 110, 109, 49, 50, 51, 52, 53, 54}
