package handler

import (
	"context"
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/db"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"io/ioutil"
	"os"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/pkg/errors"
	"strings"
	"strconv"
)

type AppAction struct {
	MQClient pb.TaskQueueClient
}

func CreateAppManager(mqClient pb.TaskQueueClient) *AppAction {
	return &AppAction{
		MQClient: mqClient,
	}
}

func (a *AppAction) Complete(tr *model.ExportAppStruct) error {
	appName := gjson.Get(tr.Body.GroupMetadata, ".group_name").String()
	if appName == "" {
		err := errors.New("Failed to get group name form metadata.")
		logrus.Error(err)
		return err
	}

	tr.Body.GroupName = appName
	appName = unicode2zh(appName)
	tr.SourceDir = fmt.Sprintf("/grdata/export-app/%s-%s", appName, tr.Body.Version)

	return nil
}

func (a *AppAction) ExportApp(tr *model.ExportAppStruct) error {
	if err := saveMetadata(tr); err != nil {
		return util.CreateAPIHandleErrorFromDBError("Failed to export app", err)
	}

	mqBody, err := json.Marshal(BuildExportAppBody(tr))
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

	ctx, cancel := context.WithCancel(context.Background())
	_, err = a.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Error("Failed to Enqueue MQ for ExportApp:", err)
		return err
	}
	logrus.Debugf("equeue mq build plugin from image success")

	return nil
}


// TODO 与ExportApp函数唯一不同的是导出目录，以后有可能合并
func (a *AppAction) ExportRunnableApp(tr *model.ExportAppStruct) error {
	appName := gjson.Get(tr.Body.GroupMetadata, ".group_name").String()
	if appName == "" {
		err := errors.New("Failed to get group name form metadata.")
		logrus.Error(err)
		return err
	}

	appName = unicode2zh(appName)

	tr.SourceDir = fmt.Sprintf("/grdata/export-runnable-app/%s-%s", appName, tr.Body.Version)

	if err := saveMetadata(tr); err != nil {
		return util.CreateAPIHandleErrorFromDBError("Failed to export app", err)
	}

	mqBody, err := json.Marshal(BuildExportAppBody(tr))
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

	ctx, cancel := context.WithCancel(context.Background())
	_, err = a.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Error("Failed to Enqueue MQ for ExportApp:", err)
		return err
	}
	logrus.Debugf("equeue mq build plugin from image success")

	return nil
}

func BuildExportAppBody(tr *model.ExportAppStruct) *ExportAppBody {
	return &ExportAppBody{
		EventID:    tr.Body.EventID,
		ServiceKey: tr.Body.GroupKey,
		Format:     tr.Body.Format,
		SourceDir:  tr.SourceDir,
	}
}

type ExportAppBody struct {
	EventID    string `json:"event_id"`
	ServiceKey string `json:"service_key"`
	Format     string `json:"format"`
	SourceDir  string `json:"source_dir"`
}

func saveMetadata(tr *model.ExportAppStruct) error {
	os.RemoveAll(tr.SourceDir)
	os.MkdirAll(tr.SourceDir, 0755)

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

	context = strings.TrimSpace(context)

	return context
}