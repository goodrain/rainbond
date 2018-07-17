package healthy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"io/ioutil"
	"net/http"
	"time"
)

type Probe interface {
	Check() map[string]string
}

type HttpProbe struct {
	name        string
	address     string
	path        string
	resultsChan chan<- service.ProbeResult
	ctx         context.Context
	cancel      context.CancelFunc
}

func (h *HttpProbe) Check() {
	util.Exec(h.ctx, func() error {
		HealthMap := GetHttpHealth(h.address, h.path)
		result := service.ProbeResult{
			Name:   h.name,
			Status: HealthMap["status"],
			Info:   HealthMap["info"],
		}
		h.resultsChan <- result
		return nil
	}, time.Second*3)
}

func GetHttpHealth(address string, path string) map[string]string {
	resp, err := http.Get("http://" + address + path)
	defer resp.Body.Close()
	if err != nil {
		return map[string]string{"status": "unusual", "info": "Service exception, request error"}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// handle error
	}

	//{"bean":{"info":"eventlog service health","status":"health"}}
	m := struct {
		Bean struct {
			Info   string `json:"info"`
			Status string `json:"status"`
		} `json:"bean"`
	}{}

	err = json.Unmarshal(body, &m)
	if err != nil {
		fmt.Println("反序列化出错")
	}

	if m.Bean.Status == "unusual"{
		return map[string]string{"status": m.Bean.Status, "info": m.Bean.Info}
	}

	fmt.Println(string(body))

	return map[string]string{"status": "health", "info": "service health"}

}
