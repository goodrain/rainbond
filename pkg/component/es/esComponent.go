package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

var defaultEsComponent *Component

// Component -
type Component struct {
	url      string
	username string
	password string
	client   *http.Client
}

// createHTTPClient - 创建带有连接池配置的 HTTP 客户端
func createHTTPClient() *http.Client {
	maxIdleConns := 100
	if os.Getenv("HTTP_MAX_IDLE_CONNS") != "" {
		idleCon, err := strconv.Atoi(os.Getenv("HTTP_MAX_IDLE_CONNS"))
		if err == nil {
			maxIdleConns = idleCon
		}
	}
	maxIdleConnsPerHost := 100
	if os.Getenv("HTTP_MAX_IDLE_CONNS_PER_HOST") != "" {
		idleCon, err := strconv.Atoi(os.Getenv("HTTP_MAX_IDLE_CONNS_PER_HOST"))
		if err == nil {
			maxIdleConnsPerHost = idleCon
		}
	}
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:          maxIdleConns,        // 设置最大空闲连接数
		MaxIdleConnsPerHost:   maxIdleConnsPerHost, // 设置每个主机的最大空闲连接数
		IdleConnTimeout:       90 * time.Second,    // 设置空闲连接的超时时间
		TLSHandshakeTimeout:   10 * time.Second,    // 设置 TLS 握手超时时间
		ExpectContinueTimeout: 1 * time.Second,     // 设置 100-continue 状态码超时时间
	}
	client := &http.Client{Transport: tr}
	return client
}

func (c *Component) Start(ctx context.Context, cfg *configs.Config) error {
	c.url = cfg.APIConfig.ElasticSearchURL
	c.username = cfg.APIConfig.ElasticSearchUsername
	c.password = cfg.APIConfig.ElasticSearchPassword
	c.client = createHTTPClient()
	return nil
}

func (c *Component) SingleStart(url, username, password string) {
	c.url = url
	c.username = username
	c.password = password
	c.client = createHTTPClient()
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

	resp, err := c.client.Do(req)
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
		data, _ := io.ReadAll(resp.Body)
		logrus.Errorf("Error request body: %s\n", body)
		logrus.Errorf("Error response body: %s\n", string(data))
		return "", err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Error reading response body: %s", err.Error())
		return "", err
	}
	return string(data), nil
}
