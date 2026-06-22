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
	"github.com/goodrain/rainbond/pkg/component/storage"

	"github.com/goodrain/rainbond/builder/sources"
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

// ImportApp Export app to specified format(rainbond-app or dockercompose)
type ImportApp struct {
	EventID       string             `json:"event_id"`
	Format        string             `json:"format"`
	SourceDir     string             `json:"source_dir"`
	Apps          []string           `json:"apps"`
	ServiceImage  model.ServiceImage `json:"service_image"`
	Logger        event.Logger
	oldAPPPath    map[string]string
	oldPluginPath map[string]string
	ImageClient   sources.ImageClient
}

// NewImportApp create
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
	importApp.ImageClient = m.imageClient

	importApp.oldAPPPath = make(map[string]string)
	importApp.oldPluginPath = make(map[string]string)
	return &importApp, nil
}

// Stop stop
func (i *ImportApp) Stop() error {
	return nil
}

// Name return worker name
func (i *ImportApp) Name() string {
	return "import_app"
}

// GetLogger GetLogger
func (i *ImportApp) GetLogger() event.Logger {
	return i.Logger
}

// ErrorCallBack if run error will callback
func (i *ImportApp) ErrorCallBack(err error) {
	i.updateStatus("failed")
}

// Run Run
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
	datas, err := runImportAppTasks(i.Apps, func(app string) (*v1alpha1.RainbondApplicationConfig, error) {
		appFile := filepath.Join(oldSourceDir, app)
		err := storage.Default().StorageCli.DownloadDirToDir(oldSourceDir, oldSourceDir)
		if err != nil {
			logrus.Errorf("s3 download dir to dir failure %s", err.Error())
			i.updateStatusForApp(app, "failed")
			return nil, err
		}
		tmpDir := path.Join(oldSourceDir, app+"-cache")
		li, err := localimport.New(logrus.StandardLogger(), i.ImageClient.GetContainerdClient(), i.ImageClient.GetDockerClient(), tmpDir)
		if err != nil {
			logrus.Errorf("create localimport failure %s", err.Error())
			i.updateStatusForApp(app, "failed")
			return nil, err
		}
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
			return nil, err
		}
		if rawMetadata, err := readImportedMetadata(tmpDir); err == nil {
			normalizeImportedRAM(rawMetadata, ram)
		} else {
			logrus.Warningf("read imported metadata for %s failure: %v", appFile, err)
		}
		if err := ensureImportedImagesPushed(i.ImageClient, ram, i.ServiceImage, i.Logger); err != nil {
			logrus.Errorf("Failed to push imported app images %s: %v", appFile, err)
			i.updateStatusForApp(app, "failed")
			return nil, err
		}
		os.Rename(appFile, appFile+".success")
		logrus.Infof("Successful import app: %s", appFile)
		os.Remove(tmpDir)
		return ram, nil
	})
	if err != nil {
		if updateErr := i.updateStatus("failed"); updateErr != nil {
			logrus.Errorf("Failed to update import status to failed for %s: %v", i.SourceDir, updateErr)
		}
		return err
	}
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
	err = storage.Default().StorageCli.UploadFileToFile(metadatasFile, metadatasFile, nil)
	if err != nil {
		logrus.Errorf("Failed to upload apps %s metadatas.json: %v", i.SourceDir, err)
		return err
	}
	return nil
}

func runImportAppTasks(apps []string, task func(string) (*v1alpha1.RainbondApplicationConfig, error)) ([]v1alpha1.RainbondApplicationConfig, error) {
	results := make([]*v1alpha1.RainbondApplicationConfig, len(apps))
	errCh := make(chan error, len(apps))
	var wait sync.WaitGroup
	var mu sync.Mutex

	for idx, app := range apps {
		idx, app := idx, app
		wait.Add(1)
		go func() {
			defer wait.Done()
			ram, err := task(app)
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", app, err)
				return
			}
			mu.Lock()
			results[idx] = ram
			mu.Unlock()
		}()
	}

	wait.Wait()
	close(errCh)

	var importErr error
	for err := range errCh {
		importErr = errors.Join(importErr, err)
	}
	if importErr != nil {
		return nil, importErr
	}

	datas := make([]v1alpha1.RainbondApplicationConfig, 0, len(results))
	for _, ram := range results {
		if ram == nil {
			continue
		}
		datas = append(datas, *ram)
	}
	return datas, nil
}

func ensureImportedImagesPushed(imageClient sources.ImageClient, ram *v1alpha1.RainbondApplicationConfig, serviceImage model.ServiceImage, logger event.Logger) error {
	if imageClient == nil || ram == nil {
		return nil
	}
	pushed := make(map[string]struct{})
	for _, imageName := range importedImageNames(ram) {
		if imageName == "" {
			continue
		}
		if _, ok := pushed[imageName]; ok {
			continue
		}
		pushed[imageName] = struct{}{}
		logrus.Infof("wait for imported image push completion: %s", imageName)
		if err := imageClient.ImagePush(imageName, serviceImage.HubUser, serviceImage.HubPassword, logger, 20); err != nil {
			return fmt.Errorf("push imported image %s: %w", imageName, err)
		}
	}
	return nil
}

func importedImageNames(ram *v1alpha1.RainbondApplicationConfig) []string {
	var imageNames []string
	for _, component := range ram.Components {
		if component == nil {
			continue
		}
		imageNames = append(imageNames, strings.TrimSpace(component.ShareImage))
	}
	for _, plugin := range ram.Plugins {
		if plugin == nil {
			continue
		}
		imageNames = append(imageNames, strings.TrimSpace(plugin.ShareImage))
	}
	return imageNames
}

type importedRAMMetadata struct {
	Components []importedComponentMetadata `json:"apps"`
}

type importedComponentMetadata struct {
	ExtendMethodMap     map[string]interface{} `json:"extend_method_map"`
	ServiceExtendMethod map[string]interface{} `json:"service_extend_method"`
	CPU                 *int                   `json:"cpu"`
	Memory              *int                   `json:"memory"`
	DeployType          string                 `json:"extend_method"`
	ServiceType         string                 `json:"service_type"`
}

func readImportedMetadata(tmpDir string) ([]byte, error) {
	files, err := ioutil.ReadDir(tmpDir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("metadata dir is empty: %s", tmpDir)
	}
	return ioutil.ReadFile(path.Join(tmpDir, files[0].Name(), "metadata.json"))
}

func normalizeImportedRAM(rawMetadata []byte, ram *v1alpha1.RainbondApplicationConfig) {
	var metadata importedRAMMetadata
	if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
		return
	}
	for idx := range ram.Components {
		if idx >= len(metadata.Components) || ram.Components[idx] == nil {
			continue
		}
		normalizeImportedComponent(ram.Components[idx], metadata.Components[idx])
	}
}

func normalizeImportedComponent(component *v1alpha1.Component, metadata importedComponentMetadata) {
	normalizeImportedExtendMethodRule(&component.ExtendMethodRule, metadata)
	normalizeImportedComponentResources(component, metadata)
	if isDaemonSetImportedComponent(component, metadata) {
		clearDaemonSetNodeScaling(&component.ExtendMethodRule)
	}
}

func normalizeImportedComponentResources(component *v1alpha1.Component, metadata importedComponentMetadata) {
	if shouldRestoreLegacyCPU(component, metadata) {
		if value, ok := legacyExtendMethodValueWithPresence(metadata, "container_cpu"); ok {
			component.CPU = value
		}
	}
	if component.Memory == 0 && metadata.Memory == nil {
		if value, ok := legacyExtendMethodValueWithPresence(metadata, "init_memory"); ok {
			component.Memory = value
			return
		}
		if value, ok := legacyExtendMethodValueWithPresence(metadata, "container_memory"); ok {
			component.Memory = value
			return
		}
		if value := legacyExtendMethodValue(metadata, "min_memory"); value != 0 {
			component.Memory = value
		}
	}
}

func shouldRestoreLegacyCPU(component *v1alpha1.Component, metadata importedComponentMetadata) bool {
	return component.CPU == 250 || (component.CPU == 0 && metadata.CPU == nil)
}

func isDaemonSetImportedComponent(component *v1alpha1.Component, metadata importedComponentMetadata) bool {
	return strings.EqualFold(string(component.DeployType), "daemonset") ||
		strings.EqualFold(component.ServiceType, "daemonset") ||
		strings.EqualFold(metadata.DeployType, "daemonset") ||
		strings.EqualFold(metadata.ServiceType, "daemonset")
}

func clearDaemonSetNodeScaling(rule *v1alpha1.ComponentExtendMethodRule) {
	rule.MinNode = 0
	rule.MaxNode = 0
	rule.StepNode = 0
}

func normalizeImportedExtendMethodRule(rule *v1alpha1.ComponentExtendMethodRule, metadata importedComponentMetadata) {
	if rule.MinNode == 0 {
		rule.MinNode = legacyExtendMethodValue(metadata, "min_node")
	}
	if rule.MaxNode == 0 {
		rule.MaxNode = legacyExtendMethodValue(metadata, "max_node")
	}
	if rule.StepNode == 0 {
		rule.StepNode = legacyExtendMethodValue(metadata, "step_node")
	}
	if rule.MinMemory == 0 {
		rule.MinMemory = legacyExtendMethodValue(metadata, "min_memory")
	}
	if rule.MaxMemory == 0 {
		rule.MaxMemory = legacyExtendMethodValue(metadata, "max_memory")
	}
	if rule.StepMemory == 0 {
		rule.StepMemory = legacyExtendMethodValue(metadata, "step_memory")
	}
	if rule.IsRestart == 0 {
		rule.IsRestart = legacyExtendMethodValue(metadata, "is_restart")
	}
	if rule.InitMemory == 0 {
		if value, ok := legacyExtendMethodValueWithPresence(metadata, "init_memory"); ok {
			rule.InitMemory = value
		} else {
			rule.InitMemory = rule.MinMemory
		}
	}
}

func legacyExtendMethodValue(metadata importedComponentMetadata, key string) int {
	if value, ok := metadata.ExtendMethodMap[key]; ok {
		if intValue := metadataValueToInt(value); intValue != 0 {
			return intValue
		}
	}
	return metadataValueToInt(metadata.ServiceExtendMethod[key])
}

func metadataValueToInt(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case bool:
		if v {
			return 1
		}
	}
	return 0
}

func legacyExtendMethodValueWithPresence(metadata importedComponentMetadata, key string) (int, bool) {
	if value, ok := metadata.ExtendMethodMap[key]; ok {
		return metadataValueToInt(value), true
	}
	if value, ok := metadata.ServiceExtendMethod[key]; ok {
		return metadataValueToInt(value), true
	}
	return 0, false
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
