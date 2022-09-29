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

package exector

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/goodrain/rainbond-oam/pkg/export"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond-oam/pkg/localimport"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/sirupsen/logrus"
)

func init() {
	RegisterWorker("import_app", NewImportApp)
}

//ImportApp Export app to specified format(rainbond-app or dockercompose)
type ImportApp struct {
	EventID       string             `json:"event_id"`
	Format        string             `json:"format"`
	SourceDir     string             `json:"source_dir"`
	Apps          []string           `json:"apps"`
	ServiceImage  model.ServiceImage `json:"service_image"`
	Logger        event.Logger
	oldAPPPath    map[string]string
	oldPluginPath map[string]string
	ContainerdCli export.ContainerdAPI
}

//NewImportApp create
func NewImportApp(in []byte, m *exectorManager) (TaskWorker, error) {
	var importApp ImportApp
	if err := json.Unmarshal(in, &importApp); err != nil {
		return nil, err
	}
	if importApp.ServiceImage.HubURL == "" || importApp.ServiceImage.HubURL == "goodrain.me" {
		importApp.ServiceImage.HubURL = builder.REGISTRYDOMAIN
		importApp.ServiceImage.HubUser = builder.REGISTRYUSER
		importApp.ServiceImage.HubPassword = builder.REGISTRYPASS
	}
	logrus.Infof("load app image to hub %s", importApp.ServiceImage.HubURL)
	importApp.Logger = event.GetManager().GetLogger(importApp.EventID)
	importApp.ContainerdCli = m.ContainerdCli
	importApp.oldAPPPath = make(map[string]string)
	importApp.oldPluginPath = make(map[string]string)
	return &importApp, nil
}

//Stop stop
func (i *ImportApp) Stop() error {
	return nil
}

//Name return worker name
func (i *ImportApp) Name() string {
	return "import_app"
}

//GetLogger GetLogger
func (i *ImportApp) GetLogger() event.Logger {
	return i.Logger
}

//ErrorCallBack if run error will callback
func (i *ImportApp) ErrorCallBack(err error) {
	i.updateStatus("failed")
}

//Run Run
func (i *ImportApp) Run(timeout time.Duration) error {
	if i.Format == "rainbond-app" {
		err := i.importApp()
		if err != nil {
			logrus.Errorf("load rainbond app failure %s", err.Error())
			return err
		}
		return nil
	}
	return errors.New("Unsupported the format: " + i.Format)
}

// importApp import app
// support batch import
func (i *ImportApp) importApp() error {
	oldSourceDir := i.SourceDir
	var datas []v1alpha1.RainbondApplicationConfig
	var wait sync.WaitGroup
	for _, app := range i.Apps {
		wait.Add(1)
		go func(app string) {
			defer wait.Done()
			appFile := filepath.Join(oldSourceDir, app)
			tmpDir := path.Join(oldSourceDir, app+"-cache")
			li := localimport.New(logrus.StandardLogger(), i.ContainerdCli, tmpDir)
			if err := i.updateStatusForApp(app, "importing"); err != nil {
				logrus.Errorf("Failed to update status to importing for app %s: %v", app, err)
			}
			ram, err := li.Import(appFile, v1alpha1.ImageInfo{
				HubURL:      i.ServiceImage.HubURL,
				HubUser:     i.ServiceImage.HubUser,
				HubPassword: i.ServiceImage.HubPassword,
				Namespace:   i.ServiceImage.NameSpace,
			})
			if err != nil {
				logrus.Errorf("Failed to load app %s: %v", appFile, err)
				i.updateStatusForApp(app, "failed")
				return
			}
			os.Rename(appFile, appFile+".success")
			datas = append(datas, *ram)
			logrus.Infof("Successful import app: %s", appFile)
			os.Remove(tmpDir)
		}(app)
	}
	wait.Wait()
	metadatasFile := fmt.Sprintf("%s/metadatas.json", i.SourceDir)
	dataBytes, _ := json.Marshal(datas)
	if err := ioutil.WriteFile(metadatasFile, []byte(dataBytes), 0644); err != nil {
		logrus.Errorf("Failed to load apps %s: %v", i.SourceDir, err)
		return err
	}
	if err := i.updateStatus("success"); err != nil {
		logrus.Errorf("Failed to load apps %s: %v", i.SourceDir, err)
		return err
	}
	return nil
}

func (i *ImportApp) updateStatus(status string) error {
	logrus.Debug("Update app status in database to: ", status)
	// 从数据库中获取该应用的状态信息
	res, err := db.GetManager().AppDao().GetByEventId(i.EventID)
	if err != nil {
		err = fmt.Errorf("failed to get app %s from db: %s", i.EventID, err.Error())
		logrus.Error(err)
		return err
	}

	// 在数据库中更新该应用的状态信息
	res.Status = status

	if err := db.GetManager().AppDao().UpdateModel(res); err != nil {
		err = fmt.Errorf("failed to update app %s: %s", i.EventID, err.Error())
		logrus.Error(err)
		return err
	}

	return nil
}

func (i *ImportApp) updateStatusForApp(app, status string) error {
	logrus.Debugf("Update status in database for app %s to: %s", app, status)
	// 从数据库中获取该应用的状态信息
	res, err := db.GetManager().AppDao().GetByEventId(i.EventID)
	if err != nil {
		err = fmt.Errorf("Failed to get app %s from db: %s", i.EventID, err.Error())
		logrus.Error(err)
		return err
	}

	// 在数据库中更新该应用的状态信息
	appsMap := str2map(res.Apps)
	appsMap[app] = status
	res.Apps = map2str(appsMap)

	if err := db.GetManager().AppDao().UpdateModel(res); err != nil {
		err = fmt.Errorf("Failed to update app %s: %s", i.EventID, err.Error())
		logrus.Error(err)
		return err
	}

	return nil
}

func str2map(str string) map[string]string {
	result := make(map[string]string, 10)

	for _, app := range strings.Split(str, ",") {
		appMap := strings.Split(app, ":")
		result[appMap[0]] = appMap[1]
	}

	return result
}

func map2str(m map[string]string) string {
	var result string

	for k, v := range m {
		kv := k + ":" + v

		if result == "" {
			result += kv
		} else {
			result += "," + kv
		}
	}

	return result
}

// 只保留"/"后面的部分，并去掉不合法字符，一般用于把导出的镜像文件还原为镜像名
func buildFromLinuxFileName(fileName string) string {
	if fileName == "" {
		return fileName
	}

	arr := strings.Split(fileName, "/")

	if str := arr[len(arr)-1]; str == "" {
		fileName = strings.Replace(fileName, "---", "/", -1)
	} else {
		fileName = str
	}

	fileName = strings.Replace(fileName, "--", ":", -1)
	fileName = strings.TrimSpace(fileName)

	return fileName
}
