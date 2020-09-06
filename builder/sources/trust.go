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

package sources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

//trustedRegistryClient
type trustedRegistryClient struct {
	httpCli            *http.Client
	server, user, pass string
}

//Repostory repostory info
type Repostory struct {
	ID               int    `json:"id,omitempty"`
	Namespace        string `json:"namespace,omitempty"`
	NamespaceType    string `json:"namespaceType,omitempty"`
	Name             string `json:"name"`
	ShortDescription string `json:"shortDescription"`
	LongDescription  string `json:"longDescription"`
	Visibility       string `json:"visibility"`
	Status           string `json:"status,omitempty"`
}

// certificateDirectory returns the directory containing
func createTrustedRegistryClient(server, user, pass string) (*trustedRegistryClient, error) {
	if server == "" {
		return nil, fmt.Errorf("server address can not be empty")
	}
	if !strings.HasPrefix(server, "http") {
		server = "https://" + server
	}
	cli := &trustedRegistryClient{
		httpCli: http.DefaultClient,
		user:    user,
		pass:    pass,
		server:  server,
	}
	return cli, nil
}
func (t *trustedRegistryClient) setAuth(res *http.Request) {
	res.SetBasicAuth(t.user, t.pass)
}
func (t *trustedRegistryClient) getRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	t.setAuth(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}
func (t *trustedRegistryClient) GetRepository(namespace, name string) (*Repostory, error) {
	req, err := t.getRequest("GET", fmt.Sprintf("%s/api/v0/repositories/%s/%s", t.server, namespace, name), nil)
	if err != nil {
		return nil, err
	}
	res, err := t.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == 200 {
		if res.Body != nil {
			defer res.Body.Close()
			var rep Repostory
			err := json.NewDecoder(res.Body).Decode(&rep)
			if err != nil {
				return nil, fmt.Errorf("read response error,%s", err.Error())
			}
			return &rep, nil
		}
	}
	return nil, t.handleErrorResponse(res)
}

func (t *trustedRegistryClient) CreateRepository(namespace string, rep *Repostory) error {
	data, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	req, err := t.getRequest("POST", fmt.Sprintf("%s/api/v0/repositories/%s", t.server, namespace), bytes.NewBuffer(data))
	if err != nil {
		logrus.Errorf("error creating http request: %v", err)
		return fmt.Errorf("error creating http request: %v", err)
	}
	res, err := t.httpCli.Do(req)
	if err != nil {
		logrus.Errorf("error doing http request: %v", err)
		return fmt.Errorf("error doing http request: %v", err)
	}
	if res.StatusCode == 200 {
		return nil
	}
	if res.StatusCode == 201 {
		if res.Body != nil {
			defer res.Body.Close()
			err := json.NewDecoder(res.Body).Decode(rep)
			if err != nil {
				return fmt.Errorf("read response error,%s", err.Error())
			}
			return nil
		}
	}
	return t.handleErrorResponse(res)
}
func (t *trustedRegistryClient) handleErrorResponse(res *http.Response) error {
	if res.Body != nil {
		defer res.Body.Close()
		body, _ := ioutil.ReadAll(res.Body)
		logrus.Debugf("registry request error:%s", string(body))
	}
	switch res.StatusCode {
	case 400:
		return fmt.Errorf("parameter error or resource is exist")
	case 401:
		return fmt.Errorf("The client is not authenticated")
	case 403:
		return fmt.Errorf("The client is not authorized")
	case 404:
		return fmt.Errorf("resource does not exist")
	case 409:
		return fmt.Errorf("Auth not yet configured. A system administrator has not yet set up an auth method")
	default:
		return fmt.Errorf("response code is %d", res.StatusCode)
	}
}
