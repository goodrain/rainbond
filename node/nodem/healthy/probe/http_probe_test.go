package probe
//
//import (
//	"testing"
//	"context"
//	"github.com/goodrain/rainbond/node/nodem/service"
//	"fmt"
//)
//
//func TestHttpProbe_Check(t *testing.T) {
//	ctx, cancel := context.WithCancel(context.Background())
//	resultChannel := make(chan service.HealthStatus,10)
//	httpProbe := HttpProbe{
//		name:"builder",
//		address:"127.0.0.1:3228/v2/builder/health",
//		ctx:ctx,
//		cancel:cancel,
//		resultsChan:resultChannel,
//	}
//	go httpProbe.Check()
//	for  {
//		result := <- resultChannel
//		fmt.Println(result.Name,result.Status,result.Info)
//	}
//}


