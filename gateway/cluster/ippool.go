package cluster

import (
	"fmt"
	"net"
	"time"

	"github.com/goodrain/rainbond/api/region"
)

type IP_EVENT_TYPE int
type IPEVENT struct {
	Type IP_EVENT_TYPE
	Ips  []string
}

type IPPool struct {
	HostIps        map[string]bool
	AddEvent       *IPEVENT
	DelEvent       *IPEVENT
	EventCh        chan IPEVENT
	isShuttingDown bool
	StopCh         chan struct{}
}

const (
	ADD_IP IP_EVENT_TYPE = iota
	DEL_IP
)

var ipPool *IPPool = NewIPPool()

func (ipev *IPEVENT) AddIp(ip string) {
	ipev.Ips = append(ipev.Ips, ip)
}

func (ipev *IPEVENT) Clear() {
	ipev.Ips = []string{}
}

func NewIpEvent(evtype IP_EVENT_TYPE) *IPEVENT {
	return &IPEVENT{
		Type: evtype,
	}
}

func NewIPPool() *IPPool {
	return &IPPool{
		HostIps:  map[string]bool{},
		AddEvent: NewIpEvent(ADD_IP),
		DelEvent: NewIpEvent(DEL_IP),
		EventCh:  make(chan IPEVENT, 3),
		StopCh:   make(chan struct{}),
	}
}

func (ipm *IPPool) Reset() {
	for ip, _ := range ipm.HostIps {
		ipm.HostIps[ip] = false
	}
	ipm.AddEvent.Clear()
	ipm.DelEvent.Clear()
}

func (ipl *IPPool) GetHostIps() []string {
	var ips []string
	for ip, _ := range ipl.HostIps {
		ips = append(ips, ip)
	}
	return ips
}

func (ipl *IPPool) CheckIps(ips []string) {
	for _, ip := range ips {
		if ok, _ := ipl.HostIps[ip]; !ok {
			ipl.AddEvent.AddIp(ip)
		}
		ipl.HostIps[ip] = true
	}

	for ip, b := range ipl.HostIps {
		if !b {
			ipl.DelEvent.AddIp(ip)
			delete(ipl.HostIps, ip)
		}
	}

	if ipl.isShuttingDown {
		close(ipl.StopCh)
		close(ipl.EventCh)
	} else {
		ipl.EventCh <- *ipl.AddEvent
		ipl.EventCh <- *ipl.DelEvent
	}
}

func (ipl *IPPool) Close() {
	ipl.isShuttingDown = true
}

func LoopCheckIps(ipl *IPPool) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			CheckIps(ipl)
		case <-ipl.StopCh:
			return
		}
	}

}

func CheckIps(ipl *IPPool) {
	ips, err := getIps()
	if err != nil {
		fmt.Println(err)
		return
	}
	ipl.Reset()
	ipl.CheckIps(ips)
}

func startLoop() {
	go LoopCheckIps(ipPool)
	go IPEventWatch(ipPool)
}

func getIps() ([]string, error) {
	var ips []string = []string{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				// fmt.Println(ipnet.IP.String())
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips, nil
}

func IPEventWatch(ipl *IPPool) {
	for ip_event := range ipl.EventCh {
		IpEventHandle(ip_event)
	}
}

func IpEventHandle(ip_event IPEVENT) {
	if ip_event.Type == ADD_IP {
		if len(ip_event.Ips) > 0 {
			//更新数据库
			gwcReg := region.GetRegion().Gateway()
			for _, ip := range ip_event.Ips {
				gwcReg.AddGwcIp(ip)
			}
		}
	} else if ip_event.Type == DEL_IP {
		gwcReg := region.GetRegion().Gateway()
		for _, ip := range ip_event.Ips {
			gwcReg.DelGwcIp(ip)
		}
	}
}

func GetHostIps() []string {
	if ipPool != nil {
		return ipPool.GetHostIps()
	}
	return nil
}
