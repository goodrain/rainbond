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

package license

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"plugin"
	"strconv"

	"github.com/sirupsen/logrus"
)

var enterprise = "false"

// LicInfo license information
type LicInfo struct {
	LicKey     string   `json:"license_key"`
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

func isEnterprise() bool {
	res, err := strconv.ParseBool(enterprise)
	if err != nil {
		logrus.Warningf("enterprise: %s; error parsing 'string' to 'bool': %v", enterprise, err)
	}
	return res
}

func readFromFile(lfile string) (string, error) {
	_, err := os.Stat(lfile)
	if err != nil {
		logrus.Errorf("license file is incorrect: %v", err)
		return "", err
	}
	bytes, err := ioutil.ReadFile(lfile)
	if err != nil {
		logrus.Errorf("license file: %s; error reading license file: %v", lfile, err)
		return "", err
	}
	return string(bytes), nil
}

// VerifyTime verifies the time in the license.
func VerifyTime(licPath, licSoPath string) bool {
	if !isEnterprise() {
		return true
	}
	lic, err := readFromFile(licPath)
	if err != nil {
		logrus.Errorf("failed to read license from file: %v", err)
		return false
	}
	p, err := plugin.Open(licSoPath)
	if err != nil {
		logrus.Errorf("license.so path: %s; error opening license.so: %v", licSoPath, err)
		return false
	}
	f, err := p.Lookup("VerifyTime")
	if err != nil {
		logrus.Errorf("method 'VerifyTime'; error looking up func: %v", err)
		return false
	}
	return f.(func(string) bool)(lic)
}

// VerifyNodes verifies the number of the nodes in the license.
func VerifyNodes(licPath, licSoPath string, nodeNums int) bool {
	if !isEnterprise() {
		return true
	}
	lic, err := readFromFile(licPath)
	if err != nil {
		logrus.Errorf("failed to read license from file: %v", err)
		return false
	}
	p, err := plugin.Open(licSoPath)
	if err != nil {
		logrus.Errorf("license.so path: %s; error opening license.so: %v", licSoPath, err)
		return false
	}
	f, err := p.Lookup("VerifyNodes")
	if err != nil {
		logrus.Errorf("method 'VerifyNodes'; error looking up func: %v", err)
		return false
	}
	return f.(func(string, int) bool)(lic, nodeNums)
}

// GetLicInfo -
func GetLicInfo(licPath, licSoPath string) (*LicInfo, error) {
	if !isEnterprise() {
		return nil, nil
	}
	lic, err := readFromFile(licPath)
	if err != nil {
		logrus.Errorf("failed to read license from file: %v", err)
		return nil, fmt.Errorf("failed to read license from file: %v", err)
	}
	p, err := plugin.Open(licSoPath)
	if err != nil {
		logrus.Errorf("license.so path: %s; error opening license.so: %v", licSoPath, err)
		return nil, fmt.Errorf("license.so path: %s; error opening license.so: %v", licSoPath, err)
	}

	f, err := p.Lookup("Decrypt")
	if err != nil {
		logrus.Errorf("method 'Decrypt'; error looking up func: %v", err)
		return nil, fmt.Errorf("method 'Decrypt'; error looking up func: %v", err)
	}
	bytes, err := f.(func(string) ([]byte, error))(lic)
	var licInfo LicInfo
	if err := json.Unmarshal(bytes, &licInfo); err != nil {
		logrus.Errorf("error unmarshalling license: %v", err)
		return nil, fmt.Errorf("error unmarshalling license: %v", err)
	}
	return &licInfo, nil
}

// GenLicKey -
func GenLicKey(licSoPath string) (string, error) {
	if !isEnterprise() {
		return "", nil
	}
	p, err := plugin.Open(licSoPath)
	if err != nil {
		logrus.Errorf("license.so path: %s; error opening license.so: %v", licSoPath, err)
		return "", fmt.Errorf("license.so path: %s; error opening license.so: %v", licSoPath, err)
	}

	f, err := p.Lookup("GenLicKey")
	if err != nil {
		logrus.Errorf("method 'GenLicKey'; error looking up func: %v", err)
		return "", fmt.Errorf("method 'GenLicKey'; error looking up func: %v", err)
	}
	return f.(func() (string, error))()
}
