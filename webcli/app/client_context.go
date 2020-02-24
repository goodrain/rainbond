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

package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/Sirupsen/logrus"
	"github.com/fatih/structs"
	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"
)

//ClientContext websocket context
type ClientContext struct {
	app        *App
	request    *http.Request
	connection *websocket.Conn
	command    *exec.Cmd
	pty        *os.File
	writeMutex *sync.Mutex
	exec       Exec
}

const (
	Input          = '0'
	Ping           = '1'
	ResizeTerminal = '2'
)

const (
	Output         = '0'
	Pong           = '1'
	SetWindowTitle = '2'
	SetPreferences = '3'
	SetReconnect   = '4'
)

type argResizeTerminal struct {
	Columns float64
	Rows    float64
}

type ContextVars struct {
	Command    string
	Pid        int
	Hostname   string
	RemoteAddr string
}

func (context *ClientContext) goHandleClient(stop chan struct{}, close func()) {
	exit := make(chan bool, 2)

	go func() {
		defer func() { exit <- true }()

		context.processSend()
	}()

	go func() {
		defer func() { exit <- true }()

		context.processReceive()
	}()

	go func() {
		select {
		case <-stop:
		case <-exit:
		}
		context.pty.Close()
		var once sync.Once
		// Even if the PTY has been closed,
		for context.exec.WaitingStop() {
			once.Do(close)
			time.Sleep(time.Millisecond * 200)
		}

		context.connection.Close()
		logrus.Info("Connection closed: %s", context.request.RemoteAddr)
	}()
}

func (context *ClientContext) processSend() {
	if err := context.sendInitialize(); err != nil {
		logrus.Errorf(err.Error())
		return
	}

	buf := make([]byte, 1024)

	for {
		size, err := context.pty.Read(buf)
		if err != nil {
			logrus.Errorf("Command exited for: %s", context.request.RemoteAddr)
			return
		}
		safeMessage := base64.StdEncoding.EncodeToString([]byte(buf[:size]))
		if err = context.write(append([]byte{Output}, []byte(safeMessage)...)); err != nil {
			logrus.Errorf(err.Error())
			return
		}
	}
}

func (context *ClientContext) write(data []byte) error {
	context.writeMutex.Lock()
	defer context.writeMutex.Unlock()
	return context.connection.WriteMessage(websocket.TextMessage, data)
}

func (context *ClientContext) sendInitialize() error {
	hostname, _ := os.Hostname()
	titleVars := ContextVars{
		Command:    "", //strings.Join(context.app.command, " "),
		Pid:        0,  //context.command.Process.Pid,
		Hostname:   hostname,
		RemoteAddr: context.request.RemoteAddr,
	}

	titleBuffer := new(bytes.Buffer)
	if err := context.app.titleTemplate.Execute(titleBuffer, titleVars); err != nil {
		return err
	}
	if err := context.write(append([]byte{SetWindowTitle}, titleBuffer.Bytes()...)); err != nil {
		return err
	}

	prefStruct := structs.New(context.app.options.Preferences)
	prefMap := prefStruct.Map()
	htermPrefs := make(map[string]interface{})
	for key, value := range prefMap {
		rawKey := prefStruct.Field(key).Tag("hcl")
		if _, ok := context.app.options.RawPreferences[rawKey]; ok {
			htermPrefs[strings.Replace(rawKey, "_", "-", -1)] = value
		}
	}
	prefs, err := json.Marshal(htermPrefs)
	if err != nil {
		return err
	}

	if err := context.write(append([]byte{SetPreferences}, prefs...)); err != nil {
		return err
	}
	if context.app.options.EnableReconnect {
		reconnect, _ := json.Marshal(context.app.options.ReconnectTime)
		if err := context.write(append([]byte{SetReconnect}, reconnect...)); err != nil {
			return err
		}
	}
	return nil
}

func (context *ClientContext) processReceive() {
	for {
		_, data, err := context.connection.ReadMessage()
		if err != nil {
			logrus.Errorf(err.Error())
			return
		}
		if len(data) == 0 {
			logrus.Errorf("An error has occurred")
			return
		}

		switch data[0] {
		case Input:
			if !context.app.options.PermitWrite {
				break
			}
			_, err := context.pty.Write(data[1:])
			if err != nil {
				return
			}

		case Ping:
			if err := context.write([]byte{Pong}); err != nil {
				logrus.Errorf(err.Error())
				return
			}
		case ResizeTerminal:
			var args argResizeTerminal
			err = json.Unmarshal(data[1:], &args)
			if err != nil {
				logrus.Errorf("Malformed remote command")
				return
			}

			window := struct {
				row uint16
				col uint16
				x   uint16
				y   uint16
			}{
				uint16(args.Rows),
				uint16(args.Columns),
				0,
				0,
			}
			syscall.Syscall(
				syscall.SYS_IOCTL,
				context.pty.Fd(),
				syscall.TIOCSWINSZ,
				uintptr(unsafe.Pointer(&window)),
			)

		default:
			logrus.Errorf("Unknown message type")
			return
		}
	}
}

//Next next
func (context *ClientContext) Next() *remotecommand.TerminalSize {
	return &remotecommand.TerminalSize{
		Width:  1200,
		Height: 600,
	}
}
