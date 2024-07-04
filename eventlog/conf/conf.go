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

package conf

import "time"

// Conf conf
type Conf struct {
	Entry       EntryConf
	EventStore  EventStoreConf
	Log         LogConf
	WebSocket   WebSocketConf
	WebHook     WebHookConf
	ClusterMode bool
	Cluster     ClusterConf
	Kubernetes  KubernetsConf
}

// WebHookConf webhook conf
type WebHookConf struct {
	ConsoleURL   string
	ConsoleToken string
}

// DBConf db conf
type DBConf struct {
	Type                string
	URL                 string
	DBInterpolateParams string
	PoolSize            int
	PoolMaxSize         int
	HomePath            string
}

// WebSocketConf websocket conf
type WebSocketConf struct {
	BindIP               string
	BindPort             int
	SSLBindPort          int
	EnableCompression    bool
	ReadBufferSize       int
	WriteBufferSize      int
	MaxRestartCount      int
	TimeOut              string
	SSL                  bool
	CertFile             string
	KeyFile              string
	PrometheusMetricPath string
}

// LogConf log conf
type LogConf struct {
	LogLevel   string
	LogOutType string
	LogPath    string
}

// EntryConf entry conf
type EntryConf struct {
	EventLogServer              EventLogServerConf
	DockerLogServer             DockerLogServerConf
	MonitorMessageServer        MonitorMessageServerConf
	NewMonitorMessageServerConf NewMonitorMessageServerConf
}

// EventLogServerConf eventlog server conf
type EventLogServerConf struct {
	BindIP           string
	BindPort         int
	CacheMessageSize int
}

// DockerLogServerConf docker log server conf
type DockerLogServerConf struct {
	BindIP           string
	BindPort         int
	CacheMessageSize int
	Mode             string
}

// DiscoverConf discover conf
type DiscoverConf struct {
	Type          string
	EtcdAddr      []string
	EtcdCaFile    string
	EtcdCertFile  string
	EtcdKeyFile   string
	EtcdUser      string
	EtcdPass      string
	ClusterMode   bool
	InstanceIP    string
	HomePath      string
	DockerLogPort int
	WebPort       int
	NodeID        string
}

// PubSubConf pub sub conf
type PubSubConf struct {
	PubBindIP      string
	PubBindPort    int
	ClusterMode    bool
	PollingTimeout time.Duration
}

// EventStoreConf event store conf
type EventStoreConf struct {
	EventLogPersistenceLength   int64
	MessageType                 string
	GarbageMessageSaveType      string
	GarbageMessageFile          string
	PeerEventMaxLogNumber       int64 //每个event最多日志条数。
	PeerEventMaxCacheLogNumber  int
	PeerDockerMaxCacheLogNumber int64
	ClusterMode                 bool
	HandleMessageCoreNumber     int
	HandleSubMessageCoreNumber  int
	HandleDockerLogCoreNumber   int
	DB                          DBConf
}

// KubernetsConf kubernetes conf
type KubernetsConf struct {
	Master string
}

// ClusterConf cluster conf
type ClusterConf struct {
	PubSub   PubSubConf
	Discover DiscoverConf
}

// MonitorMessageServerConf monitor message server conf
type MonitorMessageServerConf struct {
	SubAddress       []string
	SubSubscribe     string
	CacheMessageSize int
}

// NewMonitorMessageServerConf new monitor message server conf
type NewMonitorMessageServerConf struct {
	ListenerHost string
	ListenerPort int
}
