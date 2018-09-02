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
// along with c program. If not, see <http://www.gnu.org/licenses/>.

package sources

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"golang.org/x/net/context"
)

//SVNClient svn svnclient
type SVNClient interface {
	Checkout() (*Info, error)
}

type svnclient struct {
	username string
	password string
	svnURL   string
	svnDir   string
	Env      []string
	logger   event.Logger
}

// NewClient new svn svnclient
func NewClient(username, password, url, sourceDir string, logger event.Logger) SVNClient {
	util.CheckAndCreateDir(sourceDir)
	return &svnclient{username: username, password: password, svnURL: url, svnDir: sourceDir, logger: logger}
}

// NewClientWithEnv ...
func NewClientWithEnv(username, password, url, sourceDir string, env []string) SVNClient {
	util.CheckAndCreateDir(sourceDir)
	return &svnclient{username: username, password: password, svnURL: url, Env: env, svnDir: sourceDir}
}

// Diff ...
func (c *svnclient) Diff(start, end int) (string, error) {
	r := fmt.Sprintf("%d:%d", start, end)
	out, err := c.run("diff", "-r", r, c.svnURL)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Cat ...
func (c *svnclient) Cat(file string) (string, error) {
	out, err := c.run("cat", c.svnURL+"/"+file)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Export ...
func (c *svnclient) Export(dir string) error {
	_, err := c.run("export", c.svnURL, dir)
	if err != nil {
		return err
	}
	return nil

}

// Log ...
func (c *svnclient) Log() (*Logs, error) {
	out, err := c.run("log", "--xml", "-v")
	if err != nil {
		return nil, err
	}
	l := new(Logs)
	err = xml.Unmarshal(out, l)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// list ...
func (c *svnclient) List() (*lists, error) {
	cmd := []string{"list", c.svnURL, "--xml"}
	out, err := c.run(cmd...)
	if err != nil {
		return nil, err
	}
	l := new(lists)
	err = xml.Unmarshal(out, l)
	if err != nil {
		return nil, err
	}
	return l, nil

}

// Checkout
func (c *svnclient) Checkout() (*Info, error) {
	cmd := []string{"checkout", c.svnURL}
	if c.svnDir != "" {
		cmd = append(cmd, c.svnDir)
	}
	_, err := c.runWithLogger(cmd...)
	if err != nil {
		return nil, err
	}
	return c.Info()
}

// svnclient Checkout from specific revision
func (c *svnclient) CheckoutWithRevision(revision string) (string, error) {
	cmd := []string{"checkout", c.svnURL}
	if c.svnDir != "" {
		cmd = append(cmd, c.svnDir)
	}
	cmd = append(cmd, "-r", revision)
	out, err := c.run(cmd...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// info ...
func (c *svnclient) Info() (*Info, error) {
	start := time.Now()
	cmd := []string{"info", "--xml"}
	out, err := c.run(cmd...)
	if err != nil {
		return nil, err
	}
	info := new(Info)
	err = xml.Unmarshal(out, info)
	if err != nil {
		return nil, err
	}
	log, err := c.Log()
	if err == nil {
		info.Logs = log
	}
	fmt.Println(time.Now().Sub(start).String())
	info.Branchs = c.readBranchs()
	info.Tags = c.readTags()
	return info, nil
}
func (c *svnclient) readBranchs() []string {
	re := []string{""}
	exist, _ := util.FileExists(path.Join(c.svnDir, "Branches"))
	if exist {
		list, err := ioutil.ReadDir(path.Join(c.svnDir, "Branches"))
		if err != nil {
			return re
		}
		for _, f := range list {
			re = append(re, f.Name())
		}
		return re
	}
	exist, _ = util.FileExists(path.Join(c.svnDir, "branches"))
	if exist {
		list, err := ioutil.ReadDir(path.Join(c.svnDir, "Branches"))
		if err != nil {
			return re
		}
		for _, f := range list {
			re = append(re, f.Name())
		}
		return re
	}
	return re
}

func (c *svnclient) readTags() []string {
	re := []string{}
	exist, _ := util.FileExists(path.Join(c.svnDir, "Tags"))
	if exist {
		list, err := ioutil.ReadDir(path.Join(c.svnDir, "Tags"))
		if err != nil {
			return re
		}
		for _, f := range list {
			re = append(re, f.Name())
		}
		return re
	}
	exist, _ = util.FileExists(path.Join(c.svnDir, "tags"))
	if exist {
		list, err := ioutil.ReadDir(path.Join(c.svnDir, "tags"))
		if err != nil {
			return re
		}
		for _, f := range list {
			re = append(re, f.Name())
		}
		return re
	}
	return re
}

// run 运行命令
func (c *svnclient) runWithLogger(args ...string) ([]byte, error) {
	ops := []string{"--username", c.username, "--password", c.password, "--non-interactive", "--trust-server-cert"}
	args = append(args, ops...)
	cmd := exec.Command("svn", args...)
	if len(c.Env) > 0 {
		cmd.Env = append(os.Environ(), c.Env...)
	}
	cmd.Dir = c.svnDir
	writer := c.logger.GetWriter("progress", "debug")
	writer.SetFormat(`{"progress":"%s","id":"SVN:"}`)
	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// run 运行命令
func (c *svnclient) run(args ...string) ([]byte, error) {
	ops := []string{"--username", c.username, "--password", c.password, "--non-interactive", "--trust-server-cert"}
	args = append(args, ops...)
	cmd := exec.Command("svn", args...)
	if len(c.Env) > 0 {
		cmd.Env = append(os.Environ(), c.Env...)
	}
	cmd.Dir = c.svnDir
	return cmd.Output()
}

//Logs commit logs
type Logs struct {
	XMLName      xml.Name `xml:"log"`
	CommitEntrys []Commit `xml:"logentry"`
}

type paths struct {
	Path     string `xml:",innerxml"`
	Action   string `xml:"action,attr"`
	PropMods string `xml:"prop-mods,attr"`
	TextMods string `xml:"text-mods,attr"`
	Kind     string `xml:"kind,attr"`
}

//Info Info
type Info struct {
	XMLName       xml.Name `xml:"info"`
	URL           string   `xml:"entry>url"`
	RelativeURL   string   `xml:"entry>relative-url"`
	Root          string   `xml:"entry>repository>root"`
	UUID          string   `xml:"entry>repository>uuid"`
	WcrootAbspath string   `xml:"entry>wc-info>wcroot-abspath"`
	Schedule      string   `xml:"entry>wc-info>schedule"`
	Depth         string   `xml:"entry>wc-info>depth"`
	Logs          *Logs
	Branchs       []string
	Tags          []string
}

//Commit Commit
type Commit struct {
	Revision string `xml:"revision,attr"`
	Author   string `xml:"author"`
	Msg      string `xml:"msg"`
	Date     string `xml:"date"`
}

type lists struct {
	XMLName xml.Name `xml:"lists"`
	List    list     `xml:"list"`
}

type list struct {
	Path  string  `xml:"path,attr"`
	Entry []entry `xml:"entry"`
}

type entry struct {
	Kind   string `xml:"kind,attr"`
	Name   string `xml:"name"`
	Size   string `xml:"size"`
	Commit Commit `xml:"commit"`
}

//SvnPull SvnPull
func SvnPull(dir, user, password string) error {
	cmd := exec.Command(
		"svn",
		"update",
		"--ignore-externals",
		"--username",
		user,
		"--password",
		password)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Failed to SVN update %s, see output below\n%sContinuing...", dir, out)
		return err
	}
	return nil
}

//SvnClone clone code by svn
func SvnClone(dir, url, user, password string, logger event.Logger, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	par, rep := filepath.Split(dir)
	if err := util.CheckAndCreateDir(par); err != nil {
		return "", err
	}
	cmd := exec.Command(
		"svn",
		"checkout",
		"--non-interactive",
		"--trust-server-cert-failures=unknown-ca",
		"--username",
		user,
		"--password",
		password,
		url,
		rep)
	cmd.Dir = par
	reader, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	readererr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	startReadProgress(ctx, reader, logger)
	startReadProgress(ctx, readererr, logger)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return "", nil
}

//startReadProgress create svn log progress
func startReadProgress(ctx context.Context, read io.ReadCloser, logger event.Logger) {
	var reader = bufio.NewReader(read)
	go func() {
		defer read.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, _, err := reader.ReadLine()
				if err != nil {
					if err.Error() != "EOF" {
						fmt.Println("read svn log err", err.Error())
					}
					return
				}
				if len(line) > 0 {
					progess := strings.Replace(string(line), "\r", "", -1)
					progess = strings.Replace(progess, "\n", "", -1)
					progess = strings.Replace(progess, "\u0000", "", -1)
					if len(progess) > 0 {
						message := fmt.Sprintf(`{"progress":"%s","id":"%s"}`, progess, "SVN:")
						logger.Debug(message, map[string]string{"step": "progress"})
					}
				}
			}
		}
	}()
}
