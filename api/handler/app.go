package handler

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/mq/client"

	"regexp"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

var re = regexp.MustCompile(`\s`)

//AppAction app action
type AppAction struct {
	MQClient  client.MQClient
	staticDir string
}

//GetStaticDir get static dir
func (a *AppAction) GetStaticDir() string {
	return a.staticDir
}

//CreateAppManager create app manager
func CreateAppManager(mqClient client.MQClient) *AppAction {
	staticDir := "/grdata/app"
	if os.Getenv("LOCAL_APP_CACHE_DIR") != "" {
		staticDir = os.Getenv("LOCAL_APP_CACHE_DIR")
	}
	return &AppAction{
		MQClient:  mqClient,
		staticDir: staticDir,
	}
}

//Complete Complete
func (a *AppAction) Complete(tr *model.ExportAppStruct) error {
	appName := gjson.Get(tr.Body.GroupMetadata, "group_name").String()
	if appName == "" {
		err := errors.New("Failed to get group name form metadata")
		logrus.Error(err)
		return err
	}

	if tr.Body.Format != "rainbond-app" && tr.Body.Format != "docker-compose" && tr.Body.Format != "slug" && tr.Body.Format != "helm-chart" {
		err := errors.New("Unsupported the format: " + tr.Body.Format)
		logrus.Error(err)
		return err
	}

	version := gjson.Get(tr.Body.GroupMetadata, "group_version").String()

	appName = unicode2zh(appName)
	tr.SourceDir = fmt.Sprintf("%s/%s/%s-%s", a.staticDir, tr.Body.Format, appName, version)

	return nil
}

//ExportApp ExportApp
func (a *AppAction) ExportApp(tr *model.ExportAppStruct) error {
	// 保存元数据到组目录
	if err := saveMetadata(tr); err != nil {
		return util.CreateAPIHandleErrorFromDBError("Failed to export app", err)
	}
	err := a.MQClient.SendBuilderTopic(client.TaskStruct{
		TaskBody: model.BuildMQBodyFrom(tr),
		TaskType: "export_app",
		Topic:    client.BuilderTopic,
	})
	if err != nil {
		logrus.Error("Failed to Enqueue MQ for ExportApp:", err)
		return err
	}

	return nil
}

//ImportApp import app
func (a *AppAction) ImportApp(importApp *model.ImportAppStruct) error {

	err := a.MQClient.SendBuilderTopic(client.TaskStruct{
		TaskBody: importApp,
		TaskType: "import_app",
		Topic:    client.BuilderTopic,
	})
	if err != nil {
		logrus.Error("Failed to MQ Enqueue for ImportApp:", err)
		return err
	}
	logrus.Debugf("equeue mq build plugin from image success")

	return nil
}

func saveMetadata(tr *model.ExportAppStruct) error {
	// 创建应用组目录
	os.MkdirAll(tr.SourceDir, 0755)

	// 写入元数据到文件
	err := ioutil.WriteFile(fmt.Sprintf("%s/metadata.json", tr.SourceDir), []byte(tr.Body.GroupMetadata), 0644)
	if err != nil {
		logrus.Error("Failed to save metadata", err)
		return err
	}

	return nil
}

// unicode2zh 将unicode转为中文，并去掉空格
func unicode2zh(uText string) (context string) {
	for i, char := range strings.Split(uText, `\\u`) {
		if i < 1 {
			context = char
			continue
		}

		length := len(char)
		if length > 3 {
			pre := char[:4]
			zh, err := strconv.ParseInt(pre, 16, 32)
			if err != nil {
				context += char
				continue
			}

			context += fmt.Sprintf("%c", zh)

			if length > 4 {
				context += char[4:]
			}
		}

	}

	context = re.ReplaceAllString(context, "")

	return context
}
