package healthy

import (
	"testing"
	"github.com/goodrain/rainbond/node/nodem/service"
	"context"
	"fmt"
)

//func TestProbeManager_WatchServiceHealthy(t *testing.T) {
//	ctx, cancel := context.WithCancel(context.Background())
//	serviceList := make([]*service.Service, 0, 10)
//
//	h := &service.Service{
//		Name: "builder",
//		ServiceHealth: &service.Health{
//			Name:    "builder",
//			Model:   "http",
//			Address: "127.0.0.1:3228",
//			Path:    "/v2/builder/health",
//		},
//	}
//	serviceList = append(serviceList, h)
//	v := ProbeManager{
//		ctx:      ctx,
//		cancel:   cancel,
//		services: serviceList,
//	}
//
//	resultchan := v.WatchServiceHealthy()
//
//	fmt.Println(len(resultchan))
//	for {
//		result := <-resultchan
//		fmt.Println(result.Name, result.Status, result.Info)
//	}
//}

//func TestGetHttpHealth(t *testing.T) {
//	ctx, cancel := context.WithCancel(context.Background())
//	serviceList := make([]*service.Service, 0, 10)
//
//	h := &service.Service{
//		Name: "builder",
//		ServiceHealth: &service.Health{
//			Name:    "builder",
//			Model:   "http",
//			Address: "127.0.0.1:3228",
//			Path:    "/v2/builder/health",
//		},
//	}
//	serviceList = append(serviceList, h)
//	v := ProbeManager{
//		ctx:      ctx,
//		cancel:   cancel,
//		services: serviceList,
//	}
//	result := v.GetServiceHealthy("builder")
//	fmt.Println(result.Name, result.Status, result.Info)
//}

func TestProbeManager_Start(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		serviceList := make([]*service.Service, 0, 10)

		h := &service.Service{
			Name: "builder",
			ServiceHealth: &service.Health{
				Name:    "builder",
				Model:   "http",
				Address: "127.0.0.1:3228",
				Path:    "/v2/builder/health",
			},
		}
		serviceList = append(serviceList, h)
		v := ProbeManager{
			ctx:      ctx,
			cancel:   cancel,
			services: serviceList,
		}
		channel,_ := v.Start()
	for  {
		result := <-channel
		fmt.Println(result.Name,result.Status,result.Info)
	}


}