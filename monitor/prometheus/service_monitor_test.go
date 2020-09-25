// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package prometheus

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	mv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	yaml "gopkg.in/yaml.v2"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/workqueue"
)

var smYaml = `
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: region-tokenchecker
  namespace: default
spec:
  jobLabel: service_alias
  endpoints:
  - interval: 10s
    path: /metrics
    port: tcp-9090
    relabelings:
      - sourceLabels: [__address__]
        targetLabel: app_name
        replacement: "region-tokenchecker"
  namespaceSelector:
    any: true
  selector:
    matchLabels:
      service_port: 9090
      port_protocol: http
      name: gr0a581fService
`

func TestCreateScrapeBySM(t *testing.T) {
	var smc ServiceMonitorController
	var sm mv1.ServiceMonitor
	k8syaml.NewYAMLOrJSONDecoder(bytes.NewBuffer([]byte(smYaml)), 1024).Decode(&sm)
	var scrapes []*ScrapeConfig
	t.Logf("%+v", sm)
	for i, ep := range sm.Spec.Endpoints {
		scrapes = append(scrapes, smc.createScrapeBySM(&sm, ep, i))
	}
	out, err := yaml.Marshal(scrapes)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(out))
}

func TestQueue(t *testing.T) {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "sm-monitor")
	defer queue.ShutDown()

	go func() {
		for i := 0; i < 10; i++ {
			queue.Add("abc")
			time.Sleep(time.Second * 1)
		}
	}()
	for {
		item, close := queue.Get()
		if close {
			t.Fatal("queue closed")
		}
		time.Sleep(time.Second * 2)
		fmt.Println(item)
		queue.Forget(item)
		queue.Done(item)
	}
}
