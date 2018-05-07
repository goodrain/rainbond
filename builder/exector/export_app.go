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
	"time"

	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/event"
	"github.com/tidwall/gjson"
)

//ExportApp Export app to specified format(rainbond-app or dockercompose)
type ExportApp struct {
	EventID      string `json:"event_id"`
	ServiceKey   string `json:"service_key"`
	Format       string `json:"format"`
	SourceDir    string `json:"source_dir"`
	Logger       event.Logger
	DockerClient *client.Client
}

func init() {
	RegisterWorker("export_app", NewExportApp)
}

//NewExportApp create
func NewExportApp(in []byte) Worker {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	return &ExportApp{
		ServiceKey: gjson.GetBytes(in, "service_key").String(),
		Format:     gjson.GetBytes(in, "format").String(),
		SourceDir:  gjson.GetBytes(in, "source_dir").String(),
		Logger:     logger,
		EventID:    eventID,
	}
}

//Run Run
func (i *ExportApp) Run(timeout time.Duration) error {
	if i.Format == "rainbond-app" {
		i.exportRainbondAPP()
	} else if i.Format == "dockercompose" {
		i.exportDockerCompose()
	}
	return nil
}

func (i *ExportApp) exportRainbondAPP() {
	//step1: Read app metadata from source dir

	//step2: export docker image

	//step3: export slug file

	//step4: perfect app metadata
}

func (i *ExportApp) exportDockerCompose() {
	//step1: Read app metadata from source dir

	//step2: export docker image

	//step3: export slug file

	//step4: conversion app metadata to dockercompose

	//step5: generate installation package

}

//Stop stop
func (i *ExportApp) Stop() error {
	return nil
}

//Name return worker name
func (i *ExportApp) Name() string {
	return "export_app"
}

//GetLogger GetLogger
func (i *ExportApp) GetLogger() event.Logger {
	return i.Logger
}
