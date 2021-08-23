package streamlog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/context"

	"strconv"

	"sync"

	"github.com/goodrain/rainbond/node/nodem/logger"
	"github.com/sirupsen/logrus"
)

//STREAMLOGNAME driver name
const name = "streamlog"
const defaultClusterAddress = "http://rbd-eventlog:6363/docker-instance"
const defaultAddress = "rbd-eventlog:6362"

var clusterAddress = []string{defaultClusterAddress}

//ResponseBody api返回数据格式
type ResponseBody struct {
	ValidationError url.Values  `json:"validation_error,omitempty"`
	Msg             string      `json:"msg,omitempty"`
	Bean            interface{} `json:"bean,omitempty"`
	List            []*Endpoint `json:"list,omitempty"`
	//数据集总数
	ListAllNumber int `json:"number,omitempty"`
	//当前页码数
	Page int `json:"page,omitempty"`
}

//Endpoint endpoint
type Endpoint struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Weight int    `json:"weight"`
	Mode   int    `json:"-"` //0 表示URL变化，1表示Weight变化 ,2表示全变化
}

//ParseResponseBody 解析成ResponseBody
func ParseResponseBody(red io.ReadCloser) (re ResponseBody, err error) {
	if red == nil {
		err = errors.New("readcloser can not be nil")
		return
	}
	defer red.Close()
	err = json.NewDecoder(red).Decode(&re)
	return
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

//StreamLog 消息流log
type StreamLog struct {
	writer                         *Client
	serviceID                      string
	tenantID                       string
	containerID                    string
	reConnecting                   chan bool
	serverAddress                  string
	ctx                            context.Context
	cancel                         context.CancelFunc
	cacheSize                      int
	cacheQueue                     chan string
	config                         map[string]string
	intervalSendMicrosecondTime    int64
	minIntervalSendMicrosecondTime int64
	closedChan                     chan struct{}
	once                           sync.Once
}

//New new logger
func New(ctx logger.Info) (logger.Logger, error) {
	var (
		env       = make(map[string]string)
		tenantID  string
		serviceID string
	)
	for _, pair := range ctx.ContainerEnv {
		p := strings.SplitN(pair, "=", 2)
		//logrus.Errorf("ContainerEnv pair: %s", pair)
		if len(p) == 2 {
			key := p[0]
			value := p[1]
			env[key] = value
		}
	}
	tenantID = env["TENANT_ID"]
	serviceID = env["SERVICE_ID"]
	if tenantID == "" {
		tenantID = "default"
	}
	if serviceID == "" {
		serviceID = "default"
	}
	address := getTCPConnConfig(serviceID, ctx.Config["stream-server"])
	writer, err := NewClient(address)
	if err != nil {
		return nil, err
	}

	cacheSize, err := strconv.Atoi(ctx.Config["cache-log-size"])
	if err != nil {
		cacheSize = 1024
	}
	currentCtx, cancel := context.WithCancel(context.Background())
	logger := &StreamLog{
		writer:                         writer,
		serviceID:                      serviceID,
		tenantID:                       tenantID,
		containerID:                    ctx.ContainerID,
		ctx:                            currentCtx,
		cancel:                         cancel,
		cacheSize:                      cacheSize,
		config:                         ctx.Config,
		serverAddress:                  address,
		reConnecting:                   make(chan bool, 1),
		cacheQueue:                     make(chan string, 20000),
		intervalSendMicrosecondTime:    1000 * 10,
		minIntervalSendMicrosecondTime: 1000,
		closedChan:                     make(chan struct{}),
	}
	err = writer.Dial()
	if err != nil {
		logrus.Error("connect log server error.log can not be sended.")
		go logger.reConect()
	} else {
		logrus.Info("stream log server is connected")
	}
	go logger.send()
	return logger, nil
}

func getTCPConnConfig(serviceID, address string) string {
	if address == "" {
		address = GetLogAddress(serviceID)
	}
	return strings.TrimPrefix(address, "tcp://")
}

//ValidateLogOpt 验证参数
func ValidateLogOpt(cfg map[string]string) error {
	for key, value := range cfg {
		switch key {
		case "stream-server":
		case "cache-error-log-size":
			if _, err := strconv.Atoi(value); err != nil {
				return errors.New("cache error log size must be a number")
			}
		default:
			return fmt.Errorf("unknown log opt '%s' for %s log driver", key, name)
		}
	}
	return nil
}

func (s *StreamLog) cache(msg string) {
	defer func() {
		recover()
	}()
	select {
	case s.cacheQueue <- msg:
	default:
		return
	}
}

func (s *StreamLog) send() {
	tike := time.NewTimer(time.Second * 3)
	defer tike.Stop()
	for {
		select {
		case msg := <-s.cacheQueue:
			if msg == "" || msg == "\n" {
				continue
			}
			s.sendMsg(msg)
		case <-s.ctx.Done():
			close(s.closedChan)
			return
		case <-tike.C:
			s.ping()
		}
	}
}
func (s *StreamLog) sendMsg(msg string) {
	if !s.writer.IsClosed() {
		err := s.writer.Write(msg)
		if err != nil {
			logrus.Debug("send log message to stream server error.", err.Error())
			s.cache(msg)
			neterr, ok := err.(net.Error)
			if ok && neterr.Timeout() {
			}
			if len(s.reConnecting) < 1 {
				s.reConect()
			}
		} else {
			if s.intervalSendMicrosecondTime > s.minIntervalSendMicrosecondTime {
				s.intervalSendMicrosecondTime -= 100
			}
		}
	} else {
		if len(s.reConnecting) < 1 {
			s.reConect()
		}
	}
	time.Sleep(time.Microsecond * time.Duration(s.intervalSendMicrosecondTime))
}

func (s *StreamLog) ping() {
	pingMsg := "0x00ping"
	s.sendMsg(pingMsg)
}

//Log log
func (s *StreamLog) Log(msg *logger.Message) error {
	defer func() {
		if err := recover(); err != nil {
			logrus.Error("Stream log pinic.", err)
		}
	}()
	buf := bytes.NewBuffer(nil)
	buf.WriteString(s.containerID[0:12] + ",")
	buf.WriteString(s.serviceID)
	buf.Write(msg.Line)
	s.cache(buf.String())
	return nil
}

func (s *StreamLog) reConect() {
	s.reConnecting <- true
	defer func() {
		<-s.reConnecting
		s.intervalSendMicrosecondTime = 1000 * 10
	}()
	for {
		logrus.Info("start reconnect stream log server.")
		//step1 try reconnect current address
		if s.writer != nil {
			err := s.writer.ReConnect()
			if err == nil {
				return
			}
		}
		//step2 get new server address and reconnect
		server := getTCPConnConfig(s.serviceID, s.config["stream-server"])
		if server == s.writer.server {
			logrus.Warningf("stream log server address(%s) not change ,will reconnect", server)
			err := s.writer.ReConnect()
			if err != nil {
				logrus.Error("stream log server connect error." + err.Error())
			} else {
				return
			}
		} else {
			err := s.writer.ChangeAddress(server)
			if err != nil {
				logrus.Errorf("stream log server connect %s error. %v", server, err.Error())
			} else {
				s.serverAddress = server
				return
			}
		}

		select {
		case <-time.Tick(time.Second * 5):
		case <-s.ctx.Done():
			return
		}
	}
}

//Close 关闭
func (s *StreamLog) Close() error {
	s.cancel()
	<-s.closedChan
	s.once.Do(func() {
		s.writer.Close()
		close(s.cacheQueue)
	})
	return nil
}

//Name 返回logger name
func (s *StreamLog) Name() string {
	return name
}

// GetLogAddress 动态获取日志服务端地址
func GetLogAddress(serviceID string) string {
	var cluster []string
	if len(clusterAddress) < 1 {
		cluster = append(cluster, defaultClusterAddress+"?service_id="+serviceID+"&mode=stream")
	} else {
		for _, a := range clusterAddress {
			cluster = append(cluster, a+"?service_id="+serviceID+"&mode=stream")
		}
	}
	return getLogAddress(cluster)
}

func getLogAddress(clusterAddress []string) string {
	for _, address := range clusterAddress {
		res, err := http.DefaultClient.Get(address)
		if res != nil && res.Body != nil {
			defer res.Body.Close()
		}
		if err != nil {
			logrus.Warningf("Error get host info from %s. %s", address, err)
			continue
		}
		var host = make(map[string]string)
		err = json.NewDecoder(res.Body).Decode(&host)
		if err != nil {
			logrus.Errorf("Error Decode BEST instance host info: %v", err)
			continue
		}
		if status, ok := host["status"]; ok && status == "success" {
			return host["host"]
		}
		logrus.Warningf("Error get host info from %s. result is not success. body is:%v", address, host)
	}
	logrus.Warning("no cluster is running. return default address")
	return defaultAddress
}
