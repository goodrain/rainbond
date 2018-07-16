package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/db"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"regexp"
)

var re = regexp.MustCompile(`\s`)

type AppAction struct {
	MQClient  pb.TaskQueueClient
	staticDir string
}

func (a *AppAction) GetStaticDir() string {
	return a.staticDir
}

func CreateAppManager(mqClient pb.TaskQueueClient) *AppAction {
	return &AppAction{
		MQClient:  mqClient,
		staticDir: "/grdata/app",
	}
}

func (a *AppAction) Complete(tr *model.ExportAppStruct) error {
	appName := gjson.Get(tr.Body.GroupMetadata, "group_name").String()
	if appName == "" {
		err := errors.New("Failed to get group name form metadata.")
		logrus.Error(err)
		return err
	}

	if tr.Body.Format != "rainbond-app" && tr.Body.Format != "docker-compose" {
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

	// 构建MQ事件对象
	mqBody, err := json.Marshal(model.BuildMQBodyFrom(tr))
	if err != nil {
		logrus.Error("Failed to encode json from ExportAppStruct:", err)
		return err
	}

	ts := &db.BuildTaskStruct{
		TaskType: "export_app",
		TaskBody: mqBody,
	}

	eq, err := db.BuildTaskBuild(ts)
	if err != nil {
		logrus.Error("Failed to BuildTaskBuild for ExportApp:", err)
		return err
	}

	// 写入事件到MQ中
	ctx, cancel := context.WithCancel(context.Background())
	_, err = a.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Error("Failed to Enqueue MQ for ExportApp:", err)
		return err
	}

	return nil
}

func (a *AppAction) ImportApp(importApp *model.ImportAppStruct) error {
	mqBody, err := json.Marshal(importApp)

	ts := &db.BuildTaskStruct{
		TaskType: "import_app",
		TaskBody: mqBody,
	}

	eq, err := db.BuildTaskBuild(ts)
	if err != nil {
		logrus.Error("Failed to BuildTaskBuild for ImportApp:", err)
		return err
	}

	// 写入事件到MQ中
	ctx, cancel := context.WithCancel(context.Background())
	_, err = a.MQClient.Enqueue(ctx, eq)
	cancel()
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
