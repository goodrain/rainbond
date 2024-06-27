package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
)

var defaultEsComponent *Component

// Component -
type Component struct {
	url      string
	username string
	password string
}

func (c *Component) Start(ctx context.Context, cfg *configs.Config) error {
	c.url = cfg.APIConfig.ElasticSearchURL
	c.username = cfg.APIConfig.ElasticSearchUsername
	c.password = cfg.APIConfig.ElasticSearchPassword
	return nil
}

func (c *Component) SingleStart(url, username, password string) {
	c.url = url
	c.username = username
	c.password = password
}

func (c *Component) CloseHandle() {
}

// New -
func New() *Component {
	defaultEsComponent = &Component{}
	return defaultEsComponent
}

// Default -
func Default() *Component {
	return defaultEsComponent
}

func (c *Component) GET(url string) (string, error) {
	return c.request(url, "GET", "")
}

func (c *Component) POST(url, body string) (string, error) {
	return c.request(url, "POST", body)
}

func (c *Component) PUT(url, body string) (string, error) {
	return c.request(url, "PUT", body)
}

func (c *Component) DELETE(url string) (string, error) {
	return c.request(url, "DELETE", "")
}

func (c *Component) request(url, method, body string) (string, error) {
	req, err := http.NewRequest(method, c.url+url, bytes.NewBuffer([]byte(body)))
	if err != nil {
		logrus.Errorf("Error creating request: %s ", err.Error())
		return "", err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("Error making request: %s", err.Error())
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Errorf("Error closing response body: %s", err.Error())
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 300 {
		logrus.Error(url, body)
		logrus.Errorf("Error response from server: %d %s\n", resp.StatusCode, resp.Status)
		return "", err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Error reading response body: %s", err.Error())
		return "", err
	}
	return string(data), nil
}
